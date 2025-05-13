package score

import (
	"sync"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models/scorer"
)

type Provider struct {
	mu     sync.RWMutex
	scorer scorer.Scorer
}

func NewProvider() *Provider {
	return &Provider{
		scorer: &scorer.StandardScorer{},
	}
}

func (p *Provider) Calculate(lobby *models.Lobby) float64 {
	return p.scorer.CalculateScore(lobby)
}
