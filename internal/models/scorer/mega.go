package scorer

import (
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

type MegaScoreProvider struct{}

func (m *MegaScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	score := float64(len(lobby.Players)) + time.Since(lobby.CreatedAt).Seconds()
	score += float64(lobby.MaxPlayers - (lobby.MaxPlayers - int16(len(lobby.Players))))
	return score
}
