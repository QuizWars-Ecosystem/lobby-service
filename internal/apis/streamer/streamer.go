package streamer

import (
	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"sync"
)

type StreamManager struct {
	mu      sync.RWMutex
	streams map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]
	logger  *zap.Logger
}

func NewStreamManager(logger *zap.Logger) *StreamManager {
	return &StreamManager{
		streams: make(map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]),
		logger:  logger,
	}
}

func (s *StreamManager) RegisterStream(lobbyID, playerID string, stream grpc.ServerStreamingServer[lobbyv1.LobbyStatus]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.streams[lobbyID] == nil {
		s.streams[lobbyID] = make(map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus])
	}

	s.streams[lobbyID][playerID] = stream

	go s.watchStream(lobbyID, playerID, stream)
}

func (s *StreamManager) BroadcastLobbyUpdate(lobbyID string, status *lobbyv1.LobbyStatus) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, stream := range s.streams[lobbyID] {
		if err := stream.Send(status); err != nil {
			delete(s.streams[lobbyID], id)
		}
	}
}

func (s *StreamManager) watchStream(lobbyID, playerID string, stream grpc.ServerStreamingServer[lobbyv1.LobbyStatus]) {
	ctx := stream.Context()

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			if s.streams[lobbyID] != nil && s.streams[lobbyID][playerID] != nil {
				delete(s.streams[lobbyID], playerID)
			}
			s.mu.Unlock()
			return
		}
	}
}
