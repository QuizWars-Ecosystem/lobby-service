package report

import (
	"fmt"
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"hash/crc32"
	"html/template"
	"math"
	"os"
	"sort"
	"strings"
	"time"
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
	ID            string
	Mode          string
	PlayersCount  int
	MaxPlayers    int
	AvgRating     float64
	MinRating     int32
	MaxRating     int32
	CommonCats    []int
	UniqueCats    []int
	WaitDuration  string
	Status        string
	StatusClass   string
	RatingSet     map[string]int32
	CategoriesSet map[string][]int32
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
	threshold := 0.6

	data.RedisNodes = r.Cfg.Redis.Masters + r.Cfg.Redis.Replicas*r.Cfg.Redis.Masters

	for id, lobby := range r.Lobbies {
		view := r.createLobbyStatView(id, lobby, threshold)
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
		"div":                   func(a, b float64) float64 { return a / b * 100 },
		"mul":                   func(a, b float64) float64 { return a * b },
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

func (r *Result) createLobbyStatView(id string, lobby *LobbyStat, threshold float64) LobbyStatView {
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

	return LobbyStatView{
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
