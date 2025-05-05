package matchmaking

import (
	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"math"
	"sync"
)

type Config struct {
	CategoryWeight    float64 `mapstructure:"category_weight" default:"0.5"`
	PlayersFillWeight float64 `mapstructure:"players_fill_weight" default:"0.3"`
	RatingWeight      float64 `mapstructure:"rating_weight" default:"0.2"`
	MaxExpectedRating int     `mapstructure:"max_expected_rating" default:"1000"`
}

var _ abstractions.ConfigSubscriber[*Config] = (*Matcher)(nil)

type Matcher struct {
	mx  sync.Mutex
	cfg *Config
}

func NewMatcher(cfg *Config) *Matcher {
	return &Matcher{
		cfg: cfg,
	}
}

func (m *Matcher) FilterLobbies(lobbies []*models.Lobby, player *models.Player) []*models.Lobby {
	var matches []*models.Lobby

	m.mx.Lock()
	defer m.mx.Unlock()

	for _, l := range lobbies {
		if l.MaxPlayers == 0 {
			continue
		}

		ratingDiff := math.Abs(float64(player.Rating - l.AvgRating))

		// Border (ex 3x from MaxExpectedRating)
		if ratingDiff > float64(m.cfg.MaxExpectedRating)*3 {
			continue
		}

		// Friction of interceptions categories (by player's categories)
		categoryMatch := countIntersect(player.Categories, l.Categories)
		if len(player.Categories) > 0 {
			matchRatio := float64(categoryMatch) / float64(len(player.Categories))
			if matchRatio < 0.1 { // <10% - trash
				continue
			}
		}

		matches = append(matches, l)
	}

	return matches
}

func (m *Matcher) SelectBestLobby(lobbies []*models.Lobby, player *models.Player) *models.Lobby {
	type scoredLobby struct {
		lobby *models.Lobby
		score float64
	}

	var best *scoredLobby

	m.mx.Lock()
	defer m.mx.Unlock()

	for _, l := range lobbies {
		if l.MaxPlayers == 0 {
			continue
		}

		// Categories: Jaccard similarity
		categoryScore := jaccardIndex(player.Categories, l.Categories)

		// Fullness: more players - better
		fillScore := float64(len(l.Players)) / float64(l.MaxPlayers)

		// Difference in rating
		ratingDiff := math.Abs(float64(player.Rating - l.AvgRating))
		ratingScore := 1.0 - ratingDiff/float64(m.cfg.MaxExpectedRating)
		if ratingScore < 0 {
			ratingScore = 0
		}

		// Total score
		totalScore := m.cfg.CategoryWeight*categoryScore +
			m.cfg.PlayersFillWeight*fillScore +
			m.cfg.RatingWeight*ratingScore

		if best == nil || totalScore > best.score {
			best = &scoredLobby{
				lobby: l,
				score: totalScore,
			}
		}
	}

	if best == nil {
		return nil
	}

	return best.lobby
}

func (m *Matcher) SectionKey() string {
	return "MATCHER"
}

func (m *Matcher) UpdateConfig(newCfg *Config) error {
	m.mx.Lock()
	defer m.mx.Unlock()

	m.cfg = newCfg
	return nil
}

func jaccardIndex(a, b []int32) float64 {
	setA := make(map[int32]struct{}, len(a))
	setB := make(map[int32]struct{}, len(b))

	for _, v := range a {
		setA[v] = struct{}{}
	}
	for _, v := range b {
		setB[v] = struct{}{}
	}

	inter := 0
	for v := range setA {
		if _, ok := setB[v]; ok {
			inter++
		}
	}

	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func countIntersect(a, b []int32) int {
	set := make(map[int32]struct{}, len(a))
	for _, v := range a {
		set[v] = struct{}{}
	}

	count := 0
	for _, v := range b {
		if _, ok := set[v]; ok {
			count++
		}
	}

	return count
}
