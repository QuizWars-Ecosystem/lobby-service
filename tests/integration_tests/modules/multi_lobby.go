package modules

import (
	"context"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/clients"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/report"
	"sync"
	"testing"
	"time"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"github.com/stretchr/testify/require"
)

func MultiLobbyServiceTest(t *testing.T, manager *clients.Manager, cfg *config.TestConfig) {
	in := generator(t, cfg)
	r := report.NewResult(cfg.Generator.PlayersCount)

	t.Run("multi_lobby.JoinLobby", func(t *testing.T) {
		defer func() {
			r.Finish()

			r.LogStatsHTML()
		}()

		r.Start()

		wgWorkers := sync.WaitGroup{}
		wg := &sync.WaitGroup{}

		workers := cfg.ServerAmount * 2

		wgWorkers.Add(workers)

		for i := 0; i < workers; i++ {
			go func() {
				for p := range in {
					ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)

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

					time.Sleep(time.Millisecond * 20)
				}

				wgWorkers.Done()
			}()
		}

		wgWorkers.Wait()
		r.FinishRequesting()
		wg.Wait()
	})
}
