package scorer

import (
	"math"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

var _ Provider = (*BattleScoreProvider)(nil)

type BattleScoreProvider struct{}

func (b *BattleScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	ratingScore := 1.0
	if len(lobby.Players) >= 2 {
		spread := calculateRatingSpread(lobby)
		ratingScore = 1 - math.Min(spread/800.0, 1.0)
	}

	uniqueCats := countUniqueCategories(lobby)
	catScore := math.Min(float64(uniqueCats)/6.0, 1.0)

	fillScore := float64(len(lobby.Players)) / 4.0

	return ratingScore*0.6 + catScore*0.25 + fillScore*0.15
}
