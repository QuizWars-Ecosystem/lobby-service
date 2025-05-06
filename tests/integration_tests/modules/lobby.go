package modules

import (
	"context"
	"errors"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/report"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"log/slog"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"github.com/stretchr/testify/require"
)

func LobbyServiceTest(t *testing.T, client lobbyv1.LobbyServiceClient, cfg *config.TestConfig) {
	players := prepare(t, cfg)

	r := report.NewResult(len(players))

	t.Run("lobby.JoinLobby", func(t *testing.T) {
		defer r.LogStats()
		defer r.LogStatsHTML()

		r.StartedAt = time.Now()

		wg := &sync.WaitGroup{}

		for _, p := range players {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)

			stream, err := client.JoinLobby(ctx, &lobbyv1.JoinLobbyRequest{
				PlayerId:    p.id,
				Rating:      p.rating,
				CategoryIds: p.categories,
				Mode:        p.mode,
			})

			r.IncMode(p.mode)

			require.NoError(t, err)

			go watchStream(p, stream, r, wg, cancel)

			diff := rand.IntN(50)
			time.Sleep(time.Millisecond * time.Duration(diff))
		}

		wg.Wait()

		r.FinishedAt = time.Now()
	})
}

func watchStream(player player, stream grpc.ServerStreamingClient[lobbyv1.LobbyStatus], r *report.Result, wg *sync.WaitGroup, cancelFn func()) {
	ctx := stream.Context()
	wg.Add(1)

	defer func() {
		_ = stream.CloseSend()
		cancelFn()
		wg.Done()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		res, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
				return
			}
			slog.Error("failed to receive response", err)
			return
		}

		if res == nil {
			continue
		}

		lobby := r.GetOrCreateLobby(res.LobbyId, func() *report.LobbyStat {
			return &report.LobbyStat{
				Mode:          player.mode,
				Players:       int(res.CurrentPlayers),
				MaxPlayers:    int(res.MaxPlayers),
				RatingSet:     map[string]int32{player.id: player.rating},
				CategoriesSet: map[string][]int32{player.id: player.categories},
				CreatedAt:     time.Now(),
			}
		})

		lobby.Lock()
		switch res.Status {
		case lobbyv1.Status_STATUS_STARTING:
			r.AddToMap(&r.Starter, res.LobbyId)
			lobby.Players = int(res.CurrentPlayers)
			lobby.RatingSet[player.id] = player.rating
			lobby.CategoriesSet[player.id] = player.categories
			if lobby.StartedAt.IsZero() {
				lobby.StartedAt = time.Now()
			}
			lobby.Status = report.StartedStatus

		case lobbyv1.Status_STATUS_WAITING:
			r.AddToMap(&r.WaitedPlayers, player.id)
			lobby.Status = report.WaitedStatus

		case lobbyv1.Status_STATUS_TIMEOUT:
			r.AddToMap(&r.Expired, res.LobbyId)
			r.AddToMap(&r.ExpiredPlayers, player.id)
			lobby.Status = report.ExpiredStatus

		case lobbyv1.Status_STATUS_ERROR:
			r.AddToMap(&r.Errored, res.LobbyId)
			r.AddToMap(&r.ErroredPlayers, player.id)
			lobby.Status = report.ErroredStatus
		}
		lobby.Unlock()
		return
	}
}
