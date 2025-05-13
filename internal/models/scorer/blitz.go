package scorer

import (
	"math"
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

var _ Provider = (*BlitzScoreProvider)(nil)

type BlitzScoreProvider struct{}

func (b *BlitzScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	fillScore := float64(len(lobby.Players)) / 6.0
	catScore := math.Min(float64(countUniqueCategories(lobby))/8.0, 1.0)
	waitScore := math.Min(time.Since(lobby.CreatedAt).Minutes()/10.0, 1.0)
	return fillScore*0.5 + catScore*0.3 + waitScore*0.2
}
