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

type statRow struct {
	id            string
	mode          string
	count         int
	max           int
	avgRating     float64
	minRating     int32
	maxRating     int32
	commonCats    []int
	uniqueCats    []int
	waitDuration  string
	status        string
	ratingSet     map[string]int32
	categoriesSet map[string][]int32
}

func (r *Result) LogStatsPrint() {
	rowsByMode := make(map[string][]statRow, len(r.Modes))
	var playersInLobbies int32
	var lobbiesCount int

	threshold := 0.6

	for id, lobby := range r.Lobbies {
		lobbiesCount++
		playersInLobbies += lobby.Players

		// Rating stats
		var sum int32
		minInt := int32(math.MaxInt32)
		maxInt := int32(math.MinInt32)

		for _, rating := range lobby.RatingSet {
			sum += rating
			if rating < minInt {
				minInt = rating
			}
			if rating > maxInt {
				maxInt = rating
			}
		}

		avg := 0.0
		if len(lobby.RatingSet) > 0 {
			avg = float64(sum) / float64(len(lobby.RatingSet))
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

		var commonSlice []int
		var uniqueSlice []int

		for cat, count := range categoryCount {
			if float64(count)/float64(numPlayers) >= threshold {
				commonSlice = append(commonSlice, int(cat))
			} else {
				uniqueSlice = append(uniqueSlice, int(cat))
			}
		}

		sort.Ints(commonSlice)
		sort.Ints(uniqueSlice)

		waitDur := "-"
		if !lobby.StartedAt.IsZero() && !lobby.CreatedAt.IsZero() {
			dur := lobby.StartedAt.Sub(lobby.CreatedAt)
			if dur < time.Second {
				waitDur = "<1s"
			} else {
				waitDur = dur.Truncate(time.Second).String()
			}
		}

		statusDef := "-"
		switch lobby.Status {
		case startedStatus:
			statusDef = "STARTED"
		case expiredStatus:
			statusDef = "EXPIRED"
		case erroredStatus:
			statusDef = "ERROR"
		}

		row := statRow{
			id:           id,
			mode:         lobby.Mode,
			count:        int(lobby.Players),
			max:          int(lobby.MaxPlayers),
			avgRating:    avg,
			minRating:    minInt,
			maxRating:    maxInt,
			commonCats:   commonSlice,
			uniqueCats:   uniqueSlice,
			waitDuration: waitDur,
			status:       statusDef,
		}

		rowsByMode[lobby.Mode] = append(rowsByMode[lobby.Mode], row)
	}

	// ===== Summary table =====
	summary := table.NewWriter()
	summary.SetOutputMirror(os.Stdout)
	summary.SetStyle(table.StyleRounded)
	summary.AppendHeader(table.Row{"📊 Metric", "📈 Value"})
	summary.AppendRows([]table.Row{
		{"👥 Total Players", r.TotalPlayers},
		{"🏟️ Total Lobbies", lobbiesCount},
		{"🚀 Started Lobbies", len(r.Starter)},
		{"🔁 Waited Players", len(r.WaitedPlayers)},
		{"⌛ Expired Lobbies", len(r.Expired)},
		{"⌛ Expired Players", len(r.ExpiredPlayers)},
		{"❌ Errored Lobbies", len(r.Errored)},
		{"❌ Errored Players", len(r.ErroredPlayers)},
		{"👥 Players in Lobbies", fmt.Sprintf("%d (%.1f%%)", playersInLobbies, float64(playersInLobbies)/float64(r.TotalPlayers)*100)},
		{"⌛️ Requesting Duration", fmt.Sprintf(r.FinishRequesting.Sub(r.StartedAt).Truncate(time.Second).String())},
		{"⌛️ Test Duration", fmt.Sprintf(r.FinishedAt.Sub(r.StartedAt).Truncate(time.Second).String())},
	})
	summary.Render()
	fmt.Println()

	// ===== Mode distribution table =====
	modeCount := len(r.Modes)

	if modeCount > 0 {
		modeTbl := table.NewWriter()
		modeTbl.SetOutputMirror(os.Stdout)
		modeTbl.SetTitle("🎮 Players per Mode")
		modeTbl.AppendHeader(table.Row{"Mode", "Players"})
		modeTbl.SetStyle(table.StyleLight)

		var modes []string
		for m := range r.Modes {
			modes = append(modes, m)
		}

		sort.Strings(modes)

		for _, mode := range modes {
			if count, ok := r.Modes[mode]; ok {
				modeTbl.AppendRow([]interface{}{mode, count})
			}
		}

		modeTbl.Render()
		fmt.Println()
	}

	// ===== Lobbies by Mode =====
	if len(rowsByMode) == 0 {
		fmt.Println("ℹ️  No active Lobbies.")
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
			fmt.Sprintln("Common Cats (>=", threshold*100, "%)"), "Unique Cats", "Status", "Wait",
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
