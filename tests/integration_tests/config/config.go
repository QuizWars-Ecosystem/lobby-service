package config

import (
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/handler"
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/lobby"

	"github.com/QuizWars-Ecosystem/go-common/pkg/log"

	def "github.com/QuizWars-Ecosystem/go-common/pkg/config"
	test "github.com/QuizWars-Ecosystem/go-common/pkg/testing/config"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/config"
)

type TestConfig struct {
	ServiceConfig *config.Config
	Redis         *test.RedisConfig
}

func NewTestConfig() *TestConfig {
	redisCfg := test.DefaultRedisConfig()

	return &TestConfig{
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
				},
			},
			Lobby: &lobby.Config{
				TickerTimeout:    time.Second,
				MaxLobbyWait:     time.Minute * 5,
				LobbyIdleExtend:  time.Second * 15,
				MinReadyDuration: time.Second * 10,
			},
		},
		Redis: &redisCfg,
	}
}
