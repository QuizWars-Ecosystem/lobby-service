package modules

import (
	"context"
	"google.golang.org/grpc"
	"log/slog"
	"testing"
	"time"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"github.com/stretchr/testify/require"
)

func LobbyServiceTest(t *testing.T, client lobbyv1.LobbyServiceClient, cfg *config.TestConfig) {
	prepare(t, cfg)

	t.Run("lobby.JoinLobby", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		for _, p := range players {
			stream, err := client.JoinLobby(ctx, &lobbyv1.JoinLobbyRequest{
				PlayerId:    p.id,
				Rating:      p.rating,
				CategoryIds: p.categories,
				Mode:        classicMode,
			})

			require.NoError(t, err)

			go watchStream(p, stream)
		}

		<-ctx.Done()
	})
}

func watchStream(player player, stream grpc.ServerStreamingClient[lobbyv1.LobbyStatus]) {
	ticker := time.NewTicker(time.Second)
	ctx := stream.Context()

	defer func() {
		_ = stream.CloseSend()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			res, err := stream.Recv()
			if err != nil {
				slog.Error("failed to receive response", err)
			}

			slog.Info("got response: playerID", slog.String("player_id", player.id), slog.Any("response", res))
		}
	}
}
