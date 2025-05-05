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

	var prevPlayerCount int
	var status *lobbyv1.LobbyStatus

	for {
		select {
		case <-ticker.C:
			updated, err := w.store.GetLobby(ctx, lobby.ID)
			if err != nil {
				return
			}

			playerCount := len(updated.Players)

			/*w.logger.Debug("LOBBY",
				zap.String("ID", updated.ID),
				zap.Int32s("Categories", updated.Categories),
				zap.Int32("AvgRating", updated.AvgRating),
				zap.Any("Players", updated.Players),
			)

			w.logger.Debug("PLAYERS",
				zap.Int("Current", playerCount),
				zap.Int("Previous", prevPlayerCount),
			)*/

			// If players not come and max time waiting expired
			if playerCount == 0 && time.Since(updated.CreatedAt) > w.cfg.MaxLobbyWait {
				if err = w.store.ExpireLobby(ctx, updated.ID); err != nil {
					w.logger.Warn("Failed to expire lobby", zap.String("id", updated.ID), zap.Error(err))
				}

				w.logger.Info("Lobby removed due to inactivity", zap.String("lobby_id", updated.ID))
				return
			}

			// Is lobby time is expired
			if time.Now().After(updated.ExpireAt) {
				if err = w.store.ExpireLobby(ctx, updated.ID); err != nil {
					w.logger.Warn("Failed to expire lobby", zap.String("id", updated.ID), zap.Error(err))
				}

				status = &lobbyv1.LobbyStatus{
					LobbyId:        updated.ID,
					Status:         lobbyv1.Status_STATUS_TIMEOUT,
					CurrentPlayers: int32(playerCount),
					MaxPlayers:     int32(updated.MaxPlayers),
				}

				w.streamer.BroadcastLobbyUpdate(updated.ID, status)

				if err = w.streamer.PublishLobbyStatus(updated.ID, status); err != nil {
					w.logger.Warn("Failed to publish lobby status", zap.String("id", updated.ID), zap.Error(err))
				}

				w.logger.Info("Lobby expired", zap.String("lobby_id", updated.ID))
				return
			}

			// New Player come in lobby - extend time
			if playerCount > prevPlayerCount && prevPlayerCount != 0 {
				// Calculate how much time remains before expiration
				timeRemaining := time.Until(updated.ExpireAt)

				// If there's not much time left, extend the expiration time
				if timeRemaining < w.cfg.LobbyIdleExtend {
					updated.ExpireAt = updated.ExpireAt.Add(w.cfg.LobbyIdleExtend)
					// w.logger.Debug("New player joined, extending lobby wait", zap.Int("current", playerCount), zap.Int("previous", prevPlayerCount), zap.String("lobby_id", updated.ID))
				}
			}

			// Has min amount of players — can wait more
			if playerCount >= updated.MinPlayers && time.Since(updated.CreatedAt) < w.cfg.MaxLobbyWait {
				// Calculate how much time remains before expiration
				timeRemaining := time.Until(updated.ExpireAt)

				// If there's not much time left, extend the expiration time
				if timeRemaining < w.cfg.LobbyIdleExtend {
					updated.ExpireAt = updated.ExpireAt.Add(w.cfg.LobbyIdleExtend)
					// w.logger.Info("Min players reached, extending wait time", zap.String("lobby_id", updated.ID))
				}
			}

			// If lobby is ready to start (players & time)
			if playerCount >= updated.MinPlayers {
				// Check if the lobby has enough players
				// Calculate the time since the last player joined, if it hasn't passed the minimum waiting time, continue waiting
				lastPlayerJoinedAt := updated.LastJoinedAt // assuming we track the last player join time

				if playerCount == updated.MaxPlayers || time.Since(lastPlayerJoinedAt) >= w.cfg.MinReadyDuration {
					// Lobby is ready to start, send request to Game Router Service
					w.logger.Debug("LOBBY IS READY TO START", zap.String("ID", updated.ID))

					if err = w.store.MarkLobbyAsFull(ctx, updated.ID); err != nil {
						w.logger.Warn("Failed to mark lobby as full", zap.String("id", updated.ID), zap.Error(err))
					}

					status = &lobbyv1.LobbyStatus{
						LobbyId:        updated.ID,
						CurrentPlayers: int32(playerCount),
						MaxPlayers:     int32(updated.MaxPlayers),
						Status:         lobbyv1.Status_STATUS_STARTING,
						GameId:         "TODO",
					}

					w.streamer.BroadcastLobbyUpdate(updated.ID, status)

					if err = w.streamer.PublishLobbyStatus(updated.ID, status); err != nil {
						w.logger.Warn("Failed to publish lobby status", zap.String("id", updated.ID), zap.Error(err))
					}

					return
				}
			}

			/*w.logger.Debug("LOBBY",
				zap.String("ID", updated.ID),
				zap.Int("Current", playerCount),
				zap.Int("Previous", prevPlayerCount),
				zap.Duration("From start", time.Since(updated.CreatedAt)),
				zap.Duration("To end", time.Until(updated.ExpireAt)),
				zap.Duration("Last join", time.Since(updated.LastJoinedAt)),
			)*/

			if playerCount != prevPlayerCount {
				prevPlayerCount = playerCount
			}

			status = &lobbyv1.LobbyStatus{
				LobbyId:        updated.ID,
				CurrentPlayers: int32(playerCount),
				MaxPlayers:     int32(updated.MaxPlayers),
				Status:         lobbyv1.Status_STATUS_WAITING,
			}

			// Wait more
			w.streamer.BroadcastLobbyUpdate(updated.ID, status)

			if err = w.streamer.PublishLobbyStatus(updated.ID, status); err != nil {
				w.logger.Warn("Failed to publish lobby status", zap.String("id", updated.ID), zap.Error(err))
			}

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
