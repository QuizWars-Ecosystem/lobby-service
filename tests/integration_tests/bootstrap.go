package integration_tests

import (
	"context"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"strings"
	"testing"
	"time"

	"github.com/QuizWars-Ecosystem/go-common/pkg/testing/containers"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
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

	natsUrl, err := natsContainer.ConnectionString(ctx)
	require.NoError(t, err)

	cfg.ServiceConfig.NATS.URL = strings.TrimPrefix(natsUrl, "nats://")

	clusterContainer, err := containers.NewRedisClusterContainers(ctx, cfg.Redis)
	require.NoError(t, err)

	defer testcontainers.CleanupContainer(t, clusterContainer)

	urls := []string{
		"host.docker.internal:7000",
		"host.docker.internal:7001",
		"host.docker.internal:7002",
		"host.docker.internal:7003",
		"host.docker.internal:7004",
		"host.docker.internal:7005",
	}

	cfg.ServiceConfig.Redis.URLs = urls

	time.Sleep(time.Second * 3)

	runServerFn(t, cfg)
}
