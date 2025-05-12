package matcher

import "github.com/QuizWars-Ecosystem/lobby-service/internal/models"

var _ Scorer = (*MegaScorer)(nil)

type MegaScorer struct {
	Config ScoringConfig
}

func (s *MegaScorer) Filter(_ *models.Lobby, _ *models.Player) bool {
	return true
}

func (s *MegaScorer) Score(lobby *models.Lobby, _ *models.Player) float64 {
	return s.Config.FillWeight * fillScore(len(lobby.Players), int(lobby.MaxPlayers))
}
