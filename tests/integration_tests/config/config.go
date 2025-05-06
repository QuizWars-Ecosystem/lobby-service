package config

import (
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/handler"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/matchmaking"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/lobby"

	"github.com/QuizWars-Ecosystem/go-common/pkg/log"

	def "github.com/QuizWars-Ecosystem/go-common/pkg/config"
	test "github.com/QuizWars-Ecosystem/go-common/pkg/testing/config"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/config"
)

type TestConfig struct {
	ServiceConfig *config.Config
	Redis         *test.RedisClusterConfig
	NATS          *test.NatsConfig
	ServerAmount  int
	Generator     *Generator
}

func NewTestConfig() *TestConfig {
	redisClusterCfg := test.DefaultRedisClusterConfig()
	natsCfg := test.DefaultNatsConfig()

	redisClusterCfg.Masters = 3
	redisClusterCfg.Replicas = 1

	return &TestConfig{
		ServerAmount: 3,
		Generator: &Generator{
			PlayersCount:  10_000,
			MaxRating:     10_000,
			CategoriesMax: 10,
			CategoryMaxID: 50,
			Modes: []string{
				"classic",
				"battle",
				"1v1",
				"mega",
			},
		},
		ServiceConfig: &config.Config{
			ServiceConfig: &def.ServiceConfig{
				Name:         "lobby-service",
				Address:      "lobby_address",
				Local:        true,
				GRPCPort:     50051,
				StartTimeout: time.Second * 30,
				StopTimeout:  time.Second * 30,
				ConsulURL:    "consul:8500",
			},
			Logger: &log.Config{
				Level: "debug",
			},
			Redis: &config.RedisConfig{},
			NATS:  &config.NATSConfig{},
			Handler: &handler.Config{
				ModeStats: map[string]handler.StatPair{
					"classic": {
						Min: 4,
						Max: 8,
					},
					"battle": {
						Min: 2,
						Max: 4,
					},
					"1v1": {
						Min: 2,
						Max: 2,
					},
					"mega": {
						Min: 12,
						Max: 64,
					},
				},
				LobbyTLL:         time.Minute * 5,
				MaxLobbyAttempts: 3,
			},
			Lobby: &lobby.Config{
				TickerTimeout:    time.Second,
				MaxLobbyWait:     time.Minute,
				LobbyIdleExtend:  time.Second * 15,
				MinReadyDuration: time.Second * 10,
			},
			Matcher: &matchmaking.Config{
				CategoryWeight:    0.7,
				RatingWeight:      0.2,
				PlayersFillWeight: 0.1,
				MaxExpectedRating: 500,
			},
		},
		Redis: &redisClusterCfg,
		NATS:  &natsCfg,
	}
}

type Generator struct {
	PlayersCount  int
	MaxRating     int32
	CategoriesMax int
	CategoryMaxID int32
	Modes         []string
}
