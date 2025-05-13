package config

import (
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models/matcher"
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/handler"

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
	redisClusterCfg.Replicas = 2

	return &TestConfig{
		ServerAmount: 3,
		Generator: &Generator{
			PlayersCount:  1_000,
			MaxRating:     5_000,
			CategoriesMax: 10,
			CategoryMaxID: 10,
			Modes: []string{
				"classic",
				"battle",
				"duel",
				"blitz",
				"team",
				"mega",
			},
		},
		ServiceConfig: &config.Config{
			ServiceConfig: &def.ServiceConfig{
				Name:         "lobby-service",
				Address:      "lobby_address",
				Local:        true,
				GRPCPort:     50051,
				StartTimeout: time.Minute,
				StopTimeout:  time.Minute,
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
						Max: 10,
					},
					"battle": {
						Min: 2,
						Max: 4,
					},
					"blitz": {
						Min: 3,
						Max: 6,
					},
					"team": {
						Min: 4,
						Max: 4,
					},
					"duel": {
						Min: 2,
						Max: 2,
					},
					"mega": {
						Min: 24,
						Max: 128,
					},
				},
				LobbyTLL:         time.Minute * 5,
				MaxLobbyAttempts: 5,
			},
			Lobby: &lobby.Config{
				TickerTimeout:    time.Second,
				MaxLobbyWait:     time.Minute,
				LobbyIdleExtend:  time.Second * 15,
				MinReadyDuration: time.Second * 10,
			},
			Matcher: &matcher.Config{
				Configs: map[string]matcher.ScoringConfig{
					"default": {
						RatingWeight:     0.3,
						CategoryWeight:   0.5,
						FillWeight:       0.2,
						MaxRatingDiff:    1000,
						MinCategoryMatch: 0.3,
					},
					"duel": {
						RatingWeight:     0.9,
						CategoryWeight:   0.1,
						FillWeight:       0.0,
						MaxRatingDiff:    500,
						MinCategoryMatch: 0.2,
					},
					"battle": {
						RatingWeight:     0.7,
						CategoryWeight:   0.3,
						FillWeight:       0.0,
						MaxRatingDiff:    800,
						MinCategoryMatch: 0.1,
					},
					"classic": {
						RatingWeight:     0.3,
						CategoryWeight:   0.5,
						FillWeight:       0.2,
						MaxRatingDiff:    1000,
						MinCategoryMatch: 0.4,
					},
					"blitz": {
						RatingWeight:     0.3,
						CategoryWeight:   0.4,
						FillWeight:       0.3,
						MaxRatingDiff:    800,
						MinCategoryMatch: 0.4,
					},
					"team": {
						RatingWeight:     0.5,
						CategoryWeight:   0.4,
						FillWeight:       0.1,
						MaxRatingDiff:    1000,
						MinCategoryMatch: 0.4,
					},
					"mega": {
						RatingWeight:     0.0,
						CategoryWeight:   0.0,
						FillWeight:       1.0,
						MaxRatingDiff:    10_000,
						MinCategoryMatch: 0.0,
					},
				},
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
