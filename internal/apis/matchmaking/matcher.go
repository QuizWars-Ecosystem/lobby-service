package matchmaking

import (
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"math"
	"sort"
)

type Matcher struct {
	maxRatingDelta, minCategoryMatch int
}

func NewMatcher(maxRatingDelta, minCategoryMatch int) *Matcher {
	return &Matcher{
		maxRatingDelta:   maxRatingDelta,
		minCategoryMatch: minCategoryMatch,
	}
}

func (m *Matcher) FilterLobbies(lobbies []*models.Lobby, player *models.Player) []*models.Lobby {
	var matches []*models.Lobby

	for _, l := range lobbies {
		if int(math.Abs(float64(player.Rating-l.AvgRating))) <= m.maxRatingDelta &&
			countIntersect(player.Categories, l.Categories) >= m.minCategoryMatch {
			matches = append(matches, l)
		}
	}

	return matches
}

func (m *Matcher) SelectBestLobby(lobbies []*models.Lobby, player *models.Player) *models.Lobby {
	type scored struct {
		lobby *models.Lobby
		score int
	}

	var scoredList []scored

	for _, l := range lobbies {
		score := countIntersect(player.Categories, l.Categories)*10 - int(math.Abs(float64(player.Rating-l.AvgRating))/10)
		scoredList = append(scoredList, scored{l, score})
	}

	sort.Slice(scoredList, func(i, j int) bool {
		return scoredList[i].score > scoredList[j].score
	})

	if len(scoredList) == 0 {
		return nil
	}

	return scoredList[0].lobby
}

func countIntersect(a, b []int32) int {
	m := make(map[int32]bool, len(a))

	for _, v := range a {
		m[v] = true
	}

	count := 0

	for _, v := range b {
		if m[v] {
			count++
		}
	}

	return count
}
