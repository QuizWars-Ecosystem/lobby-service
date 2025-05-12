package matcher

import "github.com/QuizWars-Ecosystem/lobby-service/internal/models"

var _ Scorer = (*ClassicScorer)(nil)

type ClassicScorer struct {
	Config ScoringConfig
}

func (s *ClassicScorer) Filter(lobby *models.Lobby, player *models.Player) bool {
	matchRatio := categoryScore(player.Categories, lobby.Categories)
	return matchRatio >= s.Config.MinCategoryMatch
}

func (s *ClassicScorer) Score(lobby *models.Lobby, player *models.Player) float64 {
	return s.Config.RatingWeight*ratingScore(player.Rating, lobby.AvgRating, s.Config.MaxRatingDiff) +
		s.Config.CategoryWeight*categoryScore(player.Categories, lobby.Categories) +
		s.Config.FillWeight*fillScore(len(lobby.Players), int(lobby.MaxPlayers))
}
