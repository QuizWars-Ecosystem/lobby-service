package modules

import (
	"context"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/report"
	"google.golang.org/grpc"
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

	t.Run("lobby.JoinLobby", func(t *testing.T) {
		r := &report.Result{
			TotalPlayers:   len(players),
			Lobbies:        make(map[string]report.LobbyStat),
			Modes:          make(map[string]int),
			Starter:        make(map[string]struct{}),
			WaitedPlayers:  make(map[string]struct{}),
			Expired:        make(map[string]struct{}),
			ExpiredPlayers: make(map[string]struct{}),
			Errored:        make(map[string]struct{}),
			ErroredPlayers: make(map[string]struct{}),
		}

		defer r.LogStats()
		defer r.LogStatsHTML()

		wg := &sync.WaitGroup{}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
		defer cancel()

		for _, p := range players {
			stream, err := client.JoinLobby(ctx, &lobbyv1.JoinLobbyRequest{
				PlayerId:    p.id,
				Rating:      p.rating,
				CategoryIds: p.categories,
				Mode:        p.mode,
			})

			r.Modes[p.mode]++

			require.NoError(t, err)

			go watchStream(p, stream, r, wg)

			diff := rand.IntN(500)
			time.Sleep(time.Duration(diff) * time.Millisecond)
		}

		wg.Wait()
	})
}

func watchStream(player player, stream grpc.ServerStreamingClient[lobbyv1.LobbyStatus], r *report.Result, wg *sync.WaitGroup) {
	ticker := time.NewTicker(time.Second)
	ctx := stream.Context()

	wg.Add(1)

	time.AfterFunc(time.Minute, func() {
		_ = stream.CloseSend()
		defer wg.Done()
	})

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			res, err := stream.Recv()
			if err != nil {
				slog.Error("failed to receive response", err)
			}

			if res != nil {
				switch res.Status {
				case lobbyv1.Status_STATUS_STARTING:
					r.Lock()
					r.Starter[res.LobbyId] = struct{}{}

					if l, ok := r.Lobbies[res.LobbyId]; ok {
						l.Players = int(res.CurrentPlayers)
						if _, rsOk := l.RatingSet[player.id]; !rsOk {
							l.RatingSet[player.id] = player.rating
						}

						if _, csOk := l.CategoriesSet[player.id]; !csOk {
							l.CategoriesSet[player.id] = player.categories
						}

						l.StartedAt = time.Now()
						l.Status = report.StartedStatus
						r.Lobbies[res.LobbyId] = l
					} else {
						newLobbyStat := report.LobbyStat{
							Mode:       player.mode,
							Players:    int(res.CurrentPlayers),
							MaxPlayers: int(res.MaxPlayers),
							RatingSet: map[string]int32{
								player.id: player.rating,
							},
							CategoriesSet: map[string][]int32{
								player.id: player.categories,
							},
							Status: report.StartedStatus,
						}

						r.Lobbies[res.LobbyId] = newLobbyStat
					}
					r.Unlock()
					return
				case lobbyv1.Status_STATUS_WAITING:
					r.Lock()
					r.WaitedPlayers[player.id] = struct{}{}

					if _, ok := r.Lobbies[res.LobbyId]; !ok {
						newLobbyStat := report.LobbyStat{
							Mode:       player.mode,
							Players:    int(res.CurrentPlayers),
							MaxPlayers: int(res.MaxPlayers),
							RatingSet: map[string]int32{
								player.id: player.rating,
							},
							CategoriesSet: map[string][]int32{
								player.id: player.categories,
							},
							CreatedAt: time.Now(),
							Status:    report.WaitedStatus,
						}
						r.Lobbies[res.LobbyId] = newLobbyStat

					}
					r.Unlock()
				case lobbyv1.Status_STATUS_TIMEOUT:
					r.Lock()
					r.Expired[res.LobbyId] = struct{}{}
					r.ExpiredPlayers[player.id] = struct{}{}
					if l, ok := r.Lobbies[res.LobbyId]; ok {
						l.Status = report.ExpiredStatus
						r.Lobbies[res.LobbyId] = l
					}
					r.Unlock()
					return
				case lobbyv1.Status_STATUS_ERROR:
					r.Lock()
					r.Errored[res.LobbyId] = struct{}{}
					r.ErroredPlayers[player.id] = struct{}{}
					if l, ok := r.Lobbies[res.LobbyId]; ok {
						l.Status = report.ErroredStatus
						r.Lobbies[res.LobbyId] = l
					}
					r.Unlock()
					return
				}
			}
		}
	}
}
