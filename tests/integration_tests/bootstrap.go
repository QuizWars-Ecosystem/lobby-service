package integration_tests

import (
	"context"
	"fmt"
	"github.com/QuizWars-Ecosystem/go-common/pkg/testing/containers"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"testing"
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

	runServerFn(t, cfg)
}
