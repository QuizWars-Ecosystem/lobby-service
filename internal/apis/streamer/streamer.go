package streamer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"sync"
	"time"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	updatesLobbyChannelKey = "lobby_updates:%s"
)

type StreamManager struct {
	redisClient   redis.UniversalClient
	mu            sync.RWMutex
	localStreams  map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]
	remoteStreams map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]
	logger        *zap.Logger
}

func NewStreamManager(redisClient redis.UniversalClient, logger *zap.Logger) *StreamManager {
	return &StreamManager{
		redisClient:   redisClient,
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

	ps := s.redisClient.Subscribe(ctx, fmt.Sprintf(updatesLobbyChannelKey, lobbyID))

	go func() {
		defer func() {
			_ = ps.Close()
			s.mu.Lock()
			delete(s.remoteStreams[lobbyID], playerID)
			if len(s.remoteStreams[lobbyID]) == 0 {
				delete(s.remoteStreams, lobbyID)
			}
			s.mu.Unlock()
		}()

		ch := ps.Channel()

		for {
			select {
			case <-ctx.Done():
				return

			case msg, ok := <-ch:
				if !ok {
					return
				}

				var status lobbyv1.LobbyStatus
				if err := json.Unmarshal([]byte(msg.Payload), &status); err != nil {
					s.logger.Warn("Failed to unmarshal channel message", zap.Error(err))
					continue
				}

				/*s.logger.Debug("Got PubSub message",
					zap.String("channel", msg.Channel),
					zap.String("playerID", playerID),
					zap.String("status", status.Status.String()),
					zap.Int32("players", status.CurrentPlayers),
				)*/

				if err := stream.Send(&status); err != nil {
					s.logger.Warn("Failed to send lobby status over stream", zap.String("player_id", playerID), zap.Error(err))
					return
				}
			}
		}
	}()
}

func (s *StreamManager) PublishLobbyStatus(lobbyID string, status *lobbyv1.LobbyStatus) error {
	channel := fmt.Sprintf(updatesLobbyChannelKey, lobbyID)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	data, err := json.Marshal(status)
	if err != nil {
		s.logger.Error("Failed to marshal lobby status", zap.String("lobby_id", lobbyID), zap.Error(err))
		return err
	}

	if err = s.redisClient.Publish(ctx, channel, data).Err(); err != nil {
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
