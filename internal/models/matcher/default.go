package matcher

import "github.com/QuizWars-Ecosystem/lobby-service/internal/models"

var _ Scorer = (*DefaultScorer)(nil)

type DefaultScorer struct {
	Config ScoringConfig
}

func (s *DefaultScorer) Filter(lobby *models.Lobby, player *models.Player) bool {
	matchRatio := categoryScore(player.Categories, lobby.Categories)
	return matchRatio >= s.Config.MinCategoryMatch
}

func (s *DefaultScorer) Score(lobby *models.Lobby, player *models.Player) float64 {
	return s.Config.RatingWeight*ratingScore(player.Rating, lobby.AvgRating, s.Config.MaxRatingDiff) +
		s.Config.CategoryWeight*categoryScore(player.Categories, lobby.Categories) +
		s.Config.FillWeight*fillScore(len(lobby.Players), int(lobby.MaxPlayers))
}
