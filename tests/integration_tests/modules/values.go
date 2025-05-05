package modules

import (
	"github.com/brianvoe/gofakeit/v7"
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
	var categoriesSetAmount = 4
	var categories [][]int32

	for i := 0; i < categoriesSetAmount; i++ {
		var cats []int32
		amount := 3

		for j := 0; j < amount; j++ {
			cats = append(cats, int32(gofakeit.IntN(10)))
		}

		categories = append(categories, cats)
	}

	playersCount := 1000
	players := make([]player, playersCount)

	for i := 0; i < playersCount; i++ {
		p := player{
			id:         gofakeit.UUID(),
			rating:     int32(gofakeit.IntN(2000)),
			categories: categories[gofakeit.IntN(categoriesSetAmount)],
			mode:       modes[gofakeit.IntN(len(modes))],
		}

		players[i] = p
	}

	return players
}

type player struct {
	id         string
	rating     int32
	categories []int32
	mode       string
}
