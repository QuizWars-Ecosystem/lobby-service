package queue

import (
	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	"time"
)

var _ abstractions.ConfigSubscriber[*ConfigSet] = (*Manager)(nil)

type Config struct {
	BatchSize      int           `mapstructure:"batch_size" yaml:"batch_size"`
	MaxWaitTime    time.Duration `mapstructure:"max_wait_time" yaml:"max_wait_time"`
	ForceThreshold int           `mapstructure:"force_threshold" yaml:"force_threshold"`
}

type ConfigSet struct {
	set map[string]Config `mapstructure:"queue_set" yaml:"queue_set"`
}

func (m *Manager) SectionKey() string {
	return "QUEUE_MANAGER"
}

func (m *Manager) UpdateConfig(newCfg *ConfigSet) error {
	m.Lock()
	defer m.Unlock()

	for mode, cfg := range newCfg.set {
		m.configs[mode] = cfg
	}

	return nil
}
