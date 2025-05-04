package integration_tests

import (
	"context"
	"strings"
	"testing"

	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"

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
	redis, err := containers.NewRedisContainer(ctx, cfg.Redis)
	require.NoError(t, err)

	defer testcontainers.CleanupContainer(t, redis)

	redisUrl, err := redis.ConnectionString(ctx)
	require.NoError(t, err)

	cfg.ServiceConfig.Redis.URL = strings.TrimPrefix(redisUrl, "redis://")

	runServerFn(t, cfg)
}
