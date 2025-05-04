package modules

import (
	"testing"

	"github.com/google/uuid"

	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
)

const (
	classicMode = "classic"
	battleMode  = "battle"
)

const (
	player1 string = "player_1"
	player2 string = "player_2"
	player3 string = "player_3"
	player4 string = "player_4"
	player5 string = "player_5"
	player6 string = "player_6"
	player7 string = "player_7"
	player8 string = "player_8"
)

func prepare(t *testing.T, cfg *config.TestConfig) {
	players = map[string]player{
		player1: {
			id:         uuid.New().String(),
			rating:     100,
			categories: []int32{1, 10, 100, 1000},
		},
		player2: {
			id:         uuid.New().String(),
			rating:     100,
			categories: []int32{1, 10, 100, 1000},
		},
		player3: {
			id:         uuid.New().String(),
			rating:     100,
			categories: []int32{1, 10, 100, 1000},
		},
		player4: {
			id:         uuid.New().String(),
			rating:     100,
			categories: []int32{1, 10, 100, 1000},
		},
		player5: {
			id:         uuid.New().String(),
			rating:     100,
			categories: []int32{1, 10, 100, 1000},
		},
		player6: {
			id:         uuid.New().String(),
			rating:     100,
			categories: []int32{1, 10, 100, 1000},
		},
		player7: {
			id:         uuid.New().String(),
			rating:     100,
			categories: []int32{1, 10, 100, 1000},
		},
		player8: {
			id:         uuid.New().String(),
			rating:     100,
			categories: []int32{1, 10, 100, 1000},
		},
	}
}

type player struct {
	id         string
	rating     int32
	categories []int32
}

var players map[string]player
