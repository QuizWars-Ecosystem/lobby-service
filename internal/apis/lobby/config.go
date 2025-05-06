package lobby

import "time"

type Config struct {
	TickerTimeout    time.Duration `mapstructure:"tickerTimeout" default:"1s"`
	MaxLobbyWait     time.Duration `mapstructure:"maxLobbyWait" default:"1m"`
	LobbyIdleExtend  time.Duration `mapstructure:"lobbyIdleExtend" default:"15s"`
	MinReadyDuration time.Duration `mapstructure:"minReadyDuration" default:"10s"`
}

func (w *Waiter) SectionKey() string {
	return "LOBBY"
}

func (w *Waiter) UpdateConfig(newCfg *Config) error {
	w.mx.Lock()
	defer w.mx.Unlock()

	w.cfg = newCfg

	return nil
}

func (w *Waiter) getTickerTimeout() time.Duration {
	w.mx.RLock()
	defer w.mx.RUnlock()
	if w.cfg.TickerTimeout < time.Millisecond*500 {
		return time.Millisecond * 500
	}

	return w.cfg.TickerTimeout
}

func (w *Waiter) getMaxLobbyWait() time.Duration {
	w.mx.RLock()
	defer w.mx.RUnlock()
	if w.cfg.MaxLobbyWait < time.Second*30 {
		return time.Second * 30
	}
	return w.cfg.MaxLobbyWait
}

func (w *Waiter) getLobbyIdleExtend() time.Duration {
	w.mx.RLock()
	defer w.mx.RUnlock()
	if w.cfg.LobbyIdleExtend < time.Second*10 {
		return time.Second * 10
	}
	return w.cfg.LobbyIdleExtend
}

func (w *Waiter) getMinReadyDuration() time.Duration {
	w.mx.RLock()
	defer w.mx.RUnlock()
	if w.cfg.MinReadyDuration < time.Second*10 {
		return time.Second * 10
	}
	return w.cfg.MinReadyDuration
}
