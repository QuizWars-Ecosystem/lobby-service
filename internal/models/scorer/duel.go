package scorer

import (
	"math"
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

var _ Provider = (*DuelScoreProvider)(nil)

type DuelScoreProvider struct{}

func (d *DuelScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	fillScore := float64(len(lobby.Players)) / 2.0

	waitBonus := math.Min(time.Since(lobby.CreatedAt).Seconds()/300, 1.0)

	if len(lobby.Players) == 1 {
		return 0.7 + (fillScore * 0.3) + (waitBonus * 0.5)
	}

	return fillScore + waitBonus*0.2
}
