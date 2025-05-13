package report

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"html/template"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
)

type TemplateData struct {
	Timestamp        time.Time
	Config           *config.TestConfig
	Result           *Result
	PlayersInLobbies int32
	LobbiesCount     int
	RowsByMode       map[string][]LobbyStatView
	Modes            []string
	RedisNodes       int
}

type LobbyStatView struct {
	ID                 string
	Mode               string
	PlayersCount       int
	MaxPlayers         int
	AvgRating          float64
	MinRating          int32
	MaxRating          int32
	CommonCats         []int
	UniqueCats         []int
	WaitDuration       string
	Status             string
	StatusClass        string
	RatingSet          map[string]int32
	CategoriesSet      map[string][]int32
	RatingDiffValid    bool
	RatingDiffValue    float64
	CategoryMatchValid bool
	CategoryMatchValue float64
	OverallValid       bool
	OverallValue       float64
}

func (r *Result) GenerateHTMLReport() error {
	data := TemplateData{
		Timestamp: time.Now(),
		Config:    r.Cfg,
		Result:    r,
	}

	for _, lobby := range r.Lobbies {
		data.PlayersInLobbies += lobby.Players
		data.LobbiesCount++
	}

	data.RowsByMode = make(map[string][]LobbyStatView)

	data.RedisNodes = r.Cfg.Redis.Masters + r.Cfg.Redis.Replicas*r.Cfg.Redis.Masters

	for id, lobby := range r.Lobbies {
		view := r.createLobbyStatView(id, lobby)
		data.RowsByMode[lobby.Mode] = append(data.RowsByMode[lobby.Mode], view)
	}

	for mode := range data.RowsByMode {
		data.Modes = append(data.Modes, mode)
	}

	sort.Strings(data.Modes)

	filename := fmt.Sprintf("../reports/lobby_stats_%s.html", data.Timestamp.Format("2006-01-02-1504"))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	tmpl := template.New("base.gohtml").Funcs(template.FuncMap{
		"heatmapColor":          getHeatmapColor,
		"formatPercentage":      formatPercentage,
		"formatCategoryList":    formatCategoryList,
		"calculateAvgRating":    calculateAvgRating,
		"calculateWaitDuration": calculateWaitDuration,
		"getRatingClass":        getRatingClass,
		"json":                  toJSON,
		"div":                   func(a, b float64) float64 { return a / b * 100 },
		"mul":                   func(a, b float64) float64 { return a * b },
		"sub":                   func(a, b float64) float64 { return a - b },
		"add":                   func(a, b float64) float64 { return a + b },
		"toFloat64":             func(x int) float64 { return float64(x) },
		"toFloat64Int32":        func(x int32) float64 { return float64(x) },
		"randomColor": func(seed string) string {
			colors := []string{
				"#FF6384", "#36A2EB", "#FFCE56", "#4BC0C0",
				"#9966FF", "#FF9F40", "#8AC24A", "#F06292",
				"#7986CB", "#E57373", "#64B5F6", "#FFB74D",
			}
			index := crc32.ChecksumIEEE([]byte(seed)) % uint32(len(colors))
			return colors[index]
		},
		"durationSeconds": func(start, end time.Time) float64 {
			return end.Sub(start).Seconds()
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
	})

	tmpl, err = tmpl.ParseGlob("templates/*.gohtml")
	if err != nil {
		return fmt.Errorf("failed to parse templates: %v", err)
	}

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	return nil
}

func (r *Result) createLobbyStatView(id string, lobby *LobbyStat) LobbyStatView {
	cfg := r.Cfg.ServiceConfig.Matcher.Configs[lobby.Mode]
	if cfg.MinCategoryMatch == 0 {
		cfg = r.Cfg.ServiceConfig.Matcher.Configs["default"]
	}

	threshold := cfg.MinCategoryMatch

	var sum int32
	minRating := int32(math.MaxInt32)
	maxRating := int32(math.MinInt32)

	for _, rating := range lobby.RatingSet {
		sum += rating
		if rating < minRating {
			minRating = rating
		}
		if rating > maxRating {
			maxRating = rating
		}
	}

	avgRating := 0.0
	if len(lobby.RatingSet) > 0 {
		avgRating = float64(sum) / float64(len(lobby.RatingSet))
	}

	numPlayers := len(lobby.categoriesSet)
	categoryCount := make(map[int32]int)

	for _, cats := range lobby.categoriesSet {
		seen := make(map[int32]struct{})
		for _, c := range cats {
			if _, ok := seen[c]; !ok {
				categoryCount[c]++
				seen[c] = struct{}{}
			}
		}
	}

	var commonCats, uniqueCats []int
	for cat, count := range categoryCount {
		if float64(count)/float64(numPlayers) >= threshold {
			commonCats = append(commonCats, int(cat))
		} else {
			uniqueCats = append(uniqueCats, int(cat))
		}
	}

	sort.Ints(commonCats)
	sort.Ints(uniqueCats)

	status, statusClass := getLobbyStatus(lobby)

	ratingValid, categoryValid, ratingDiff, categoryMatch, overallScore := r.checkMatchConditions(lobby, lobby.Mode)

	view := LobbyStatView{
		ID:            id,
		Mode:          lobby.Mode,
		PlayersCount:  int(lobby.Players),
		MaxPlayers:    int(lobby.MaxPlayers),
		AvgRating:     avgRating,
		MinRating:     minRating,
		MaxRating:     maxRating,
		CommonCats:    commonCats,
		UniqueCats:    uniqueCats,
		WaitDuration:  calculateWaitDuration(lobby.CreatedAt, lobby.StartedAt),
		Status:        status,
		StatusClass:   statusClass,
		RatingSet:     lobby.RatingSet,
		CategoriesSet: lobby.categoriesSet,
	}

	view.RatingDiffValid = ratingValid
	view.CategoryMatchValid = categoryValid
	view.RatingDiffValue = ratingDiff
	view.CategoryMatchValue = categoryMatch
	view.OverallValid = ratingValid && categoryValid
	view.OverallValue = overallScore

	return view
}

func getLobbyStatus(lobby *LobbyStat) (string, string) {
	switch lobby.Status {
	case startedStatus:
		return "STARTED", "badge-success"
	case expiredStatus:
		return "EXPIRED", "badge-warning"
	case erroredStatus:
		return "ERROR", "badge-danger"
	case waitedStatus:
		return "WAIT", "badge-info"
	default:
		return "-", ""
	}
}

func getHeatmapColor(value, min, max float64) string {
	normalized := (value - min) / (max - min)
	if normalized < 0 {
		normalized = 0
	} else if normalized > 1 {
		normalized = 1
	}

	r := int(255 * normalized)
	g := int(255 * (1 - normalized))
	b := 128

	return fmt.Sprintf("rgb(%d,%d,%d)", r, g, b)
}

func formatPercentage(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func toJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func formatCategoryList(cats []int) string {
	if len(cats) == 0 {
		return "-"
	}

	var builder strings.Builder
	for i, cat := range cats {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprint(cat))
		if i >= 4 && len(cats) > 5 {
			builder.WriteString(fmt.Sprintf(" (+%d more)", len(cats)-i-1))
			break
		}
	}
	return builder.String()
}

func calculateAvgRating(ratingSet map[string]int32) float64 {
	var sum int32
	for _, rating := range ratingSet {
		sum += rating
	}
	if len(ratingSet) == 0 {
		return 0
	}
	return float64(sum) / float64(len(ratingSet))
}

func calculateWaitDuration(createdAt, startedAt time.Time) string {
	if createdAt.IsZero() || startedAt.IsZero() {
		return "-"
	}
	dur := startedAt.Sub(createdAt)
	if dur < time.Second {
		return "<1s"
	}
	return dur.Truncate(time.Second).String()
}

func getRatingClass(rating int32) string {
	switch {
	case rating > 8000:
		return "rating-high"
	case rating > 5000:
		return "rating-medium-high"
	case rating > 3000:
		return "rating-medium"
	case rating > 1500:
		return "rating-medium-low"
	default:
		return "rating-low"
	}
}

func (r *Result) checkMatchConditions(lobby *LobbyStat, mode string) (bool, bool, float64, float64, float64) {
	cfg := r.Cfg.ServiceConfig.Matcher.Configs[mode]
	if cfg.MaxRatingDiff == 0 {
		cfg = r.Cfg.ServiceConfig.Matcher.Configs["default"]
	}

	ratingDiff := float64(getRatingSpread(lobby.RatingSet))
	ratingValid := ratingDiff <= cfg.MaxRatingDiff

	categoryMatch := calculateCategoryMatch(lobby.categoriesSet)
	categoryValid := categoryMatch >= cfg.MinCategoryMatch

	ratingPercent := 0.0
	if cfg.MaxRatingDiff > 0 {
		ratingPercent = 1.0 - math.Min(1.0, ratingDiff/cfg.MaxRatingDiff)
	}

	categoryPercent := math.Min(1.0, categoryMatch/cfg.MinCategoryMatch)

	overallScore := ratingPercent*cfg.RatingWeight + categoryPercent*cfg.CategoryWeight

	if ratingValid && categoryValid {
		overallScore = 1.0
	} else {
		totalWeight := cfg.RatingWeight + cfg.CategoryWeight
		overallScore = (ratingPercent*cfg.RatingWeight + categoryPercent*cfg.CategoryWeight) / totalWeight
	}

	return ratingValid, categoryValid, ratingDiff, categoryMatch, overallScore
}

func getRatingSpread(ratings map[string]int32) int32 {
	if len(ratings) == 0 {
		return 0
	}

	minR, maxR := int32(math.MaxInt32), int32(math.MinInt32)
	for _, r := range ratings {
		if r < minR {
			minR = r
		}
		if r > maxR {
			maxR = r
		}
	}

	return maxR - minR
}

func calculateCategoryMatch(categories map[string][]int32) float64 {
	if len(categories) < 2 {
		return 1.0
	}

	var totalMatch float64
	var pairs int

	players := make([][]int32, 0, len(categories))
	for _, cats := range categories {
		players = append(players, cats)
	}

	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			intersection := intersect(players[i], players[j])
			union := union(players[i], players[j])
			if len(union) == 0 {
				continue
			}
			totalMatch += float64(len(intersection)) / float64(len(union))
			pairs++
		}
	}

	if pairs == 0 {
		return 0
	}
	return totalMatch / float64(pairs)
}

func intersect(a, b []int32) []int32 {
	set := make(map[int32]struct{})
	var result []int32

	for _, item := range a {
		set[item] = struct{}{}
	}

	for _, item := range b {
		if _, exists := set[item]; exists {
			result = append(result, item)
			delete(set, item)
		}
	}

	return result
}

func union(a, b []int32) []int32 {
	set := make(map[int32]struct{})
	var result []int32

	for _, item := range a {
		if _, exists := set[item]; !exists {
			set[item] = struct{}{}
			result = append(result, item)
		}
	}
	for _, item := range b {
		if _, exists := set[item]; !exists {
			set[item] = struct{}{}
			result = append(result, item)
		}
	}

	return result
}
