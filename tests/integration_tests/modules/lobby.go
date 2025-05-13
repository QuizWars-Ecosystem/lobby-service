package modules

import (
	"context"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/report"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"github.com/stretchr/testify/require"
)

func LobbyServiceTest(t *testing.T, client lobbyv1.LobbyServiceClient, cfg *config.TestConfig) {
	in := generator(t, cfg)
	r := report.NewResult(cfg.Generator.PlayersCount, cfg)

	t.Run("multi_lobby.JoinLobby", func(t *testing.T) {
		defer func() {
			r.Finish()

			require.NoError(t, r.GenerateHTMLReport())
		}()

		r.Start()

		wg := &sync.WaitGroup{}

		for p := range in {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)

			stream, err := client.JoinLobby(ctx, &lobbyv1.JoinLobbyRequest{
				PlayerId:    p.id,
				Rating:      p.rating,
				CategoryIds: p.categories,
				Mode:        p.mode,
			})

			require.NoError(t, err)

			r.ModeInc(p.mode)

			wg.Add(1)

			go watchStream(p, stream, r, wg, cancel)

			diff := rand.IntN(10)
			time.Sleep(time.Millisecond * time.Duration(diff))
		}

		r.FinishRequestingMethod()
		wg.Wait()
	})
}
