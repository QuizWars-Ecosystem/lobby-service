package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"time"

	apperrors "github.com/QuizWars-Ecosystem/go-common/pkg/error"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	lobbyKey       = "lobby:%s"
	activeLobbyKey = "lobby:active:%s"
	lockLobbyKey   = "lock:lobby:%s"
)

type Store struct {
	db     redis.UniversalClient
	logger *zap.Logger
}

func NewStore(db redis.UniversalClient, logger *zap.Logger) *Store {
	return &Store{db: db, logger: logger}
}

func (s *Store) CreateLobby(ctx context.Context, l *models.Lobby, ttl time.Duration) error {
	key := fmt.Sprintf(lobbyKey, l.ID)
	data, err := json.Marshal(l)
	if err != nil {
		s.logger.Error("Failed to marshal lobby", zap.String("id", l.ID), zap.Error(err))
		return apperrors.Internal(err)
	}

	pipe := s.db.TxPipeline()

	if err = pipe.Set(ctx, key, data, ttl).Err(); err != nil {
		s.logger.Error("Failed to save lobby", zap.String("id", l.ID), zap.Error(err))
	}

	if err = pipe.ZAdd(ctx, fmt.Sprintf(activeLobbyKey, l.Mode), redis.Z{Score: float64(l.AvgRating), Member: l.ID}).Err(); err != nil {
		s.logger.Error("Failed to save lobby as open", zap.String("id", l.ID), zap.Error(err))
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("Failed to cache lobby", zap.String("id", l.ID), zap.Error(err))
		return apperrors.Internal(err)
	}

	return err
}

func (s *Store) GetActiveLobbies(ctx context.Context, mode string) ([]*models.Lobby, error) {
	keys, err := s.db.ZRange(ctx, fmt.Sprintf(activeLobbyKey, mode), 0, -1).Result()
	if err != nil {
		s.logger.Error("Failed to get open lobbies", zap.String("mode", mode), zap.Error(err))
		return nil, err
	}

	var lobbies []*models.Lobby
	var data string

	for _, id := range keys {
		data, err = s.db.Get(ctx, "lobby:"+id).Result()
		if err != nil {
			s.logger.Error("Failed to get lobby", zap.String("id", id), zap.Error(err))
			continue
		}

		var l models.Lobby
		if err = json.Unmarshal([]byte(data), &l); err != nil {
			s.logger.Error("Failed to unmarshal lobby", zap.String("id", id), zap.Error(err))
			continue
		}

		lobbies = append(lobbies, &l)
	}

	return lobbies, nil
}

func (s *Store) AddPlayerToLobby(ctx context.Context, lobbyID string, player *models.Player) error {
	key := fmt.Sprintf(lobbyKey, lobbyID)
	lockKey := fmt.Sprintf(lockLobbyKey, lobbyID)
	lockValue := uuid.NewString()
	lockTTL := 1 * time.Second

	const (
		maxAttempts = 5
		retryDelay  = 100 * time.Millisecond
	)

	var acquired bool
	for attempt := 0; attempt < maxAttempts; attempt++ {
		ok, err := s.db.SetNX(ctx, lockKey, lockValue, lockTTL).Result()
		if err != nil {
			s.logger.Warn("Failed to acquire lock", zap.String("lobby_id", lobbyID), zap.Error(err))
			return err
		}
		if ok {
			acquired = true
			break
		}
		time.Sleep(retryDelay)
	}

	if !acquired {
		s.logger.Warn("Could not acquire lobby lock after retries", zap.String("lobby_id", lobbyID))
		return apperrors.Internal(errors.New("could not acquire lock"))
	}

	defer func() {
		val, err := s.db.Get(ctx, lockKey).Result()
		if err == nil && val == lockValue {
			s.db.Del(ctx, lockKey)
		}
	}()

	data, err := s.db.Get(ctx, key).Result()
	if err != nil {
		s.logger.Warn("Failed to get lobby from db", zap.String("id", lobbyID), zap.Error(err))
		return err
	}

	var l models.Lobby
	if err = json.Unmarshal([]byte(data), &l); err != nil {
		s.logger.Error("Failed to unmarshal lobby", zap.String("id", lobbyID), zap.Error(err))
		return err
	}

	if len(l.Players) >= l.MaxPlayers {
		s.logger.Warn("Lobby is already full", zap.String("lobby_id", lobbyID))
		return apperrors.BadRequest(errors.New("lobby is full"))
	}

	l.Players = append(l.Players, player)

	var total int32
	for _, p := range l.Players {
		total += p.Rating
	}
	l.AvgRating = total / int32(len(l.Players))
	l.Categories = mergeCategories(l.Categories, player.Categories)
	l.LastJoinedAt = time.Now()

	newData, _ := json.Marshal(l)
	if err = s.db.Set(ctx, key, newData, 30*time.Second).Err(); err != nil {
		s.logger.Error("Failed to set lobby to db", zap.String("id", lobbyID), zap.Error(err))
		return err
	}

	return nil
}

func (s *Store) MarkLobbyAsFull(ctx context.Context, lobbyID string) error {
	pipe := s.db.TxPipeline()

	if err := pipe.Del(ctx, fmt.Sprintf(activeLobbyKey, lobbyID)).Err(); err != nil {
		s.logger.Warn("Failed to remove full lobby from active", zap.String("id", lobbyID), zap.Error(err))
	}

	if err := pipe.Del(ctx, fmt.Sprintf(lobbyKey, lobbyID)).Err(); err != nil {
		s.logger.Warn("Failed to expire lobby", zap.String("id", lobbyID), zap.Error(err))
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) UpdateLobbyTTL(ctx context.Context, lobbyID string, ttl time.Duration) error {
	return s.db.Expire(ctx, fmt.Sprintf(lobbyKey, lobbyID), ttl).Err()
}

func (s *Store) GetLobby(ctx context.Context, lobbyID string) (*models.Lobby, error) {
	data, err := s.db.Get(ctx, fmt.Sprintf(lobbyKey, lobbyID)).Result()
	if err != nil {
		s.logger.Error("Failed to get lobby from db", zap.String("id", lobbyID), zap.Error(err))
		return nil, err
	}

	var l models.Lobby
	if err = json.Unmarshal([]byte(data), &l); err != nil {
		s.logger.Error("Failed to unmarshal lobby", zap.String("id", lobbyID), zap.Error(err))
		return nil, err
	}

	return &l, nil
}

func (s *Store) MarkLobbyAsStarted(ctx context.Context, lobbyID string) error {
	return s.db.ZRem(ctx, "lobby:open", lobbyID).Err()
}

func (s *Store) ExpireLobby(ctx context.Context, lobbyID string) error {
	return s.db.Del(ctx, fmt.Sprintf("lobby:%s", lobbyID)).Err()
}

func mergeCategories(a, b []int32) []int32 {
	set := make(map[int32]struct{}, len(a)+len(b))
	for _, v := range a {
		set[v] = struct{}{}
	}
	for _, v := range b {
		set[v] = struct{}{}
	}

	result := make([]int32, 0, len(set))
	for k := range set {
		result = append(result, k)
	}

	return result
}
