package rules

import (
	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"sync"
	"time"
)

var _ MatchingRule = (*StandardRule)(nil)
var _ abstractions.ConfigSubscriber[*RuleConfig] = (*StandardRule)(nil)

type StandardRule struct {
	RuleConfig
	sync.RWMutex
}

func (r *StandardRule) CanStart(lobby *models.Lobby) bool {
	players := len(lobby.Players)
	r.RLock()
	defer r.RUnlock()
	if players >= r.MaxPlayers {
		return true
	}

	if players >= r.OptimalPlayers {
		if time.Since(lobby.LastJoinedAt) > r.WaitWindow {
			return true
		}
	}

	if r.CurrentAttempt >= r.MaxAttempts && players >= r.MinPlayers {
		return true
	}

	return false
}

func (r *StandardRule) ShouldWait(lobby *models.Lobby) bool {
	if r.CanStart(lobby) {
		return false
	}

	r.RLock()
	defer r.RUnlock()

	if r.CurrentAttempt >= r.MaxAttempts {
		return false
	}

	return len(lobby.Players) < r.MaxPlayers
}

func (r *StandardRule) CanRelax() bool {
	r.RLock()
	defer r.RUnlock()
	return r.CurrentAttempt < r.MaxAttempts && time.Since(r.LastRelaxedAt) > 30*time.Second
}

func (r *StandardRule) RelaxedRule() MatchingRule {
	r.RLock()
	defer r.RUnlock()
	return &StandardRule{
		RuleConfig: RuleConfig{
			MinPlayers:     r.MinPlayers - 1,
			MaxPlayers:     r.MaxPlayers,
			OptimalPlayers: r.OptimalPlayers - 1,
			CurrentAttempt: r.CurrentAttempt + 1,
			MaxAttempts:    r.MaxAttempts,
			WaitWindow:     r.WaitWindow + 10*time.Second,
			MaxWaitTime:    r.MaxWaitTime + 10*time.Second,
			LastRelaxedAt:  time.Now(),
		},
	}
}

func (r *StandardRule) GetMaxPlayers() int {
	r.RLock()
	defer r.RUnlock()
	return r.MaxPlayers
}

func (r *StandardRule) GetWaitWindow() time.Duration {
	r.RLock()
	defer r.RUnlock()
	return r.RuleConfig.WaitWindow
}

func (r *StandardRule) GetMaxWaitTime() time.Duration {
	r.RLock()
	defer r.RUnlock()
	return r.RuleConfig.MaxWaitTime
}

func (r *StandardRule) SectionKey() string {
	return "MATCHMAKING_RULE_STANDARD"
}

func (r *StandardRule) UpdateConfig(newCfg *RuleConfig) error {
	r.Lock()
	defer r.Unlock()
	r.RuleConfig = *newCfg
	return nil
}
