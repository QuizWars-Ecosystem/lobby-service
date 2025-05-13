package lobby

import (
	"context"
	"fmt"
	"time"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/metrics"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"go.uber.org/zap"
)

type State string

const (
	StateWaiting  State = "waiting"
	StateReady    State = "ready"
	StateExpired  State = "expired"
	StateInactive State = "inactive"
	StateError    State = "error"
)

func (w *Waiter) determineState(lobby *models.Lobby) State {
	playerCount := int16(len(lobby.Players))

	switch {
	case time.Now().After(lobby.ExpireAt):
		return StateExpired

	case playerCount == 0 && time.Since(lobby.CreatedAt) > w.getMaxLobbyWait():
		return StateInactive

	case w.isLobbyReady(lobby, playerCount):
		return StateReady

	default:
		return StateWaiting
	}
}

func (w *Waiter) handleState(ctx context.Context, state State, lobby *models.Lobby) error {
	switch state {
	case StateReady:
		return w.handleReadyLobby(ctx, lobby)
	case StateExpired:
		return w.handleExpiredLobby(ctx, lobby)
	case StateInactive:
		return w.handleInactiveLobby(ctx, lobby)
	case StateError:
		return w.handleErrorState(lobby)
	default:
		return w.handleWaitingState(lobby)
	}
}

func (w *Waiter) handleReadyLobby(ctx context.Context, lobby *models.Lobby) error {
	defer metrics.LobbyStatusChanges.WithLabelValues("starting").Inc()

	status := &lobbyv1.LobbyStatus{
		LobbyId:        lobby.ID,
		Status:         lobbyv1.Status_STATUS_STARTING,
		CurrentPlayers: int32(len(lobby.Players)),
		MaxPlayers:     int32(lobby.MaxPlayers),
	}

	w.removeLobby(ctx, lobby, "starting")

	w.broadcastStatus(lobby.ID, status)
	metrics.LobbyWaitTime.WithLabelValues(lobby.Mode).Observe(time.Since(lobby.CreatedAt).Seconds())
	return nil
}

func (w *Waiter) handleExpiredLobby(ctx context.Context, lobby *models.Lobby) error {
	defer metrics.LobbyStatusChanges.WithLabelValues("timeout").Inc()

	status := &lobbyv1.LobbyStatus{
		LobbyId:        lobby.ID,
		Status:         lobbyv1.Status_STATUS_TIMEOUT,
		CurrentPlayers: int32(len(lobby.Players)),
	}

	w.removeLobby(ctx, lobby, "timeout")

	w.broadcastStatus(lobby.ID, status)
	return nil
}

func (w *Waiter) handleWaitingState(lobby *models.Lobby) error {
	playerCount := int16(len(lobby.Players))

	if w.shouldExtendLobby(lobby, playerCount) {
		lobby.ExpireAt = lobby.ExpireAt.Add(w.getLobbyIdleExtend())
	}

	w.broadcastLobbyStatus(lobby.ID, playerCount, lobby.MaxPlayers, lobbyv1.Status_STATUS_WAITING)
	return nil
}

func (w *Waiter) handleInactiveLobby(ctx context.Context, lobby *models.Lobby) error {
	defer metrics.LobbyStatusChanges.WithLabelValues("inactive").Inc()

	w.removeLobby(ctx, lobby, "inactive")

	w.logger.Info("Lobby removed due to inactivity",
		zap.String("lobby_id", lobby.ID),
		zap.Duration("inactive_time", time.Since(lobby.CreatedAt)))

	return nil
}

func (w *Waiter) handleErrorState(lobby *models.Lobby) error {
	defer metrics.LobbyStatusChanges.WithLabelValues("error").Inc()

	status := &lobbyv1.LobbyStatus{
		LobbyId: lobby.ID,
		Status:  lobbyv1.Status_STATUS_ERROR,
	}

	w.streamer.BroadcastLobbyUpdate(lobby.ID, status)

	w.logger.Error("Lobby entered error state",
		zap.String("lobby_id", lobby.ID),
		zap.Any("last_status", status))

	return fmt.Errorf("lobby %s in error state", lobby.ID)
}

func (w *Waiter) removeLobby(ctx context.Context, lobby *models.Lobby, reason string) {
	if err := w.store.RemoveLobby(ctx, lobby.ID, lobby.Mode); err != nil {
		w.logger.Warn("Failed to remove lobby",
			zap.String("reason", reason),
			zap.String("id", lobby.ID),
			zap.Error(err))
	}

	defer metrics.LobbyPlayersCount.DeleteLabelValues(lobby.ID, lobby.Mode)

	w.logger.Debug("Lobby removed",
		zap.String("reason", reason),
		zap.String("lobby_id", lobby.ID))
}

func (w *Waiter) isLobbyReady(lobby *models.Lobby, playerCount int16) bool {
	if playerCount < lobby.MinPlayers {
		return false
	}

	isFull := playerCount >= lobby.MaxPlayers
	minWaitPassed := time.Since(lobby.LastJoinedAt) >= w.getMinReadyDuration()

	return isFull || minWaitPassed
}

func (w *Waiter) shouldExtendLobby(lobby *models.Lobby, playerCount int16) bool {
	if time.Now().After(lobby.ExpireAt) {
		return false
	}
	hasMinPlayers := playerCount >= lobby.MinPlayers
	timeRemaining := time.Until(lobby.ExpireAt)
	shouldExtend := hasMinPlayers && timeRemaining < w.getLobbyIdleExtend()

	if shouldExtend {
		w.logger.Debug("Extending lobby time",
			zap.String("lobby_id", lobby.ID),
			zap.Duration("time_remaining", timeRemaining),
			zap.Int16("players", playerCount))
	}

	return shouldExtend
}

func (w *Waiter) broadcastLobbyStatus(lobbyID string, current, max int16, status lobbyv1.Status) {
	s := &lobbyv1.LobbyStatus{
		LobbyId:        lobbyID,
		CurrentPlayers: int32(current),
		MaxPlayers:     int32(max),
		Status:         status,
	}

	w.streamer.BroadcastLobbyUpdate(lobbyID, s)

	if err := w.streamer.PublishLobbyStatus(lobbyID, s); err != nil {
		w.logger.Warn("Failed to publish lobby status",
			zap.String("id", lobbyID),
			zap.Error(err))
	}
}

func (w *Waiter) broadcastStatus(lobbyID string, status *lobbyv1.LobbyStatus) {
	w.streamer.BroadcastLobbyUpdate(lobbyID, status)

	if err := w.streamer.PublishLobbyStatus(lobbyID, status); err != nil {
		w.logger.Warn("Failed to publish lobby status",
			zap.String("id", lobbyID),
			zap.Error(err))
	}
}

func (w *Waiter) initMetrics(lobby *models.Lobby) {
	metrics.ModeLobbiesCount.WithLabelValues(lobby.Mode).Inc()
	metrics.LobbyPlayersCount.WithLabelValues(lobby.ID, lobby.Mode).Set(float64(len(lobby.Players)))
}

func (w *Waiter) cleanupMetrics(lobby *models.Lobby) {
	metrics.LobbyPlayersCount.DeleteLabelValues(lobby.ID, lobby.Mode)
	metrics.ModeLobbiesCount.WithLabelValues(lobby.Mode).Dec()
}
