package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models/scorer"
	"github.com/go-redsync/redsync/v4"
	redsyncgoredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	lobbyKey        = "lobby:{%s}"
	versionLobbyKey = "lobby:version:{%s}"
	activeLobbyKey  = "lobby:active:{%s}"
	mutexLobbyKey   = "{lobby:%s}"
)

type Store struct {
	db            redis.UniversalClient
	redsync       *redsync.Redsync
	mu            sync.RWMutex
	scoreProvider *scorer.ScoreProviders
	logger        *zap.Logger
}

func NewStore(db redis.UniversalClient, logger *zap.Logger) *Store {
	pool := redsyncgoredis.NewPool(db)
	return &Store{
		db:            db,
		scoreProvider: scorer.NewScoreProviders(),
		redsync:       redsync.New(pool),
		logger:        logger,
	}
}

func (s *Store) AddLobby(ctx context.Context, lobby *models.Lobby) error {
	sp := s.scoreProvider.GetProvider(lobby.Mode)
	if sp == nil {
		return fmt.Errorf("no score provider for mode: %s", lobby.Mode)
	}

	keyLobby := fmt.Sprintf(lobbyKey, lobby.ID)
	keyScore := fmt.Sprintf(activeLobbyKey, lobby.Mode)
	keyVersion := fmt.Sprintf(versionLobbyKey, lobby.ID)

	oldVer, _ := s.db.Get(ctx, keyVersion).Int64()
	if int16(oldVer) >= lobby.Version {
		return nil
	}

	score := sp.CalculateScore(lobby)

	// s.logger.Info("Lobby data", zap.String("lobby_id", lobby.ID), zap.String("mode", lobby.Mode), zap.Int16("version", lobby.Version), zap.Float64("score", score), zap.Int32s("categories", lobby.Categories))

	data, err := json.Marshal(lobby)
	if err != nil {
		s.logger.Error("Failed to marshal lobby", zap.String("lobby_id", lobby.ID), zap.Error(err))
		return err
	}

	ttl := time.Until(lobby.ExpireAt)
	pipe := s.db.TxPipeline()

	pipe.Set(ctx, keyLobby, data, ttl)
	pipe.ZAdd(ctx, keyScore, redis.Z{Score: score, Member: lobby.ID})
	pipe.Expire(ctx, keyScore, ttl)
	pipe.Set(ctx, keyVersion, lobby.Version, ttl)

	if _, err = pipe.Exec(ctx); err != nil {
		s.logger.Error("Failed to save lobby to db", zap.String("lobby_id", lobby.ID), zap.Error(err))
		return err
	}

	return nil
}

func (s *Store) GetLobby(ctx context.Context, lobbyID string) (*models.Lobby, error) {
	key := fmt.Sprintf(lobbyKey, lobbyID)

	data, err := s.db.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		s.logger.Error("Lobby not found", zap.String("lobby_id", lobbyID))
		return nil, err
	} else if err != nil {
		s.logger.Error("Failed to get data from db", zap.String("lobby_id", lobbyID), zap.Error(err))
		return nil, err
	}

	var lobby models.Lobby
	if err = json.Unmarshal(data, &lobby); err != nil {
		s.logger.Error("Failed to unmarshal data", zap.String("lobby_id", lobbyID), zap.Error(err))
		return nil, err
	}

	return &lobby, nil
}

func (s *Store) GetLobbies(ctx context.Context, mode string) ([]*models.Lobby, error) {
	key := fmt.Sprintf(activeLobbyKey, mode)

	ids, err := s.db.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		s.logger.Error("Failed to get lobbies", zap.String("mode", mode), zap.Error(err))
		return nil, err
	}

	return s.loadLobbiesByIDs(ctx, mode, ids)
}

func (s *Store) GetTopLobbies(ctx context.Context, mode string, limit int) ([]*models.Lobby, error) {
	key := fmt.Sprintf(activeLobbyKey, mode)

	ids, err := s.db.ZRevRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		s.logger.Error("Failed to get top lobbies", zap.String("mode", mode), zap.Error(err))
		return nil, err
	}

	return s.loadLobbiesByIDs(ctx, mode, ids)
}

func (s *Store) GetLobbiesByScore(ctx context.Context, mode string, min, max float64) ([]*models.Lobby, error) {
	key := fmt.Sprintf(activeLobbyKey, mode)

	ids, err := s.db.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprint(min),
		Max: fmt.Sprint(max),
	}).Result()
	if err != nil {
		s.logger.Error("Failed to get lobbies by score range", zap.String("mode", mode), zap.Float64("min", min), zap.Float64("max", max), zap.Error(err))
		return nil, err
	}

	return s.loadLobbiesByIDs(ctx, mode, ids)
}

func (s *Store) AddPlayer(ctx context.Context, lobbyID string, player *models.Player) error {
	lockKey := fmt.Sprintf(mutexLobbyKey, lobbyID)

	mutex := s.redsync.NewMutex(lockKey,
		redsync.WithExpiry(5*time.Second),
		redsync.WithTries(5),
		redsync.WithRetryDelayFunc(func(n int) time.Duration {
			return time.Duration(100+rand.Intn(200)) * time.Millisecond
		}),
	)

	if err := mutex.LockContext(ctx); err != nil {
		return err
	}
	defer func() {
		_, _ = mutex.UnlockContext(ctx)
	}()

	lobby, err := s.GetLobby(ctx, lobbyID)
	if err != nil {
		return err
	}

	if ok := lobby.AddPlayer(player); !ok {
		return errors.New("failed to add player to lobby")
	}

	if err = s.AtomicUpdateLobby(ctx, lobby); err != nil {
		return err
	}

	return nil
}

