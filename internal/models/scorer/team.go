package scorer

import (
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"time"
)

var _ Provider = (*TeamScoreProvider)(nil)

type TeamScoreProvider struct{}

func (t *TeamScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	score := time.Since(lobby.CreatedAt).Seconds()
	score += float64(lobby.MaxPlayers - (lobby.MaxPlayers - int16(len(lobby.Players))))
	return score
}
