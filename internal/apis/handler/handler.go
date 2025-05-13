package handler

import (
	"context"
	"sync"
	"time"

	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"github.com/jaevor/go-nanoid"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/lobby"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/matchmaking"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/store"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/streamer"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var generateId func() string

func init() {
	fn, err := nanoid.Canonic()
	if err != nil {
		panic(err)
	}

	generateId = fn
}

var _ lobbyv1.LobbyServiceServer = (*Handler)(nil)

var _ abstractions.ConfigSubscriber[*Config] = (*Handler)(nil)

type StatPair struct {
	Min int16 `mapstructure:"min"`
	Max int16 `mapstructure:"max"`
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
		activeLobbies, err := h.store.GetTopLobbies(ctx, mode, 100)
		if err != nil {
			h.sendErrorStatus(stream, request.PlayerId)
			return err
		} else if len(activeLobbies) == 0 {
			break
		}

		candidateLobbies := h.matcher.FilterLobbies(mode, activeLobbies, player)
		selectedLobby := h.matcher.SelectBestLobby(mode, candidateLobbies, player)

		if selectedLobby == nil {
			continue
		}

		if err = h.store.AddPlayer(ctx, selectedLobby.ID, player); err != nil {
			// h.logger.Debug("Failed adding player to lobby", zap.String("lobby_id", selectedLobby.ID))
			continue
		}

		if err = h.store.AtomicUpdateLobby(ctx, selectedLobby); err != nil {
			continue
		}

		h.streamer.RegisterStreamWithSubscription(ctx, selectedLobby.ID, player.ID, stream)
		l = selectedLobby
		break
	}

	if l == nil {
		ttl := h.getLobbyTLL()

		newLobby := &models.Lobby{
			ID:         generateId()[0:12],
			Mode:       mode,
			Categories: request.CategoryIds,
			Players:    []*models.Player{player},
			AvgRating:  player.Rating,
			CreatedAt:  time.Now(),
			ExpireAt:   time.Now().Add(ttl),
			Version:    1,
		}

		h.setLobbyBorders(newLobby)

		if err := h.store.AddLobby(ctx, newLobby); err != nil {
			h.logger.Warn("Failed to create lobby", zap.Error(err))
			h.sendErrorStatus(stream, request.PlayerId)
			return err
		}

		l = newLobby

		lobbyCtx, cancel := context.WithTimeout(ctx, ttl)

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
