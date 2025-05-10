package rules

import (
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"time"
)

type MatchingRule interface {
	CanStart(lobby *models.Lobby) bool
	ShouldWait(lobby *models.Lobby) bool
	CanRelax() bool
	RelaxedRule() MatchingRule

	GetMaxPlayers() int
	GetWaitWindow() time.Duration
	GetMaxWaitTime() time.Duration
}

type RuleConfig struct {
	MinPlayers     int
	MaxPlayers     int
	OptimalPlayers int
	CurrentAttempt int
	MaxAttempts    int
	WaitWindow     time.Duration
	MaxWaitTime    time.Duration
	LastRelaxedAt  time.Time
}
