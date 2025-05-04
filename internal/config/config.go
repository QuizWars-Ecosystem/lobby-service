package config

import (
	"github.com/QuizWars-Ecosystem/go-common/pkg/config"
	"github.com/QuizWars-Ecosystem/go-common/pkg/log"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/handler"
	"github.com/QuizWars-Ecosystem/lobby-service/internal/apis/lobby"
)

type Config struct {
	*config.ServiceConfig `mapstructure:"service"`
	Logger                *log.Config     `mapstructure:"logger"`
	Redis                 *RedisConfig    `mapstructure:"redis"`
	Lobby                 *lobby.Config   `mapstructure:"lobby"`
	Handler               *handler.Config `mapstructure:"handler"`
}

type RedisConfig struct {
	URL string `mapstructure:"url"`
}
