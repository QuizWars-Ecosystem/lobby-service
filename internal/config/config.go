package config

import (
	"github.com/QuizWars-Ecosystem/go-common/pkg/config"
	"github.com/QuizWars-Ecosystem/go-common/pkg/log"
)

type Config struct {
	*config.ServiceConfig `mapstructure:"service"`
	Logger                *log.Config  `mapstructure:"logger"`
	Redis                 *RedisConfig `mapstructure:"redis"`
}

type RedisConfig struct {
	URL string `mapstructure:"url"`
}
