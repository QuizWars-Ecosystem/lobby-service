package rules

import (
	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"sync"
	"time"
)

var _ MatchingRule = (*DuelRule)(nil)
var _ abstractions.ConfigSubscriber[*RuleConfig] = (*DuelRule)(nil)

type DuelRule struct {
	RuleConfig
	sync.RWMutex
}

func (r *DuelRule) CanStart(lobby *models.Lobby) bool {
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

func (r *DuelRule) ShouldWait(lobby *models.Lobby) bool {
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

func (r *DuelRule) CanRelax() bool {
	return false
}

func (r *DuelRule) RelaxedRule() MatchingRule {
	return r
}

func (r *DuelRule) GetMaxPlayers() int {
	r.RLock()
	defer r.RUnlock()
	return r.MaxPlayers
}

func (r *DuelRule) GetWaitWindow() time.Duration {
	r.RLock()
	defer r.RUnlock()
	return r.RuleConfig.WaitWindow
}

func (r *DuelRule) GetMaxWaitTime() time.Duration {
	r.RLock()
	defer r.RUnlock()
	return r.RuleConfig.MaxWaitTime
}

func (r *DuelRule) SectionKey() string {
	return "MATCHMAKING_RULE_DUEL"
}

func (r *DuelRule) UpdateConfig(newCfg *RuleConfig) error {
	r.Lock()
	defer r.Unlock()
	r.RuleConfig = *newCfg
	return nil
}
