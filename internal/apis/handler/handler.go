package handler

import (
	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	_ lobbyv1.LobbyServiceServer = (*Handler)(nil)
)

type Handler struct {
	logger *zap.Logger
}

func NewHandler(logger *zap.Logger) *Handler {
	return &Handler{
		logger: logger,
	}
}

func (h *Handler) JoinLobby(request *lobbyv1.JoinLobbyRequest, stream grpc.ServerStreamingServer[lobbyv1.LobbyStatus]) error {
	//TODO implement me
	panic("implement me")
}
