package streamer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/store"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	updatesLobbyChannelKey     = "lobby.updates.%s"
	joinLobbyRequestSubjectKey = "lobby.join.request.%s"
)

type JoinLobbyRequest struct {
	Player *models.Player `json:"player"`
}

type JoinLobbyResponse struct {
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
}

type StreamManager struct {
	ns            *nats.Conn
	mu            sync.RWMutex
	localStreams  map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]
	remoteStreams map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]
	store         *store.Store
	logger        *zap.Logger
}

func NewStreamManager(ns *nats.Conn, store *store.Store, logger *zap.Logger) *StreamManager {
	return &StreamManager{
		ns:            ns,
		localStreams:  make(map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]),
		remoteStreams: make(map[string]map[string]grpc.ServerStreamingServer[lobbyv1.LobbyStatus]),
		store:         store,
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

func (s *StreamManager) SendJoinLobbyRequest(ctx context.Context, lobbyID string, player *models.Player) (*JoinLobbyResponse, error) {
	subject := fmt.Sprintf(joinLobbyRequestSubjectKey, lobbyID)
	reply := nats.NewInbox()

	respCh := make(chan *JoinLobbyResponse, 1)

	sub, err := s.ns.Subscribe(reply, func(msg *nats.Msg) {
		var resp JoinLobbyResponse
		if err := json.Unmarshal(msg.Data, &resp); err == nil {
			respCh <- &resp
		} else {
			s.logger.Warn("Failed to unmarshal join lobby response", zap.Error(err))
		}
	})
	if err != nil {
		s.logger.Error("NATS subscribe failed", zap.Error(err))
		return nil, err
	}
	defer func() {
		if err = sub.Unsubscribe(); err != nil {
			s.logger.Error("NATS unsubscribe failed", zap.Error(err))
		}
	}()

	req := JoinLobbyRequest{Player: player}
	data, err := json.Marshal(&req)
	if err != nil {
		return nil, err
	}

	if err = s.ns.PublishRequest(subject, reply, data); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		return resp, nil
	case <-time.After(time.Second * 5):
		return nil, errors.New("join lobby response timeout")
	}
}

func (s *StreamManager) subscribeJoinLobbyRequests(lobbyID string) {
	subject := fmt.Sprintf(joinLobbyRequestSubjectKey, lobbyID)

	_, err := s.ns.Subscribe(subject, func(msg *nats.Msg) {
		var req JoinLobbyRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			s.logger.Warn("Invalid join lobby request", zap.Error(err))
			return
		}

		resp := JoinLobbyResponse{
			Accepted: true,
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		if err := s.store.AddPlayer(ctx, lobbyID, req.Player); err != nil {
			s.logger.Warn("Failed to add player to store", zap.String("lobby_id", lobbyID), zap.Error(err))
			resp = JoinLobbyResponse{
				Accepted: false,
				Reason:   err.Error(),
			}
		}

		data, err := json.Marshal(&resp)
		if err != nil {
			s.logger.Warn("Failed to marshal join lobby response", zap.Error(err))
			return
		}

		if err = s.ns.Publish(msg.Reply, data); err != nil {
			s.logger.Warn("Failed to reply join lobby request", zap.Error(err))
		}
	})
	if err != nil {
		s.logger.Error("Failed to subscribe to join lobby requests", zap.Error(err))
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
