package lobby

import (
	"context"
	"sync"
	"time"

	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/store"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/streamer"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"go.uber.org/zap"
)

var _ abstractions.ConfigSubscriber[*Config] = (*Waiter)(nil)

type Waiter struct {
	store    *store.Store
	streamer *streamer.StreamManager
	logger   *zap.Logger
	mx       sync.RWMutex
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
	defer w.cleanupMetrics(lobby)

	w.initMetrics(lobby)
	ticker := time.NewTicker(w.getTickerTimeout())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			updated, err := w.store.GetLobby(ctx, lobby.ID)
			if err != nil {
				if err = w.handleState(ctx, StateError, lobby); err != nil {
					w.logger.Warn("Failed to handle lobby state update",
						zap.String("lobby_id", lobby.ID),
						zap.Error(err),
					)
				}
				return
			}

			state := w.determineState(updated)
			if err = w.handleState(ctx, state, updated); err != nil {
				w.logger.Error("State handling failed",
					zap.String("state", string(state)),
					zap.Error(err))
				return
			}

			if state != StateWaiting {
				return
			}

		case <-ctx.Done():
			return
		}
	}
}
