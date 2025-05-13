package matcher

import (
	"math"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

var _ Scorer = (*TeamScorer)(nil)

type TeamScorer struct {
	Config ScoringConfig
}

func (s *TeamScorer) Filter(lobby *models.Lobby, player *models.Player) bool {
	matchRatio := categoryScore(player.Categories, lobby.Categories)
	if matchRatio < s.Config.MinCategoryMatch {
		return false
	}

	if len(lobby.Players) > 0 {
		avgTeamRating := calculateAverageRating(lobby.Players)
		ratingDiff := math.Abs(float64(player.Rating) - avgTeamRating)
		return ratingDiff <= s.Config.MaxRatingDiff*1.5
	}

	return true
}

func (s *TeamScorer) Score(lobby *models.Lobby, player *models.Player) float64 {
	var teamBalanceScore float64
	if len(lobby.Players) > 0 {
		avgTeamRating := calculateAverageRating(lobby.Players)
		teamBalanceScore = 1 - math.Min(1,
			math.Abs(float64(player.Rating)-avgTeamRating)/s.Config.MaxRatingDiff)
	} else {
		teamBalanceScore = 1.0
	}

	return s.Config.RatingWeight*teamBalanceScore +
		s.Config.CategoryWeight*categoryScore(player.Categories, lobby.Categories) +
		s.Config.FillWeight*fillScore(len(lobby.Players), int(lobby.MaxPlayers))
}

func calculateAverageRating(players []*models.Player) float64 {
	sum := 0.0
	for _, p := range players {
		sum += float64(p.Rating)
	}
	return sum / float64(len(players))
}
