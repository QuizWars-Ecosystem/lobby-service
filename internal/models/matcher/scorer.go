package matcher

import (
	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"sync"
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
		switch mode {
		default:
			scorers["default"] = &DefaultScorer{
				Config: cfg,
			}
		}
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
	return nil
}

func (s *LobbyScorer) GetScorer(mode string) Scorer {
	scorer, ok := s.scorers[mode]
	if !ok {
		s.mx.RLock()
		c := s.config.GetConfig(mode)
		s.mx.RUnlock()

		scorer = &DefaultScorer{
			Config: c,
		}

		s.scorers[mode] = scorer
	}

	return scorer
}
