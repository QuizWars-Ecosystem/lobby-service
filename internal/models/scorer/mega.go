package scorer

import (
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

var _ Provider = (*MegaScoreProvider)(nil)

type MegaScoreProvider struct{}

func (m *MegaScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	fillScore := float64(len(lobby.Players)) / 128.0

	if fillScore > 0.4 {
		return fillScore * 1.5
	}
	return fillScore
}
