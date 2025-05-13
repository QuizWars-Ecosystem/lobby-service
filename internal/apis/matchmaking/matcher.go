package matchmaking

import (
	"sync"

	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models/matcher"
)

var _ abstractions.ConfigSubscriber[*matcher.Config] = (*Matcher)(nil)

type Matcher struct {
	mx          sync.RWMutex
	lobbyScorer *matcher.LobbyScorer
}

func NewMatcher(cfg *matcher.Config) *Matcher {
	return &Matcher{
		lobbyScorer: matcher.NewLobbyScorer(cfg),
	}
}

func (m *Matcher) FilterLobbies(mode string, lobbies []*models.Lobby, player *models.Player) []*models.Lobby {
	var result []*models.Lobby

	scorer := m.lobbyScorer.GetScorer(mode)

	for _, l := range lobbies {
		if scorer.Filter(l, player) {
			result = append(result, l)
		}
	}

	return result
}

func (m *Matcher) SelectBestLobby(mode string, lobbies []*models.Lobby, player *models.Player) *models.Lobby {
	var (
		best   *models.Lobby
		bestSc float64
	)

	scorer := m.lobbyScorer.GetScorer(mode)

	for _, l := range lobbies {
		score := scorer.Score(l, player)

		if best == nil || score > bestSc {
			best = l
			bestSc = score
		}
	}

	return best
}
