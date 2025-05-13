package modules

import (
	"math/rand/v2"
	"testing"

	"github.com/google/uuid"

	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
)

type player struct {
	id         string
	rating     int32
	categories []int32
	mode       string
}

func generator(_ *testing.T, cfg *config.TestConfig) <-chan player {
	var out chan player

	if cfg.Generator.PlayersCount > 10_000 {
		out = make(chan player, cfg.Generator.PlayersCount/100)
	} else {
		out = make(chan player, 500)
	}

	go func() {
		for i := 0; i < cfg.Generator.PlayersCount; i++ {
			out <- generatePlayer(cfg.Generator)
		}

		close(out)
	}()

	return out
}

func generatePlayer(cfg *config.Generator) player {
	categoriesAmount := rand.IntN(cfg.CategoriesMax + 1)
	if categoriesAmount <= 2 {
		categoriesAmount += 4
	}

	uniqueCategories := make(map[int32]struct{})
	for len(uniqueCategories) < categoriesAmount {
		catID := rand.Int32N(cfg.CategoryMaxID + 1)
		uniqueCategories[catID] = struct{}{}
	}

	categories := make([]int32, 0, len(uniqueCategories))
	for cat := range uniqueCategories {
		categories = append(categories, cat)
	}

	p := player{
		id:         uuid.NewString(),
		categories: categories,
		rating:     rand.Int32N(cfg.MaxRating + 1),
		mode:       cfg.Modes[rand.IntN(len(cfg.Modes))],
	}

	return p
}
