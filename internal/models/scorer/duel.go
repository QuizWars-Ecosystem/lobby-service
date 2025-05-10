package scorer

import (
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"time"
)

type DuelScoreProvider struct{}

func (d *DuelScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	if len(lobby.Players) == 0 {
		return time.Since(lobby.CreatedAt).Seconds()
	}
	return float64(lobby.AvgRating)
}
