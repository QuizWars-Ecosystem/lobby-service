package scorer

import (
	"sync"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

var defaultProviders = map[string]Provider{
	"classic": &StandardScoreProvider{},
	"battle":  &BattleScoreProvider{},
	"blitz":   &BlitzScoreProvider{},
	"mega":    &MegaScoreProvider{},
	"team":    &TeamScoreProvider{},
	"dual":    &DuelScoreProvider{},
}

type Provider interface {
	CalculateScore(lobby *models.Lobby) float64
}

type ScoreProviders struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

func NewScoreProviders() *ScoreProviders {
	return &ScoreProviders{
		providers: defaultProviders,
	}
}

func (p *ScoreProviders) GetProvider(name string) Provider {
	p.mu.Lock()
	defer p.mu.Unlock()

	provider, ok := p.providers[name]
	if !ok {
		provider = &StandardScoreProvider{}
		p.providers[name] = provider
	}

	return provider
}
