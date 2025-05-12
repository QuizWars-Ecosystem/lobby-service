package matcher

import "github.com/QuizWars-Ecosystem/lobby-service/internal/models"

var _ Scorer = (*DuelScorer)(nil)

type DuelScorer struct {
	Config ScoringConfig
}

func (s *DuelScorer) Filter(_ *models.Lobby, _ *models.Player) bool {
	return true
}

func (s *DuelScorer) Score(lobby *models.Lobby, player *models.Player) float64 {
	return s.Config.RatingWeight*ratingScore(player.Rating, lobby.AvgRating, s.Config.MaxRatingDiff) +
		s.Config.CategoryWeight*categoryScore(player.Categories, lobby.Categories)
}
