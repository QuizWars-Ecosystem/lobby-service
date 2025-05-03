package lobby

import (
	"context"
	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/store"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/streamer"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"go.uber.org/zap"
	"sync"
	"time"
)

type Config struct {
	tickerTimeout    time.Duration `mapstructure:"tickerTimeout" default:"1s"`
	maxLobbyWait     time.Duration `mapstructure:"maxLobbyWait" default:"1m"`
	lobbyIdleExtend  time.Duration `mapstructure:"lobbyIdleExtend" default:"5s"`
	minReadyDuration time.Duration `mapstructure:"minReadyDuration" default:"10s"`
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
	ticker := time.NewTicker(w.cfg.tickerTimeout)
	defer ticker.Stop()

	createdAt := time.Now()
	expireAt := createdAt.Add(w.cfg.maxLobbyWait)

	for {
		select {
		case <-ticker.C:
			updated, err := w.store.GetLobby(ctx, lobby.ID)
			if err != nil {
				return
			}

			playerCount := len(updated.Players)

			if playerCount == 0 && time.Since(createdAt) > w.cfg.maxLobbyWait {
				if err = w.store.ExpireLobby(ctx, updated.ID); err != nil {
					w.logger.Warn("Failed to set expire lobby", zap.String("id", updated.ID), zap.Error(err))
				}

				w.logger.Info("Lobby removed due to inactivity", zap.String("lobby_id", updated.ID))
				return
			}

			if time.Now().After(expireAt) {
				if err = w.store.ExpireLobby(ctx, updated.ID); err != nil {
					w.logger.Warn("Failed to set expire lobby", zap.String("id", updated.ID), zap.Error(err))
				}

				w.streamer.BroadcastLobbyUpdate(updated.ID, &lobbyv1.LobbyStatus{
					LobbyId: updated.ID,
					Status:  lobbyv1.Status_STATUS_TIMEOUT,
				})

				w.logger.Info("Lobby expired", zap.String("lobby_id", updated.ID))
				return
			}

			if playerCount >= updated.MinPlayers &&
				(playerCount == updated.MaxPlayers || time.Since(createdAt) > w.cfg.minReadyDuration) {

				/*err := w.gameRouter.StartGame(updated) // <- нужно реализовать
				if err != nil {
					w.logger.Error("failed to start game", zap.Error(err))
					return
				}*/

				if err = w.store.MarkLobbyAsStarted(ctx, updated.ID); err != nil {
					w.logger.Warn("Failed to mark lobby as started", zap.String("id", updated.ID), zap.Error(err))
				}

				w.streamer.BroadcastLobbyUpdate(updated.ID, &lobbyv1.LobbyStatus{
					LobbyId:        updated.ID,
					CurrentPlayers: int32(playerCount),
					MaxPlayers:     int32(updated.MaxPlayers),
					Status:         lobbyv1.Status_STATUS_STARTING,
					GameId:         "TODO: REPACE ON CREATED GAME ID",
				})

				return
			}

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
