package matchmaking

type Config struct {
	CategoryWeight    float64 `mapstructure:"category_weight" default:"0.5"`
	PlayersFillWeight float64 `mapstructure:"players_fill_weight" default:"0.3"`
	RatingWeight      float64 `mapstructure:"rating_weight" default:"0.2"`
	MaxExpectedRating int     `mapstructure:"max_expected_rating" default:"1000"`
}

func (m *Matcher) SectionKey() string {
	return "MATCHER"
}

func (m *Matcher) UpdateConfig(newCfg *Config) error {
	m.mx.Lock()
	defer m.mx.Unlock()

	m.cfg = newCfg
	return nil
}

func (m *Matcher) getCategoryWeight() float64 {
	m.mx.RLock()
	defer m.mx.RUnlock()
	return m.cfg.CategoryWeight
}

func (m *Matcher) getPlayersFillWeight() float64 {
	m.mx.RLock()
	defer m.mx.RUnlock()
	return m.cfg.PlayersFillWeight
}

func (m *Matcher) getRatingWeight() float64 {
	m.mx.RLock()
	defer m.mx.RUnlock()
	return m.cfg.RatingWeight
}

func (m *Matcher) getMaxExpectedRating() int {
	m.mx.RLock()
	defer m.mx.RUnlock()
	return m.cfg.MaxExpectedRating
}
