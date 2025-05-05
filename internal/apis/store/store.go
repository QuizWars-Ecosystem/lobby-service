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
	lockKey := fmt.Sprintf(lockLobbyKey, l.ID)

	err := s.withRedisLock(ctx, lockKey, time.Second*2, func() error {
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
	})

	return err
}

func (s *Store) GetActiveLobbies(ctx context.Context, mode string) ([]*models.Lobby, error) {
	zsetKey := fmt.Sprintf(activeLobbyKey, mode)

	ids, err := s.db.ZRange(ctx, zsetKey, 0, -1).Result()
	if err != nil {
		s.logger.Error("Failed to get open lobbies", zap.String("mode", mode), zap.Error(err))
		return nil, err
	}

	if len(ids) == 0 {
		return nil, nil
	}

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = fmt.Sprintf(lobbyKey, id)
	}

	values, err := s.db.MGet(ctx, keys...).Result()
	if err != nil {
		s.logger.Error("Failed to batch fetch lobbies", zap.Error(err))
		return nil, err
	}

	var (
		lobbies     []*models.Lobby
		missingKeys []string
	)

	for i, val := range values {
		if val == nil {
			s.logger.Info("Lobby key missing, scheduling removal", zap.String("id", ids[i]))
			missingKeys = append(missingKeys, ids[i])
			continue
		}

		var l models.Lobby
		if err = json.Unmarshal([]byte(val.(string)), &l); err != nil {
			s.logger.Error("Failed to unmarshal lobby", zap.String("id", ids[i]), zap.Error(err))
			continue
		}

		if len(l.Players) >= l.MaxPlayers {
			continue
		}

		lobbies = append(lobbies, &l)
	}

	if len(missingKeys) > 0 {
		members := make([]interface{}, len(missingKeys))
		for i, id := range missingKeys {
			members[i] = id
		}

		if _, err = s.db.ZRem(ctx, zsetKey, members...).Result(); err != nil {
			s.logger.Warn("Failed to clean up broken lobby references", zap.Error(err))
		}
	}

	return lobbies, nil
}

func (s *Store) AddPlayerToLobby(ctx context.Context, lobbyID string, player *models.Player) error {
	lockKey := fmt.Sprintf(lockLobbyKey, lobbyID)

	return s.withRedisLock(ctx, lockKey, 3*time.Second, func() error {
		key := fmt.Sprintf(lobbyKey, lobbyID)

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
			_ = s.db.ZRem(ctx, fmt.Sprintf(activeLobbyKey, l.Mode), lobbyID)
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

		newData, err := json.Marshal(l)
		if err != nil {
			s.logger.Error("Failed to marshal lobby", zap.String("id", lobbyID), zap.Error(err))
			return err
		}

		if err = s.db.Set(ctx, key, newData, time.Minute*5).Err(); err != nil {
			s.logger.Error("Failed to set lobby to db", zap.String("id", lobbyID), zap.Error(err))
			return err
		}

		return nil
	})
}

func (s *Store) UpdateLobbyTTL(ctx context.Context, lobbyID string, ttl time.Duration) error {
	lockKey := fmt.Sprintf(lockLobbyKey, lobbyID)

	return s.withRedisLock(ctx, lockKey, time.Second*2, func() error {
		return s.db.Expire(ctx, fmt.Sprintf(lobbyKey, lobbyID), ttl).Err()
	})
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

func (s *Store) MarkLobbyAsFull(ctx context.Context, lobbyID, mode string) error {
	lockKey := fmt.Sprintf(lockLobbyKey, lobbyID)

	return s.withRedisLock(ctx, lockKey, time.Second*2, func() error {
		return s.removeLobby(ctx, lobbyID, mode)
	})
}

func (s *Store) ExpireLobby(ctx context.Context, lobbyID, mode string) error {
	lockKey := fmt.Sprintf(lockLobbyKey, lobbyID)

	return s.withRedisLock(ctx, lockKey, time.Second*2, func() error {
		return s.removeLobby(ctx, lobbyID, mode)
	})
}

func (s *Store) removeLobby(ctx context.Context, lobbyID, mode string) error {
	pipe := s.db.TxPipeline()

	zsetKey := fmt.Sprintf(activeLobbyKey, mode)
	key := fmt.Sprintf(lobbyKey, lobbyID)

	pipe.ZRem(ctx, zsetKey, lobbyID)
	pipe.Del(ctx, key)

	_, err := pipe.Exec(ctx)
	if err != nil {
		s.logger.Warn("Failed to remove lobby", zap.String("lobby_id", lobbyID), zap.Error(err))
		return err
	}

	return nil
}

func (s *Store) withRedisLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	lockValue := uuid.NewString()
	const (
		maxAttempts = 5
		retryDelay  = 100 * time.Millisecond
	)

	var acquired bool
	for attempt := 0; attempt < maxAttempts; attempt++ {
		ok, err := s.db.SetNX(ctx, key, lockValue, ttl).Result()
		if err != nil {
			s.logger.Warn("Failed to acquire lock", zap.String("key", key), zap.Error(err))
			return err
		}
		if ok {
			acquired = true
			break
		}
		time.Sleep(retryDelay)
	}

	if !acquired {
		s.logger.Warn("Could not acquire lock after retries", zap.String("key", key))
		return apperrors.Internal(errors.New("could not acquire lock"))
	}

	defer func() {
		val, err := s.db.Get(ctx, key).Result()
		if err == nil && val == lockValue {
			s.db.Del(ctx, key)
		}
	}()

	return fn()
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
