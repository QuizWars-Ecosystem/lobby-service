package report

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

func formatCatList(list []int) string {
	if len(list) == 0 {
		return "-"
	}
	var strList []string
	for _, c := range list {
		strList = append(strList, fmt.Sprint(c))
	}
	return strings.Join(strList, ", ")
}

func (r *Result) LogStats() {
	r.Lock()
	defer r.Unlock()

	type statRow struct {
		id           string
		mode         string
		count        int
		max          int
		avgRating    float64
		minRating    int32
		maxRating    int32
		commonCats   []int
		uniqueCats   []int
		waitDuration string
		status       string
	}

	if countAsyncMap(&r.Lobbies) > 250 {
		r.Lobbies = *cutAsyncMap(&r.Lobbies, 1000)
	}

	rowsByMode := make(map[string][]statRow)
	var playersInLobbies int
	var lobbiesCount int

	r.Lobbies.Range(func(key, value interface{}) bool {
		id := key.(string)
		stat := value.(*LobbyStat)
		stat.Lock()
		defer stat.Unlock()

		lobbiesCount++
		playersInLobbies += stat.Players

		// Rating stats
		var sum int32
		minInt := int32(math.MaxInt32)
		maxInt := int32(math.MinInt32)
		for _, rating := range stat.RatingSet {
			sum += rating
			if rating < minInt {
				minInt = rating
			}
			if rating > maxInt {
				maxInt = rating
			}
		}
		avg := 0.0
		if len(stat.RatingSet) > 0 {
			avg = float64(sum) / float64(len(stat.RatingSet))
		}

		// Categories
		common := map[int32]bool{}
		all := map[int32]bool{}
		first := true

		for _, cats := range stat.CategoriesSet {
			playerCats := map[int32]bool{}
			for _, c := range cats {
				playerCats[c] = true
				all[c] = true
			}
			if first {
				for c := range playerCats {
					common[c] = true
				}
				first = false
			} else {
				for c := range common {
					if !playerCats[c] {
						delete(common, c)
					}
				}
			}
		}

		var commonSlice, uniqueSlice []int
		for c := range common {
			commonSlice = append(commonSlice, int(c))
		}
		for c := range all {
			if !common[c] {
				uniqueSlice = append(uniqueSlice, int(c))
			}
		}
		sort.Ints(commonSlice)
		sort.Ints(uniqueSlice)

		waitDur := "-"
		if !stat.StartedAt.IsZero() && !stat.CreatedAt.IsZero() {
			dur := stat.StartedAt.Sub(stat.CreatedAt)
			if dur < time.Second {
				waitDur = "<1s"
			} else {
				waitDur = dur.Truncate(time.Second).String()
			}
		}

		statusDef := "-"
		switch stat.Status {
		case StartedStatus:
			statusDef = "STARTED"
		case ExpiredStatus:
			statusDef = "EXPIRED"
		case ErroredStatus:
			statusDef = "ERROR"
		}

		row := statRow{
			id:           id,
			mode:         stat.Mode,
			count:        stat.Players,
			max:          stat.MaxPlayers,
			avgRating:    avg,
			minRating:    minInt,
			maxRating:    maxInt,
			commonCats:   commonSlice,
			uniqueCats:   uniqueSlice,
			waitDuration: waitDur,
			status:       statusDef,
		}
		rowsByMode[stat.Mode] = append(rowsByMode[stat.Mode], row)
		return true
	})

	// ===== Summary table =====
	summary := table.NewWriter()
	summary.SetOutputMirror(os.Stdout)
	summary.SetStyle(table.StyleRounded)
	summary.AppendHeader(table.Row{"📊 Metric", "📈 Value"})
	summary.AppendRows([]table.Row{
		{"👥 Total Players", r.TotalPlayers},
		{"🏟️ Total Lobbies", lobbiesCount},
		{"🚀 Started Lobbies", countAsyncMap(&r.Starter)},
		{"🔁 Waited Players", countAsyncMap(&r.WaitedPlayers)},
		{"⌛ Expired Lobbies", countAsyncMap(&r.Expired)},
		{"⌛ Expired Players", countAsyncMap(&r.ExpiredPlayers)},
		{"❌ Errored Lobbies", countAsyncMap(&r.Errored)},
		{"❌ Errored Players", countAsyncMap(&r.ErroredPlayers)},
		{"👥 Players in Lobbies", fmt.Sprintf("%d (%.1f%%)", playersInLobbies, float64(playersInLobbies)/float64(r.TotalPlayers)*100)},
		{"⌛️ Test Duration", fmt.Sprintf(r.FinishedAt.Sub(r.StartedAt).Truncate(time.Second).String())},
	})
	summary.Render()
	fmt.Println()

	// ===== Mode distribution table =====
	modeCount := 0
	r.Modes.Range(func(_, _ interface{}) bool {
		modeCount++
		return true
	})

	if modeCount > 0 {
		modeTbl := table.NewWriter()
		modeTbl.SetOutputMirror(os.Stdout)
		modeTbl.SetTitle("🎮 Players per Mode")
		modeTbl.AppendHeader(table.Row{"Mode", "Players"})
		modeTbl.SetStyle(table.StyleLight)

		var modes []string
		r.Modes.Range(func(key, value interface{}) bool {
			modes = append(modes, key.(string))
			return true
		})
		sort.Strings(modes)

		for _, mode := range modes {
			if count, ok := r.Modes.Load(mode); ok {
				modeTbl.AppendRow([]interface{}{mode, count})
			}
		}
		modeTbl.Render()
		fmt.Println()
	}

	// ===== Lobbies by Mode =====
	if len(rowsByMode) == 0 {
		fmt.Println("ℹ️  No active lobbies.")
		return
	}

	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()

	for _, mode := range sortedKeys(rowsByMode) {
		fmt.Printf("🎮 Mode: %s\n", mode)
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleBold)
		t.AppendHeader(table.Row{
			"Lobby ID", "Players", "Avg Rating", "Min", "Max",
			"Common Cats", "Unique Cats", "Status", "Wait",
		})

		for _, row := range rowsByMode[mode] {
			t.AppendRow([]interface{}{
				row.id,
				fmt.Sprintf("%d/%d", row.count, row.max),
				yellow(int32(row.avgRating)),
				red(row.minRating),
				green(row.maxRating),
				formatCatList(row.commonCats),
				formatCatList(row.uniqueCats),
				row.status,
				row.waitDuration,
			})
		}
		t.Render()
		fmt.Println()
	}
}

func sortedKeys[V any](m map[string]V) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
