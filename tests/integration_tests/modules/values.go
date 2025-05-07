package modules

import (
	"github.com/google/uuid"
	"math/rand/v2"
	"testing"

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

	var categories = make([]int32, categoriesAmount)
	for j := 0; j < categoriesAmount; j++ {
		categories[j] = rand.Int32N(cfg.CategoryMaxID + 1)
	}

	p := player{
		id:         uuid.NewString(),
		categories: categories,
		rating:     rand.Int32N(cfg.MaxRating + 1),
		mode:       cfg.Modes[rand.IntN(len(cfg.Modes))],
	}

	return p
}
