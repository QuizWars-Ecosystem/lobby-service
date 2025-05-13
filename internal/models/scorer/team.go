package scorer

import (
	"math"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

var _ Provider = (*TeamScoreProvider)(nil)

type TeamScoreProvider struct{}

func (t *TeamScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	fillScore := float64(len(lobby.Players)) / 4.0

	uniqueCats := countUniqueCategories(lobby)
	catScore := math.Min(float64(uniqueCats)/10.0, 1.0)

	balanceScore := 1.0
	if len(lobby.Players) > 1 {
		balanceScore = 1 - (calculateRatingSpread(lobby) / 1000.0)
	}

	return fillScore*0.4 + catScore*0.3 + balanceScore*0.3
}

func countUniqueCategories(lobby *models.Lobby) int {
	unique := make(map[int32]struct{})
	for _, p := range lobby.Players {
		for _, cat := range p.Categories {
			unique[cat] = struct{}{}
		}
	}
	return len(unique)
}

func calculateRatingSpread(lobby *models.Lobby) float64 {
	if len(lobby.Players) < 2 {
		return 0
	}
	var minR, maxR int32
	for i, p := range lobby.Players {
		if i == 0 || p.Rating < minR {
			minR = p.Rating
		}
		if i == 0 || p.Rating > maxR {
			maxR = p.Rating
		}
	}
	return float64(maxR - minR)
}
