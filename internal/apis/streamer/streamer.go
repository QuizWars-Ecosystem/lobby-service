package streamer

import (
	"context"
	"encoding/json"
	"fmt"
	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"sync"
)

const (
	updatesLobbyChannelKey = "lobby.updates.%s"
)

type StreamManager struct {
	ns            *nats.Conn
	mu            sync.RWMutex
	localStreams  map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]
	remoteStreams map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]
	logger        *zap.Logger
}

func NewStreamManager(ns *nats.Conn, logger *zap.Logger) *StreamManager {
	return &StreamManager{
		ns:            ns,
		localStreams:  make(map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]),
		remoteStreams: make(map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]),
		logger:        logger,
	}
}

func (s *StreamManager) RegisterStream(lobbyID, playerID string, stream grpc.ServerStreamingServer[lobbyv1.LobbyStatus]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.localStreams[lobbyID] == nil {
		s.localStreams[lobbyID] = make(map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus])
	}

	s.localStreams[lobbyID][playerID] = stream

	go s.watchStream(lobbyID, playerID, stream)
}

func (s *StreamManager) RegisterStreamWithSubscription(
	ctx context.Context,
	lobbyID, playerID string,
	stream grpc.ServerStreamingServer[lobbyv1.LobbyStatus],
) {
	s.mu.Lock()
	if s.remoteStreams[lobbyID] == nil {
		s.remoteStreams[lobbyID] = make(map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus])
	}
	s.remoteStreams[lobbyID][playerID] = stream
	s.mu.Unlock()

	subject := fmt.Sprintf(updatesLobbyChannelKey, lobbyID)

	subscription, err := s.ns.Subscribe(subject, func(msg *nats.Msg) {
		var status lobbyv1.LobbyStatus
		if err := json.Unmarshal(msg.Data, &status); err != nil {
			s.logger.Warn("Failed to unmarshal NATS message", zap.Error(err))
			return
		}

		if err := stream.Send(&status); err != nil {
			s.logger.Warn("Failed to send lobby status over stream", zap.String("player_id", playerID), zap.Error(err))
			return
		}
	})

	if err != nil {
		s.logger.Error("Failed to subscribe to NATS channel", zap.String("subject", subject), zap.Error(err))
		return
	}

	go func() {
		<-ctx.Done()
		_ = subscription.Unsubscribe()
		s.mu.Lock()
		delete(s.remoteStreams[lobbyID], playerID)
		if len(s.remoteStreams[lobbyID]) == 0 {
			delete(s.remoteStreams, lobbyID)
		}
		s.mu.Unlock()
	}()
}

func (s *StreamManager) PublishLobbyStatus(lobbyID string, status *lobbyv1.LobbyStatus) error {
	subject := fmt.Sprintf(updatesLobbyChannelKey, lobbyID)
	data, err := json.Marshal(status)
	if err != nil {
		s.logger.Error("Failed to marshal lobby status", zap.String("lobby_id", lobbyID), zap.Error(err))
		return err
	}

	if err = s.ns.Publish(subject, data); err != nil {
		s.logger.Error("Failed to publish lobby status", zap.String("lobby_id", lobbyID), zap.Error(err))
		return err
	}

	return nil
}

func (s *StreamManager) BroadcastLobbyUpdate(lobbyID string, status *lobbyv1.LobbyStatus) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, stream := range s.localStreams[lobbyID] {
		if err := stream.Send(status); err != nil {
			delete(s.localStreams[lobbyID], id)
		}
	}
}

func (s *StreamManager) watchStream(lobbyID, playerID string, stream grpc.ServerStreamingServer[lobbyv1.LobbyStatus]) {
	ctx := stream.Context()

	select {
	case <-ctx.Done():
		s.mu.Lock()
		if s.localStreams[lobbyID] != nil && s.localStreams[lobbyID][playerID] != nil {
			delete(s.localStreams[lobbyID], playerID)
		}
		s.mu.Unlock()
		return
	}
}
