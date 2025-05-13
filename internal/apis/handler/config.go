package handler

import "time"

type Config struct {
	ModeStats        map[string]StatPair `mapstructure:"mode_stats" yaml:"mode_stats"`
	LobbyTLL         time.Duration       `mapstructure:"lobby_tll" yaml:"lobby_tll" default:"4m"`
	MaxLobbyAttempts int                 `mapstructure:"max_lobby_attempts" yaml:"max_lobby_attempts" default:"3"`
	TopLobbiesLimit  int                 `mapstructure:"top_lobbies_limit" yaml:"top_lobbies_limit" default:"25"`
}

func (h *Handler) SectionKey() string {
	return "HANDLER"
}

func (h *Handler) UpdateConfig(newCfg *Config) error {
	h.mx.Lock()
	defer h.mx.Unlock()

	h.cfg = newCfg
	return nil
}

func (h *Handler) getModeStats(mode string) StatPair {
	h.mx.RLock()
	pair, ok := h.cfg.ModeStats[mode]
	h.mx.RUnlock()
	if !ok {
		pair = StatPair{
			Min: 4,
			Max: 8,
		}
	}

	return pair
}

func (h *Handler) getLobbyTLL() time.Duration {
	h.mx.RLock()
	defer h.mx.RUnlock()
	if h.cfg.LobbyTLL < time.Minute {
		h.cfg.LobbyTLL = time.Minute
	}
	return h.cfg.LobbyTLL
}

func (h *Handler) getMaxLobbyAttempts() int {
	h.mx.RLock()
	defer h.mx.RUnlock()
	if h.cfg.MaxLobbyAttempts < 2 {
		h.cfg.MaxLobbyAttempts = 2
	}
	return h.cfg.MaxLobbyAttempts
}

func (h *Handler) getTopLobbiesLimit() int {
	h.mx.RLock()
	defer h.mx.RUnlock()
	if h.cfg.TopLobbiesLimit < 10 {
		h.cfg.TopLobbiesLimit = 10
	}
	return h.cfg.TopLobbiesLimit
}
