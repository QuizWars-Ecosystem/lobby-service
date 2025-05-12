package matcher

type ScoringConfig struct {
	RatingWeight     float64 `mapstructure:"rating_weight" yaml:"rating_weight"`                       // from 0 to 1
	CategoryWeight   float64 `mapstructure:"category_weight" yaml:"category_weight"`                   // from 0 to 1
	FillWeight       float64 `mapstructure:"fill_weight" yaml:"fill_weight"`                           // from 0 to 1
	MaxRatingDiff    float64 `mapstructure:"max_rating_diff" yaml:"max_rating_diff"`                   // e.g 1000
	MinCategoryMatch float64 `mapstructure:"min_category_match_ratio" yaml:"min_category_match_ratio"` // from 0 to 1
}

type Config struct {
	Configs map[string]ScoringConfig `mapstructure:"configs" yaml:"configs"`
}

func (c *Config) GetConfig(mode string) ScoringConfig {
	cfg, ok := c.Configs[mode]
	if !ok {
		cfg = ScoringConfig{
			CategoryWeight:   0.5,
			RatingWeight:     0.3,
			FillWeight:       0.2,
			MaxRatingDiff:    1000.0,
			MinCategoryMatch: 0.3,
		}
		c.Configs[mode] = cfg
	}

	return cfg
}
