package matchmaking

import "github.com/QuizWars-Ecosystem/lobby-service/internal/models/matcher"

func (m *Matcher) SectionKey() string {
	return m.lobbyScorer.SectionKey()
}

func (m *Matcher) UpdateConfig(newCfg *matcher.Config) error {
	return m.lobbyScorer.UpdateConfig(newCfg)
}
