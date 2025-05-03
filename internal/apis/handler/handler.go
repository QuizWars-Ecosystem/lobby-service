package handler

import (
	"context"
	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/lobby"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/matchmaking"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/store"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/streamer"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"time"
)

var (
	_ lobbyv1.LobbyServiceServer = (*Handler)(nil)
)

type Config struct {
	modeStats map[string]StatPair `mapstructure:"mode_stats" yaml:"mode_stats"`
}

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
	cfg      *Config
}

func NewHandler(
	streamer *streamer.StreamManager,
	waiter *lobby.Waiter,
	matcher *matchmaking.Matcher,
	store *store.Store,
	logger *zap.Logger) *Handler {
	return &Handler{
		streamer: streamer,
		waiter:   waiter,
		matcher:  matcher,
		store:    store,
		logger:   logger,
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

	openLobbies, err := h.store.GetOpenLobbies(ctx, mode)
	if err != nil {
		if err = stream.Send(&lobbyv1.LobbyStatus{
			Status: lobbyv1.Status_STATUS_ERROR,
		}); err != nil {
			h.logger.Warn("Failed to send response", zap.String("player_id", request.PlayerId), zap.Error(err))
			return err
		}
		return err
	}

	candidateLobbies := h.matcher.FilterLobbies(openLobbies, player)
	selectedLobby := h.matcher.SelectBestLobby(candidateLobbies, player)

	var l *models.Lobby
	if selectedLobby != nil {
		err = h.store.AddPlayerToLobby(ctx, selectedLobby.ID, player)
		if err != nil {
			h.logger.Warn("Failed to add player to l", zap.Error(err))

			if err = stream.Send(&lobbyv1.LobbyStatus{
				Status: lobbyv1.Status_STATUS_ERROR,
			}); err != nil {
				h.logger.Warn("Failed to send response", zap.String("player_id", request.PlayerId), zap.Error(err))
				return err
			}

			return err
		}

		if err = h.store.UpdateLobbyTTL(ctx, selectedLobby.ID, time.Minute*2); err != nil {
			h.logger.Warn("Failed to update l TTL", zap.Error(err))
		}

		l = selectedLobby
	} else {
		newLobby := &models.Lobby{
			ID:         uuid.New().String(),
			Mode:       mode,
			Categories: request.CategoryIds,
			Players:    []*models.Player{player},
		}

		h.setLobbyBorders(newLobby)

		if err = h.store.CreateLobby(ctx, l, time.Minute*2); err != nil {
			h.logger.Warn("Failed to create l", zap.Error(err))

			if err = stream.Send(&lobbyv1.LobbyStatus{
				Status: lobbyv1.Status_STATUS_ERROR,
			}); err != nil {
				h.logger.Warn("Failed to send response", zap.String("player_id", request.PlayerId), zap.Error(err))
				return err
			}

			return err
		}

		l = newLobby

		lobbyCtx, cancel := context.WithTimeout(ctx, time.Minute*4)
		defer cancel()

		go h.waiter.WaitForLobbyFill(lobbyCtx, l)
	}

	h.streamer.RegisterStream(l.ID, request.PlayerId, stream)

	<-ctx.Done()

	return nil
}

func (h *Handler) setLobbyBorders(lobby *models.Lobby) {
	pair, ok := h.cfg.modeStats[lobby.Mode]

	if !ok {
		h.logger.Warn("Lobby mode not found in config", zap.String("mode", lobby.Mode))
		lobby.MinPlayers = 4
		lobby.MaxPlayers = 8
		return
	}

	lobby.MinPlayers = pair.Min
	lobby.MaxPlayers = pair.Max
	return
}
