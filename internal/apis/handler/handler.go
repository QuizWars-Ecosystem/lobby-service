package handler

import (
	"context"
	"errors"
	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"sync"
	"time"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/lobby"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/matchmaking"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/store"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/streamer"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var _ lobbyv1.LobbyServiceServer = (*Handler)(nil)

var _ abstractions.ConfigSubscriber[*Config] = (*Handler)(nil)

type StatPair struct {
	Min int `mapstructure:"min"`
	Max int `mapstructure:"max"`
}

type Handler struct {
	streamer *streamer.StreamManager
	waiter   *lobby.Waiter
	matcher  *matchmaking.Matcher
	store    *store.Store
	logger   *zap.Logger
	mx       sync.RWMutex
	cfg      *Config
}

func NewHandler(
	streamer *streamer.StreamManager,
	waiter *lobby.Waiter,
	matcher *matchmaking.Matcher,
	store *store.Store,
	logger *zap.Logger,
	cfg *Config,
) *Handler {
	return &Handler{
		streamer: streamer,
		waiter:   waiter,
		matcher:  matcher,
		store:    store,
		logger:   logger,
		cfg:      cfg,
	}
}

func (h *Handler) JoinLobby(request *lobbyv1.JoinLobbyRequest, stream grpc.ServerStreamingServer[lobbyv1.LobbyStatus]) error {
	ctx := stream.Context()

	player := &models.Player{
		ID:         request.PlayerId,
		Rating:     request.Rating,
		Categories: request.CategoryIds,
	}

	mode := request.Mode

	var l *models.Lobby

	for attempt := 0; attempt < h.getMaxLobbyAttempts(); attempt++ {
		activeLobbies, err := h.store.GetActiveLobbies(ctx, mode)
		if err != nil {
			h.sendErrorStatus(stream, request.PlayerId)
			return err
		}

		candidateLobbies := h.matcher.FilterLobbies(activeLobbies, player)
		selectedLobby := h.matcher.SelectBestLobby(candidateLobbies, player)

		if selectedLobby == nil {
			break
		}

		err = h.store.AddPlayerToLobby(ctx, selectedLobby.ID, player)
		if err != nil {
			if errors.Is(err, store.ErrLobbyFull) {
				continue
			}

			h.logger.Warn("Failed to add player to lobby", zap.String("lobby_id", selectedLobby.ID), zap.Error(err))
			h.sendErrorStatus(stream, request.PlayerId)
			return err
		}

		if err = h.store.UpdateLobbyTTL(ctx, selectedLobby.ID, h.getLobbyTLL()); err != nil {
			h.logger.Warn("Failed to update lobby TTL", zap.Error(err))
		}

		h.streamer.RegisterStreamWithSubscription(ctx, selectedLobby.ID, player.ID, stream)
		l = selectedLobby
		break
	}

	if l == nil {
		newLobby := &models.Lobby{
			ID:         uuid.New().String(),
			Mode:       mode,
			Categories: request.CategoryIds,
			Players:    []*models.Player{player},
			AvgRating:  player.Rating,
			CreatedAt:  time.Now(),
			ExpireAt:   time.Now().Add(h.getLobbyTLL()),
		}

		h.setLobbyBorders(newLobby)

		if err := h.store.CreateLobby(ctx, newLobby, h.getLobbyTLL()); err != nil {
			h.logger.Warn("Failed to create lobby", zap.Error(err))
			h.sendErrorStatus(stream, request.PlayerId)
			return err
		}

		l = newLobby

		lobbyCtx, cancel := context.WithTimeout(ctx, h.getLobbyTLL())

		defer cancel()

		go h.waiter.WaitForLobbyFill(lobbyCtx, l)

		h.streamer.RegisterStream(l.ID, request.PlayerId, stream)
	}

	<-ctx.Done()
	return nil
}

func (h *Handler) setLobbyBorders(lobby *models.Lobby) {
	pair := h.getModeStats(lobby.Mode)
	lobby.MinPlayers = pair.Min
	lobby.MaxPlayers = pair.Max
}

func (h *Handler) sendErrorStatus(stream grpc.ServerStreamingServer[lobbyv1.LobbyStatus], playerID string) {
	if sendErr := stream.Send(&lobbyv1.LobbyStatus{
		Status: lobbyv1.Status_STATUS_ERROR,
	}); sendErr != nil {
		h.logger.Warn("Failed to send error status", zap.String("player_id", playerID), zap.Error(sendErr))
	}
}
