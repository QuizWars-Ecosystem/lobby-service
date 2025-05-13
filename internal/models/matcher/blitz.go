package matcher

import (
	"math"
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

var _ Scorer = (*BlitzScorer)(nil)

type BlitzScorer struct {
	Config ScoringConfig
}

func (s *BlitzScorer) Filter(lobby *models.Lobby, player *models.Player) bool {
	matchRatio := categoryScore(player.Categories, lobby.Categories)
	return matchRatio >= s.Config.MinCategoryMatch
}

func (s *BlitzScorer) Score(lobby *models.Lobby, player *models.Player) float64 {
	waitTimeScore := 1 - math.Min(1, time.Since(lobby.CreatedAt).Minutes()/10)

	return s.Config.RatingWeight*ratingScore(player.Rating, lobby.AvgRating, s.Config.MaxRatingDiff) +
		s.Config.CategoryWeight*categoryScore(player.Categories, lobby.Categories) +
		s.Config.FillWeight*(fillScore(len(lobby.Players), int(lobby.MaxPlayers))*0.7+waitTimeScore*0.3)
}
