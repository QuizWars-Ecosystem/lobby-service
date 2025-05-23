package modules

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/clients"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/report"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"github.com/stretchr/testify/require"
)

func MultiLobbyServiceTest(t *testing.T, manager *clients.Manager, cfg *config.TestConfig) {
	in := generator(t, cfg)
	r := report.NewResult(cfg.Generator.PlayersCount, cfg)

	t.Run("multi_lobby.JoinLobby", func(t *testing.T) {
		defer func() {
			r.Finish()

			require.NoError(t, r.GenerateHTMLReport())
		}()

		r.Start()

		wgWorkers := sync.WaitGroup{}
		wg := &sync.WaitGroup{}

		workers := cfg.ServerAmount

		wgWorkers.Add(workers)

		for i := 0; i < workers; i++ {
			go func() {
				for p := range in {
					ctx, cancel := context.WithTimeout(context.Background(), time.Minute*4)

					stream, err := manager.GetClient().JoinLobby(ctx, &lobbyv1.JoinLobbyRequest{
						PlayerId:    p.id,
						Rating:      p.rating,
						CategoryIds: p.categories,
						Mode:        p.mode,
					})

					require.NoError(t, err)

					r.ModeInc(p.mode)

					wg.Add(1)

					go watchStream(p, stream, r, wg, cancel)

					time.Sleep(time.Millisecond * 30)
				}

				wgWorkers.Done()
			}()
			time.Sleep(time.Second)
		}

		wgWorkers.Wait()
		r.FinishRequestingMethod()
		wg.Wait()
	})
}
