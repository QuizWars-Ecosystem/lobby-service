package store

import (
	"context"
	"encoding/json"
	"fmt"
	apperrors "github.com/QuizWars-Ecosystem/go-common/pkg/error"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"time"
)

const (
	lobbyKey     = "lobby:%s"
	openLobbyKey = "lobby:open:%s"
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

	pipe.Set(ctx, key, data, ttl)

	pipe.ZAdd(ctx, fmt.Sprintf(openLobbyKey, l.Mode), redis.Z{Score: float64(l.AvgRating), Member: l.ID})

	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("Failed to cache lobby", zap.String("id", l.ID), zap.Error(err))
		return apperrors.Internal(err)
	}

	return err
}

func (s *Store) GetOpenLobbies(ctx context.Context, mode string) ([]*models.Lobby, error) {
	keys, err := s.db.ZRange(ctx, fmt.Sprintf(openLobbyKey, mode), 0, -1).Result()
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

	data, err := s.db.Get(ctx, key).Result()
	if err != nil {
		s.logger.Error("Failed to get lobby from db", zap.String("id", lobbyID), zap.Error(err))
		return err
	}

	var l models.Lobby
	if err = json.Unmarshal([]byte(data), &l); err != nil {
		s.logger.Error("Failed to unmarshal lobby", zap.String("id", lobbyID), zap.Error(err))
		return err
	}

	l.Players = append(l.Players, player)
	var total int32

	for _, p := range l.Players {
		total += p.Rating
	}

	l.AvgRating = total / int32(len(l.Players))

	newData, _ := json.Marshal(l)
	if err = s.db.Set(ctx, key, newData, 30*time.Second).Err(); err != nil {
		s.logger.Error("Failed to set lobby to db", zap.String("id", lobbyID), zap.Error(err))
		return err
	}

	return nil
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
