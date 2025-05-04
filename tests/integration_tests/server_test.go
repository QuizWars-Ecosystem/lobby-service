package integration_tests

import (
	"testing"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/server"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/modules"

	test "github.com/QuizWars-Ecosystem/go-common/pkg/testing/server"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	testCtx := t.Context()
	cfg := config.NewTestConfig()

	prepareInfrastructure(testCtx, t, cfg, runServer)
}

func runServer(t *testing.T, cfg *config.TestConfig) {
	srv, err := server.NewTestServer(t.Context(), cfg.ServiceConfig)
	require.NoError(t, err)

	conn, stop := test.RunServer(t, srv, cfg.ServiceConfig.GRPCPort)
	defer stop()

	lobbyClient := lobbyv1.NewLobbyServiceClient(conn)

	modules.LobbyServiceTest(t, lobbyClient, cfg)
}
