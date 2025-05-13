package scorer

import (
	"math"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

var _ Provider = (*ClassicScoreProvider)(nil)

type ClassicScoreProvider struct{}

func (c *ClassicScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	catScore := math.Min(float64(countUniqueCategories(lobby))/15.0, 1.0)
	fillScore := float64(len(lobby.Players)) / 10.0

	balanceScore := 1.0
	if len(lobby.Players) > 1 {
		balanceScore = 1 - (calculateRatingSpread(lobby) / 1500.0)
	}

	return catScore*0.5 + fillScore*0.3 + balanceScore*0.2
}
