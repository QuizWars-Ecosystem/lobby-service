package modules

import (
	"github.com/brianvoe/gofakeit/v7"
	"math/rand/v2"
	"testing"

	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
)

var (
	modes = []string{
		"classic",
		"battle",
		"1v1",
		"mega",
	}
)

func prepare(_ *testing.T, _ *config.TestConfig) []player {
	_ = gofakeit.Seed(rand.Int())

	playersCount := 1000
	players := make([]player, playersCount)

	for i := 0; i < playersCount; i++ {
		categoriesAmount := rand.IntN(10)
		if categoriesAmount <= 2 {
			categoriesAmount += 4
		}

		var categories = make([]int32, categoriesAmount)
		for j := 0; j < categoriesAmount; j++ {
			categories[j] = rand.Int32N(25)
		}

		players[i] = player{
			id:         gofakeit.UUID(),
			categories: categories,
			rating:     rand.Int32N(5000),
			mode:       modes[gofakeit.IntN(len(modes))],
		}
	}

	return players
}

type player struct {
	id         string
	rating     int32
	categories []int32
	mode       string
}
