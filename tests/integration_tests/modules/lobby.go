package modules

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"github.com/stretchr/testify/require"
)

type lobbyStat struct {
	mode       string
	players    int
	maxPlayers int
}

type result struct {
	totalPlayers int
	lobbies      map[string]lobbyStat
	starter      map[string]struct{}
	expired      map[string]struct{}
	errored      map[string]struct{}
	sync.Mutex
}

func LobbyServiceTest(t *testing.T, client lobbyv1.LobbyServiceClient, cfg *config.TestConfig) {
	players := prepare(t, cfg)

	t.Run("lobby.JoinLobby", func(t *testing.T) {
		r := &result{
			totalPlayers: len(players),
			lobbies:      make(map[string]lobbyStat),
			starter:      make(map[string]struct{}),
			expired:      make(map[string]struct{}),
			errored:      make(map[string]struct{}),
		}

		defer r.LogStats()

		wg := &sync.WaitGroup{}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
		defer cancel()

		for _, p := range players {
			stream, err := client.JoinLobby(ctx, &lobbyv1.JoinLobbyRequest{
				PlayerId:    p.id,
				Rating:      p.rating,
				CategoryIds: p.categories,
				Mode:        p.mode,
			})

			require.NoError(t, err)

			go watchStream(p, stream, r, wg)
		}

		wg.Wait()
	})
}

func watchStream(player player, stream grpc.ServerStreamingClient[lobbyv1.LobbyStatus], r *result, wg *sync.WaitGroup) {
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
					r.starter[res.LobbyId] = struct{}{}
					r.lobbies[res.LobbyId] = lobbyStat{
						mode:       player.mode,
						players:    int(res.CurrentPlayers),
						maxPlayers: int(res.MaxPlayers),
					}
					r.Unlock()

					slog.Info("Starting lobby", slog.String("ID", player.id))
					return
				case lobbyv1.Status_STATUS_TIMEOUT:
					r.Lock()
					r.expired[res.LobbyId] = struct{}{}
					r.Unlock()
					return
				case lobbyv1.Status_STATUS_ERROR:
					r.Lock()
					r.errored[res.LobbyId] = struct{}{}
					r.Unlock()
					return
				}
			}
		}
	}
}

func (r *result) LogStats() {
	r.Lock()
	defer r.Unlock()

	type statRow struct {
		id    string
		mode  string
		count int
		max   int
	}

	rows := make([]statRow, 0, len(r.lobbies))
	var playersInLobbies int

	for id, stat := range r.lobbies {
		rows = append(rows, statRow{
			id:    id,
			mode:  stat.mode,
			count: stat.players,
			max:   stat.maxPlayers,
		})
		playersInLobbies += stat.players
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].count > rows[j].count
	})

	percent := 0.0
	if r.totalPlayers > 0 {
		percent = float64(playersInLobbies) / float64(r.totalPlayers) * 100
	}

	fmt.Println("📊  Lobby Statistics Summary")
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Printf("👥  Total Players:       %d\n", r.totalPlayers)
	fmt.Printf("🏟️  Total Lobbies:       %d\n", len(r.lobbies))
	fmt.Printf("🔢  Players in Lobbies:  %d (%.1f%%)\n", playersInLobbies, percent)
	fmt.Printf("🚀  Started Lobbies:     %d\n", len(r.starter))
	fmt.Printf("⌛  Expired Lobbies:     %d\n", len(r.expired))
	fmt.Printf("❌  Errored Lobbies:     %d\n", len(r.errored))
	fmt.Println()

	if len(rows) == 0 {
		fmt.Println("ℹ️  No active lobbies found.")
		return
	}

	fmt.Println("📋  Active Lobbies:")
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Printf(" %-36s │ %-12s │ %-10s\n", "🆔 Lobby ID", "🎮 Mode", "👥 Players")
	fmt.Println("─────────────────────────────────────────────────────────────")

	for _, row := range rows {
		playerStr := fmt.Sprintf("%d/%d", row.count, row.max)
		fmt.Printf(" %-36s │ %-12s │ %-10s\n", row.id, row.mode, playerStr)
	}

	fmt.Println("─────────────────────────────────────────────────────────────")
}
