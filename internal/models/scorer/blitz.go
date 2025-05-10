package scorer

import (
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"time"
)

var _ Provider = (*BlitzScoreProvider)(nil)

type BlitzScoreProvider struct{}

func (p *BlitzScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	score := float64(len(lobby.Categories)) + time.Since(lobby.CreatedAt).Seconds()
	score += float64(lobby.MaxPlayers - (lobby.MaxPlayers - int16(len(lobby.Players))))

	return score
}
