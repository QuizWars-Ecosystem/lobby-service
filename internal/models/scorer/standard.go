package scorer

import (
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"time"
)

var _ Provider = (*StandardScoreProvider)(nil)

type StandardScoreProvider struct{}

func (s *StandardScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	score := float64(len(lobby.Categories)) + time.Since(lobby.CreatedAt).Seconds()
	score += float64(lobby.MaxPlayers - (lobby.MaxPlayers - int16(len(lobby.Players))))
	return score
}
