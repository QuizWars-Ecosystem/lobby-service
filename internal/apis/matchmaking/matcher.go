package matchmaking

import (
	"math"

	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models/matcher"
)

var _ abstractions.ConfigSubscriber[*matcher.Config] = (*Matcher)(nil)

type Matcher struct {
	lobbyScorer *matcher.LobbyScorer
}

func NewMatcher(cfg *matcher.Config) *Matcher {
	return &Matcher{
		lobbyScorer: matcher.NewLobbyScorer(cfg),
	}
}

func (m *Matcher) FilterLobbies(mode string, lobbies []*models.Lobby, player *models.Player) []*models.Lobby {
	scorer := m.lobbyScorer.GetScorer(mode)
	result := make([]*models.Lobby, 0, len(lobbies))

	for _, l := range lobbies {
		if scorer.Filter(l, player) {
			result = append(result, l)
		}
	}

	return result
}

func (m *Matcher) SelectBestLobby(mode string, lobbies []*models.Lobby, player *models.Player) *models.Lobby {
	if len(lobbies) == 0 {
		return nil
	}

	scorer := m.lobbyScorer.GetScorer(mode)
	var (
		bestLobby *models.Lobby
		bestScore = math.Inf(-1)
	)

	for _, lobby := range lobbies {
		if score := scorer.Score(lobby, player); score > bestScore {
			bestLobby = lobby
			bestScore = score
		}
	}
	return bestLobby
}
