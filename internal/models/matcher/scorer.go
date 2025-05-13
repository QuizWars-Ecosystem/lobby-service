package matcher

import (
	"sync"

	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
)

type Scorer interface {
	Filter(*models.Lobby, *models.Player) bool
	Score(*models.Lobby, *models.Player) float64
}

var _ abstractions.ConfigSubscriber[*Config] = (*LobbyScorer)(nil)

type LobbyScorer struct {
	scorers map[string]Scorer
	mx      sync.RWMutex
	config  *Config
}

func NewLobbyScorer(config *Config) *LobbyScorer {
	scorers := make(map[string]Scorer)

	for mode, cfg := range config.Configs {
		scorers[mode] = newScorer(mode, cfg)
	}

	return &LobbyScorer{
		scorers: make(map[string]Scorer),
		config:  config,
	}
}

func (s *LobbyScorer) SectionKey() string {
	return "MATCHER_LOBBY_CONFIG"
}

func (s *LobbyScorer) UpdateConfig(newCfg *Config) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.config = newCfg

	for mode, cfg := range newCfg.Configs {
		s.scorers[mode] = newScorer(mode, cfg)
	}

	return nil
}

func (s *LobbyScorer) GetScorer(mode string) Scorer {
	s.mx.RLock()
	scorer, ok := s.scorers[mode]
	s.mx.RUnlock()

	if ok {
		return scorer
	}

	s.mx.Lock()
	defer s.mx.Unlock()

	if scorer, ok = s.scorers[mode]; ok {
		return scorer
	}

	c := s.config.GetConfig(mode)
	scorer = &DefaultScorer{Config: c}
	s.scorers[mode] = scorer

	return scorer
}

func newScorer(mode string, cfg ScoringConfig) Scorer {
	var s Scorer

	switch mode {
	case "duel":
		s = &DuelScorer{cfg}
	case "battle":
		s = &BattleScorer{cfg}
	case "classic":
		s = &ClassicScorer{cfg}
	case "mega":
		s = &MegaScorer{cfg}
	default:
		s = &DefaultScorer{cfg}
	}

	return s
}
