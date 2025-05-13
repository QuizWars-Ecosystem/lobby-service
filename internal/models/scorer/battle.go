package scorer

import (
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

var _ Provider = (*BattleScoreProvider)(nil)

type BattleScoreProvider struct{}

func (b *BattleScoreProvider) CalculateScore(lobby *models.Lobby) float64 {
	score := float64(len(lobby.Categories)) + time.Since(lobby.CreatedAt).Seconds()

	if int16(len(lobby.Players)) == 2 {
		score += 100
	} else if int16(len(lobby.Players)) == 3 {
		score += 150
	} else if int16(len(lobby.Players)) == 4 {
		score += 200
	}

	return score
}