func (s *Store) AtomicUpdateLobby(ctx context.Context, lobby *models.Lobby) error {
	sp := s.scoreProvider.GetProvider(lobby.Mode)
	if sp == nil {
		return fmt.Errorf("no score provider for mode: %s", lobby.Mode)
	}

	keyVersion := fmt.Sprintf(versionLobbyKey, lobby.ID)
	oldVer, err := s.db.Get(ctx, keyVersion).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		s.logger.Error("Failed to get lobby version", zap.String("lobby_id", lobby.ID), zap.Error(err))
		return err
	}

	if int16(oldVer) >= lobby.Version {
		return nil
	}

	data, err := json.Marshal(lobby)
	if err != nil {
		return err
	}

	score := sp.CalculateScore(lobby)
	ttl := time.Until(lobby.ExpireAt)

	keyLobby := fmt.Sprintf(lobbyKey, lobby.ID)
	keyScore := fmt.Sprintf(activeLobbyKey, lobby.Mode)
	keyVersion = fmt.Sprintf(versionLobbyKey, lobby.ID)

	pipe := s.db.TxPipeline()

	pipe.Set(ctx, keyLobby, data, ttl)
	pipe.ZAdd(ctx, keyScore, redis.Z{Score: score, Member: lobby.ID})
	pipe.Expire(ctx, keyScore, ttl)
	pipe.Set(ctx, keyVersion, lobby.Version, ttl)

	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("Failed to update lobby", zap.String("lobby_id", lobby.ID), zap.Error(err))
		return err
	}

	return nil
}

func (s *Store) RemoveLobby(ctx context.Context, lobbyID, mode string) error {
	keyLobby := fmt.Sprintf(lobbyKey, lobbyID)
	keyZSet := fmt.Sprintf(activeLobbyKey, mode)
	keyVer := fmt.Sprintf(versionLobbyKey, lobbyID)

	pipe := s.db.TxPipeline()
	pipe.Del(ctx, keyLobby)
	pipe.ZRem(ctx, keyZSet, lobbyID)
	pipe.Del(ctx, keyVer)

	if _, err := pipe.Exec(ctx); err != nil {
		s.logger.Error("Failed to remove lobby from db", zap.String("lobby_id", lobbyID), zap.Error(err))
		return err
	}

	return nil
}

func (s *Store) loadLobbiesByIDs(ctx context.Context, mode string, ids []string) ([]*models.Lobby, error) {
	if len(ids) == 0 {
		return []*models.Lobby{}, nil
	}

	keyToID := make(map[string]string, len(ids))
	keys := make([]string, len(ids))
	for i, id := range ids {
		key := fmt.Sprintf(lobbyKey, id)
		keys[i] = key
		keyToID[key] = id
	}

	slotMap := make(map[int64][]string)
	for _, key := range keys {
		slot, err := s.db.ClusterKeySlot(ctx, key).Result()
		if err != nil {
			s.logger.Error("Failed to get slot for key", zap.String("key", key), zap.Error(err))
			return nil, err
		}
		slotMap[slot] = append(slotMap[slot], key)
	}

	type redisData struct {
		Key string
		Val any
	}

	var (
		wg       sync.WaitGroup
		resultCh = make(chan []redisData, len(slotMap))
		errCh    = make(chan error, 1)
	)

	for _, slotKeys := range slotMap {
		slotKeysCopy := append([]string(nil), slotKeys...)
		wg.Add(1)
		go func(keys []string) {
			defer wg.Done()
			values, err := s.db.MGet(ctx, keys...).Result()
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}

			pairs := make([]redisData, 0, len(keys))
			for i, val := range values {
				pairs = append(pairs, redisData{
					Key: keys[i],
					Val: val,
				})
			}
			resultCh <- pairs
		}(slotKeysCopy)
	}

	wg.Wait()
	close(resultCh)

	select {
	case err := <-errCh:
		s.logger.Error("Failed during parallel MGet", zap.Error(err))
		return nil, err
	default:
	}

	lobbies := make([]*models.Lobby, 0, len(ids))
	var missingKeys []interface{}

	for pairs := range resultCh {
		for i, p := range pairs {
			if p.Val == nil {
				// s.logger.Warn("Skipping missing lobby", zap.String("id", ids[i]))
				missingKeys = append(missingKeys, fmt.Sprintf(lobbyKey, ids[i]))
				continue
			}

			strVal, ok := p.Val.(string)
			if !ok {
				s.logger.Warn("Unexpected value type for key", zap.String("key", p.Key))
				continue
			}
			var lobby models.Lobby
			if err := json.Unmarshal([]byte(strVal), &lobby); err != nil {
				s.logger.Error("Failed to unmarshal lobby", zap.String("key", p.Key), zap.Error(err))
				continue
			}

			if lobby.CanAddPlayer() {
				lobbies = append(lobbies, &lobby)
			}
		}
	}

	defer func() {
		activeLobbiesKey := fmt.Sprintf(activeLobbyKey, mode)
		if len(missingKeys) > 0 {
			if _, err := s.db.ZRem(ctx, activeLobbiesKey, missingKeys...).Result(); err != nil {
				s.logger.Warn("Failed to clean up broken lobby references", zap.Error(err))
			}
		}
	}()

	return lobbies, nil
}
