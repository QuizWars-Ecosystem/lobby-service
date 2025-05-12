package integration_tests

import (
	"context"
	"fmt"
	"github.com/QuizWars-Ecosystem/go-common/pkg/testing/containers"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"testing"
	"time"
)

type runServerFn func(t *testing.T, cfg *config.TestConfig)

func prepareInfrastructure(
	ctx context.Context,
	t *testing.T,
	cfg *config.TestConfig,
	runServerFn runServerFn,
) {
	natsContainer, err := containers.NewNATSContainer(ctx, cfg.NATS)
	require.NoError(t, err)

	defer testcontainers.CleanupContainer(t, natsContainer)

	cfg.ServiceConfig.NATS.URL = ":4222"

	clusterContainer, err := containers.NewRedisClusterContainers(ctx, cfg.Redis)
	require.NoError(t, err)

	defer testcontainers.CleanupContainer(t, clusterContainer)

	totalNodes := cfg.Redis.Masters + cfg.Redis.Replicas*cfg.Redis.Masters

	exposedPorts := make([]string, totalNodes)
	for i := 0; i < totalNodes; i++ {
		exposedPorts[i] = fmt.Sprintf(":%d", 7000+i)
	}

	cfg.ServiceConfig.Redis.URLs = exposedPorts

	//pingRedis(t, cfg.ServiceConfig.Redis.URLs)

	runServerFn(t, cfg)
}

func pingRedis(t *testing.T, ports []string) {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:          ports,
		DialTimeout:    20 * time.Second,
		MaxRetries:     5,
		PoolSize:       1000,
		MinIdleConns:   200,
		PoolTimeout:    5 * time.Second,
		ReadTimeout:    2 * time.Second,
		WriteTimeout:   2 * time.Second,
		RouteByLatency: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err := client.Ping(ctx).Err()
	require.NoError(t, err)

	t.Logf("Redis URLs: %v", ports)

	client.Shutdown(ctx)
}
