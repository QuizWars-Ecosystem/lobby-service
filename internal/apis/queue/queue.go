package queue

import (
	"github.com/QuizWars-Ecosystem/lobby-service/internal/models"
	"sync"
	"time"
)

type event struct {
	Mode    string
	Players []*models.Player
	Action  string // "add" | "flush"
}

type PlayerBatch struct {
	players   []*models.Player
	createdAt time.Time
	mode      string
}

type Manager struct {
	inputChan  chan event
	outputChan chan []*models.Player
	configs    map[string]Config
	batches    map[string]*PlayerBatch
	closeChan  chan struct{}
	sync.RWMutex
}

func NewQueueManager(cfg *ConfigSet) *Manager {
	qm := &Manager{
		inputChan:  make(chan event, 1000),
		outputChan: make(chan []*models.Player, 100),
		configs:    cfg.set,
		batches:    make(map[string]*PlayerBatch),
		closeChan:  make(chan struct{}),
	}
	go qm.eventLoop()
	return qm
}

func (m *Manager) AddPlayer(mode string, p *models.Player) {
	m.inputChan <- event{
		Mode:    mode,
		Players: []*models.Player{p},
		Action:  "add",
	}
}

func (m *Manager) eventLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case e := <-m.inputChan:
			switch e.Action {
			case "add":
				m.handleAddPlayers(e.Mode, e.Players)
			case "flush":
				m.flushBatch(e.Mode)
			}
		case <-ticker.C:
			m.checkTimeouts()
		case <-m.closeChan:
			return
		}
	}
}

func (m *Manager) handleAddPlayers(mode string, players []*models.Player) {
	batch, exists := m.batches[mode]
	if !exists {
		batch = &PlayerBatch{
			mode:      mode,
			createdAt: time.Now(),
		}
		m.batches[mode] = batch
	}

	batch.players = append(batch.players, players...)
	cfg := m.getConfig(mode)

	if len(batch.players) >= cfg.BatchSize {
		m.flushBatch(mode)
	}
}

func (m *Manager) flushBatch(mode string) {
	if batch, exists := m.batches[mode]; exists && len(batch.players) > 0 {
		players := make([]*models.Player, len(batch.players))
		copy(players, batch.players)

		m.outputChan <- players
		delete(m.batches, mode)
	}
}

func (m *Manager) checkTimeouts() {
	now := time.Now()
	for mode, batch := range m.batches {
		cfg := m.getConfig(mode)

		if now.Sub(batch.createdAt) > cfg.MaxWaitTime ||
			(len(batch.players) >= cfg.ForceThreshold && len(batch.players) > 0) {
			m.flushBatch(mode)
		}
	}
}

func (m *Manager) getConfig(mode string) Config {
	m.RLock()
	defer m.RUnlock()

	cfg, ok := m.configs[mode]
	if !ok {
		cfg = Config{
			BatchSize:   16,
			MaxWaitTime: time.Second * 2,
		}

		m.configs[mode] = cfg
	}

	return cfg
}
