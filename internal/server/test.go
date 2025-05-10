package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/DavidMovas/gopherbox/pkg/closer"
	"github.com/QuizWars-Ecosystem/go-common/pkg/clients"
	"github.com/QuizWars-Ecosystem/go-common/pkg/log"
	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/handler"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/lobby"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/matchmaking"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/store"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/streamer"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/config"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/metrics"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"net"
	"strings"
	"time"
)

type TestServer struct {
	grpcServer *grpc.Server
	listener   net.Listener
	logger     *log.Logger
	cfg        *config.Config
	closer     *closer.Closer
}

func NewTestServer(_ context.Context, cfg *config.Config) (*TestServer, error) {
	cl := closer.NewCloser()

	logger := log.NewLogger(cfg.Local, cfg.Logger.Level)
	cl.PushIO(logger)

	redisClient, err := clients.NewRedisClusterClient(
		clients.NewRedisClusterOptions(cfg.Redis.URLs).
			WithDialTimeout(30*time.Second).
			WithMaxRetries(5).
			WithPoolSize(1000).
			WithMinIdleConns(200).
			WithPoolTimeout(time.Second*5).
			WithReadTimeout(time.Second*2).
			WithWriteTimeout(time.Second*2).
			WithRouterByLatency(true).
			WithBackoffTimeouts(100*time.Millisecond, time.Second),
	)
	if err != nil {
		logger.Zap().Error("error initializing redis client", zap.Error(err))
		return nil, fmt.Errorf("error initializing redis client: %w", err)
	}

	cl.PushIO(redisClient)

	ns, err := clients.NewNATSClient(clients.DefaultNATSOptions.WithURL(cfg.NATS.URL), logger.Zap())
	if err != nil {
		logger.Zap().Error("error initializing nats client", zap.Error(err))
		return nil, fmt.Errorf("error initializing nats client: %w", err)
	}

	cl.PushNE(ns.Close)

	storage := store.NewStore(redisClient, logger.Zap())
	streamManager := streamer.NewStreamManager(ns, storage, logger.Zap())
	matcher := matchmaking.NewMatcher(cfg.Matcher)
	waiter := lobby.NewWaiter(storage, streamManager, logger.Zap(), cfg.Lobby)
	hand := handler.NewHandler(streamManager, waiter, matcher, storage, logger.Zap(), cfg.Handler)

	grpcServer := grpc.NewServer()

	healthServer := health.NewServer()
	healthServer.SetServingStatus(fmt.Sprintf("%s-%d", cfg.Name, cfg.GRPCPort), grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	cl.PushNE(healthServer.Shutdown)

	lobbyv1.RegisterLobbyServiceServer(grpcServer, hand)

	metrics.Initialize()

	return &TestServer{
		grpcServer: grpcServer,
		logger:     logger,
		cfg:        cfg,
		closer:     cl,
	}, nil
}

func (s *TestServer) Start() error {
	z := s.logger.Zap()

	z.Info("Starting server", zap.String("name", s.cfg.Name), zap.Int("port", s.cfg.GRPCPort))

	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.GRPCPort))
	if err != nil {
		z.Error("Failed to start listener", zap.String("name", s.cfg.Name), zap.Int("port", s.cfg.GRPCPort), zap.Error(err))
		return err
	}

	return s.grpcServer.Serve(s.listener)
}

func (s *TestServer) Shutdown(ctx context.Context) error {
	z := s.logger.Zap()
	z.Info("Shutting down server gracefully", zap.String("name", s.cfg.Name))

	stopChan := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopChan)
	}()

	select {
	case <-stopChan:
	case <-ctx.Done():
		z.Warn("Graceful shutdown timed out, forcing stop")
		s.grpcServer.Stop()
	}

	if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("shutting down listener: %w", err)
	}

	if err := s.logger.Close(); err != nil && !isStdoutSyncErr(err) {
		return fmt.Errorf("error closing logger: %w", err)
	}

	return s.closer.Close(ctx)
}

func isStdoutSyncErr(err error) bool {
	return strings.Contains(err.Error(), "sync")
}
