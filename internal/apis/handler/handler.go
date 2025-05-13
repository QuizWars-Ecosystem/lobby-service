package handler

import (
	"context"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/metrics"
	"github.com/jaevor/go-nanoid"
	"google.golang.org/grpc/status"
	"sync"
	"time"

	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/lobby"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/matchmaking"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/store"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/streamer"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var _ lobbyv1.LobbyServiceServer = (*Handler)(nil)

var _ abstractions.ConfigSubscriber[*Config] = (*Handler)(nil)

type StatPair struct {
	Min int16 `mapstructure:"min"`
	Max int16 `mapstructure:"max"`
}

type Handler struct {
	streamer   *streamer.StreamManager
	waiter     *lobby.Waiter
	matcher    *matchmaking.Matcher
	store      *store.Store
	logger     *zap.Logger
	mx         sync.RWMutex
	generateId func() string
	cfg        *Config
}

func NewHandler(
	streamer *streamer.StreamManager,
	waiter *lobby.Waiter,
	matcher *matchmaking.Matcher,
	store *store.Store,
	logger *zap.Logger,
	cfg *Config,
) *Handler {
	fn, err := nanoid.Canonic()
	if err != nil {
		logger.Warn("Nanoid canonicalization failed", zap.Error(err))
	}

	return &Handler{
		streamer:   streamer,
		waiter:     waiter,
		matcher:    matcher,
		store:      store,
		logger:     logger,
		generateId: fn,
		cfg:        cfg,
	}
}

func (h *Handler) JoinLobby(request *lobbyv1.JoinLobbyRequest, stream grpc.ServerStreamingServer[lobbyv1.LobbyStatus]) error {
	ctx := stream.Context()
	var err error

	metrics.ActiveGRPCStreams.Inc()
	defer func() {
		defer metrics.ActiveGRPCStreams.Dec()
		if err != nil {
			metrics.GRPCStreamErrors.WithLabelValues(status.Code(err).String()).Inc()
		}
	}()

	player := &models.Player{
		ID:         request.PlayerId,
		Rating:     request.Rating,
		Categories: request.CategoryIds,
	}

	mode := request.Mode

	var activeLobbies []*models.Lobby
	var l *models.Lobby

	activeLobbies, err = h.store.GetTopLobbies(ctx, mode, h.getTopLobbiesLimit())
	if err != nil {
		h.sendErrorStatus(stream, request.PlayerId)
		return err
	}

	if len(activeLobbies) != 0 {
		excludedLobbies := make(map[string]struct{})

		for attempt := 0; attempt < h.getMaxLobbyAttempts(); attempt++ {
			filteredLobbies := make([]*models.Lobby, 0, len(activeLobbies))
			for _, availableLobby := range activeLobbies {
				if _, excluded := excludedLobbies[availableLobby.ID]; !excluded {
					filteredLobbies = append(filteredLobbies, availableLobby)
				}
			}

			if len(filteredLobbies) == 0 {
				break
			}

			candidateLobbies := h.matcher.FilterLobbies(mode, filteredLobbies, player)
			selectedLobby := h.matcher.SelectBestLobby(mode, candidateLobbies, player)

			if selectedLobby == nil {
				continue
			}

			if err = h.store.AddPlayer(ctx, selectedLobby.ID, player); err != nil {
				excludedLobbies[selectedLobby.ID] = struct{}{}
				continue
			}

			h.streamer.RegisterStreamWithSubscription(ctx, selectedLobby.ID, player.ID, stream)
			l = selectedLobby

			metrics.LobbyPlayersCount.WithLabelValues(l.ID, l.Mode).Set(float64(len(l.Players)))

			h.logger.Debug("Lobby was found",
				zap.String("lobby_id", l.ID),
				zap.String("mode", l.Mode),
			)

			break
		}
	}

	if l == nil {
		ttl := h.getLobbyTLL()

		newLobby := &models.Lobby{
			ID:         h.generateId(),
			Mode:       mode,
			Categories: request.CategoryIds,
			Players:    []*models.Player{player},
			AvgRating:  player.Rating,
			CreatedAt:  time.Now(),
			ExpireAt:   time.Now().Add(ttl),
			Version:    1,
		}

		h.setLobbyBorders(newLobby)
		attempts := h.getMaxLobbyAttempts()

		for attempt := 0; attempt < attempts; attempt++ {
			if attempt >= attempts {
				h.logger.Error("Failed to create lobby", zap.Error(err))
				h.sendErrorStatus(stream, request.PlayerId)
				return err
			}

			if err = h.store.AddLobby(ctx, newLobby); err == nil {
				break
			}
		}

		l = newLobby

		lobbyCtx, cancel := context.WithTimeout(ctx, ttl)
		defer cancel()

		go h.waiter.WaitForLobbyFill(lobbyCtx, l)

		h.streamer.RegisterStream(l.ID, request.PlayerId, stream)

		h.logger.Debug("Lobby was created",
			zap.String("lobby_id", l.ID),
			zap.String("mode", l.Mode),
		)
	}

	metrics.ModePlayersQueued.WithLabelValues(l.Mode).Inc()
	defer metrics.ModePlayersQueued.WithLabelValues(l.Mode).Dec()

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
