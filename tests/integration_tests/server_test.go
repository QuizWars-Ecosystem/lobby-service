package integration_tests

import (
	test "github.com/QuizWars-Ecosystem/go-common/pkg/testing/server"
	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/server"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/clients"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/modules"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test(t *testing.T) {
	testCtx := t.Context()
	cfg := config.NewTestConfig()

	prepareInfrastructure(testCtx, t, cfg, runServer)
}

func runServer(t *testing.T, cfg *config.TestConfig) {
	if cfg.ServerAmount > 1 {

		var srv *server.TestServer
		var err error

		manager := clients.NewManager(t.Context())
		initialPort := cfg.ServiceConfig.ServiceConfig.GRPCPort

		for i := 0; i < cfg.ServerAmount; i++ {
			initialPort++

			serviceConfigCopy := *cfg.ServiceConfig.ServiceConfig
			serviceConfigCopy.GRPCPort = initialPort

			srvCfgCopy := *cfg.ServiceConfig
			srvCfgCopy.ServiceConfig = &serviceConfigCopy

			srv, err = server.NewTestServer(t.Context(), &srvCfgCopy)
			require.NoError(t, err)

			manager.AddServer(clients.ServerSet{
				Server: srv,
				Port:   initialPort,
			})
		}

		defer func() {
			err = manager.Stop()
			require.NoError(t, err)
		}()

		err = manager.Start(t)
		require.NoError(t, err)

		modules.MultiLobbyServiceTest(t, manager, cfg)

	} else {
		t.Logf("Redis URLS: %v", cfg.ServiceConfig.Redis.URLs)

		srv, err := server.NewTestServer(t.Context(), cfg.ServiceConfig)
		require.NoError(t, err)

		conn, stop := test.RunServer(t, srv, cfg.ServiceConfig.GRPCPort)
		defer stop()

		lobbyClient := lobbyv1.NewLobbyServiceClient(conn)

		modules.LobbyServiceTest(t, lobbyClient, cfg)
	}
}
