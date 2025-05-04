package lobby

import (
	"context"
	"sync"
	"time"

	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/store"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/streamer"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"go.uber.org/zap"
)

type Config struct {
	TickerTimeout    time.Duration `mapstructure:"tickerTimeout" default:"1s"`
	MaxLobbyWait     time.Duration `mapstructure:"maxLobbyWait" default:"1m"`
	LobbyIdleExtend  time.Duration `mapstructure:"lobbyIdleExtend" default:"15s"`
	MinReadyDuration time.Duration `mapstructure:"minReadyDuration" default:"10s"`
}

var _ abstractions.ConfigSubscriber[*Config] = (*Waiter)(nil)

type Waiter struct {
	store    *store.Store
	streamer *streamer.StreamManager
	logger   *zap.Logger
	mx       sync.Mutex
	cfg      *Config
}

func NewWaiter(store *store.Store, streamer *streamer.StreamManager, logger *zap.Logger, cfg *Config) *Waiter {
	return &Waiter{
		store:    store,
		streamer: streamer,
		logger:   logger,
		cfg:      cfg,
	}
}

func (w *Waiter) WaitForLobbyFill(ctx context.Context, lobby *models.Lobby) {
	ticker := time.NewTicker(w.cfg.TickerTimeout)
	defer ticker.Stop()

	createdAt := time.Now()
	expireAt := createdAt.Add(w.cfg.MaxLobbyWait)

	var prevPlayerCount int

	for {
		select {
		case <-ticker.C:
			updated, err := w.store.GetLobby(ctx, lobby.ID)
			if err != nil {
				return
			}

			playerCount := len(updated.Players)

			w.logger.Debug("LOBBY", zap.Any("UPDATED", *updated))
			w.logger.Debug("PLAYERS", zap.Int("CURRENT", playerCount), zap.Int("PREVIOUS", prevPlayerCount))

			// If players not come and max time waiting expired
			if playerCount == 0 && time.Since(createdAt) > w.cfg.MaxLobbyWait {
				if err = w.store.ExpireLobby(ctx, updated.ID); err != nil {
					w.logger.Warn("Failed to expire lobby", zap.String("id", updated.ID), zap.Error(err))
				}

				w.logger.Info("Lobby removed due to inactivity", zap.String("lobby_id", updated.ID))
				return
			}

			// Is lobby time is expired
			if time.Now().After(expireAt) {
				if err = w.store.ExpireLobby(ctx, updated.ID); err != nil {
					w.logger.Warn("Failed to expire lobby", zap.String("id", updated.ID), zap.Error(err))
				}

				w.streamer.BroadcastLobbyUpdate(updated.ID, &lobbyv1.LobbyStatus{
					LobbyId: updated.ID,
					Status:  lobbyv1.Status_STATUS_TIMEOUT,
				})

				w.logger.Info("Lobby expired", zap.String("lobby_id", updated.ID))
				return
			}

			// New Player come in lobby - extend time
			if playerCount > prevPlayerCount && prevPlayerCount != 0 {
				expireAt = time.Now().Add(w.cfg.LobbyIdleExtend)
				w.logger.Debug("New player joined, extending lobby wait", zap.Int("CURRENT", playerCount), zap.Int("PREV", prevPlayerCount), zap.String("lobby_id", updated.ID))
			}

			// Has min amount of players — can wait more
			if playerCount >= updated.MinPlayers && time.Since(createdAt) < w.cfg.MaxLobbyWait {
				expireAt = time.Now().Add(w.cfg.LobbyIdleExtend)
				w.logger.Info("Min players reached, extending wait time", zap.String("lobby_id", updated.ID))
			}

			// If lobby is ready to start (players & time)
			if playerCount >= updated.MinPlayers &&
				(playerCount == updated.MaxPlayers || time.Since(createdAt) > w.cfg.MinReadyDuration) {

				// Request to Game Router Service
				w.logger.Debug("LOBBY IS READY TO START")

				if err = w.store.MarkLobbyAsStarted(ctx, updated.ID); err != nil {
					w.logger.Warn("Failed to mark lobby as started", zap.String("id", updated.ID), zap.Error(err))
				}

				w.streamer.BroadcastLobbyUpdate(updated.ID, &lobbyv1.LobbyStatus{
					LobbyId:        updated.ID,
					CurrentPlayers: int32(playerCount),
					MaxPlayers:     int32(updated.MaxPlayers),
					Status:         lobbyv1.Status_STATUS_STARTING,
					GameId:         "TODO",
				})

				return
			}

			// More wait
			w.streamer.BroadcastLobbyUpdate(updated.ID, &lobbyv1.LobbyStatus{
				LobbyId:        updated.ID,
				CurrentPlayers: int32(playerCount),
				MaxPlayers:     int32(updated.MaxPlayers),
				Status:         lobbyv1.Status_STATUS_WAITING,
			})

		case <-ctx.Done():
			return
		}
	}
}

func (w *Waiter) SectionKey() string {
	return "LOBBY"
}

func (w *Waiter) UpdateConfig(newCfg *Config) error {
	w.mx.Lock()
	defer w.mx.Unlock()

	w.cfg = newCfg

	return nil
}
