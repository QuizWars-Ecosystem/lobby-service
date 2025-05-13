package modules

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/report"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func watchStream(player player, stream grpc.ServerStreamingClient[lobbyv1.LobbyStatus], r *report.Result, wg *sync.WaitGroup, cancelCtxFn func()) {
	ctx := stream.Context()
	done := make(chan struct{})

	go func() {
		defer func() {
			_ = stream.CloseSend()
			cancelCtxFn()
			wg.Done()
			close(done)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			resCh := make(chan *lobbyv1.LobbyStatus, 1)
			errCh := make(chan error, 1)

			go func() {
				res, err := stream.Recv()
				if err != nil {
					errCh <- err
					return
				}
				resCh <- res
			}()

			select {
			case <-ctx.Done():
				return

			case err := <-errCh:
				if errors.Is(err, io.EOF) ||
					errors.Is(err, context.DeadlineExceeded) ||
					status.Code(err) == codes.DeadlineExceeded {
					return
				}
				slog.Error("failed to receive response", err)
				return

			case res := <-resCh:
				if res == nil {
					continue
				}

				lobby := r.GetOrCreateLobby(res.LobbyId, func() report.LobbyStatCreator {
					return report.LobbyStatCreator{
						Mode:          player.mode,
						Players:       int(res.CurrentPlayers),
						MaxPlayers:    int(res.MaxPlayers),
						RatingSet:     map[string]int32{player.id: player.rating},
						CategoriesSet: map[string][]int32{player.id: player.categories},
						CreatedAt:     time.Now(),
					}
				})

				switch res.Status {
				case lobbyv1.Status_STATUS_STARTING:
					r.AddStartedLobby(res.LobbyId)
					lobby.SetCurrentPlayers(res.CurrentPlayers).
						AddRating(player.id, player.rating).
						AddCategories(player.id, player.categories).
						SetAsStarted()
					cancelCtxFn()

				case lobbyv1.Status_STATUS_WAITING:
					r.AddWaitedPlayer(player.id)
					lobby.SetAsWaited()

				case lobbyv1.Status_STATUS_TIMEOUT:
					r.AddExpiredLobby(res.LobbyId).
						AddExpiredPlayer(player.id)
					lobby.SetAsExpired()
					cancelCtxFn()

				case lobbyv1.Status_STATUS_ERROR:
					r.AddErroredLobby(res.LobbyId).
						AddErroredPlayer(player.id)
					lobby.SetAsErrored()
					cancelCtxFn()
				}
			}
		}
	}()
}
