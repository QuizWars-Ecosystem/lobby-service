package report

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func (r *Result) LogStatsHTML() {
	r.Lock()
	defer r.Unlock()

	file, err := os.Create(fmt.Sprintf("../reports/lobby_stats_%s.html", time.Now().Format("2006-01-02-1504")))
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = file.Close()
	}()

	var htmlBuilder strings.Builder

	htmlBuilder.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8">
	<title>Lobby Stats</title>
	<style>
		body { font-family: sans-serif; padding: 20px; }
		table { border-collapse: collapse; width: 100%; margin-bottom: 2em; }
		th, td { border: 1px solid #ccc; padding: 8px; text-align: left; }
		th { background-color: #f0f0f0; cursor: pointer; }
		th.sorted-asc { background-color: #e1e1e1; }
		th.sorted-desc { background-color: #d0d0d0; }
		.red { color: red; }
		.green { color: green; }
		.yellow { color: orange; }
	</style>
	<script>
		function sortTable(table, colIndex, asc) {
			var rows = Array.from(table.rows).slice(1);
			rows.sort((rowA, rowB) => {
				var cellA = rowA.cells[colIndex].innerText;
				var cellB = rowB.cells[colIndex].innerText;

				if (colIndex === 1 || colIndex === 2 || colIndex === 3 || colIndex === 4 || colIndex === 5) {
					cellA = parseInt(cellA.replace(/[^\d.-]/g, ''), 10);
					cellB = parseInt(cellB.replace(/[^\d.-]/g, ''), 10);
				}

				if (asc) {
					return cellA > cellB ? 1 : cellA < cellB ? -1 : 0;
				} else {
					return cellA < cellB ? 1 : cellA > cellB ? -1 : 0;
				}
			});

			rows.forEach(row => table.appendChild(row));
		}

		function toggleSort(table, colIndex) {
			var th = table.rows[0].cells[colIndex];
			var isAsc = th.classList.contains('sorted-asc');
			var newIsAsc = !isAsc;
			Array.from(th.parentNode.children).forEach(cell => cell.classList.remove('sorted-asc', 'sorted-desc'));

			th.classList.add(newIsAsc ? 'sorted-asc' : 'sorted-desc');

			sortTable(table, colIndex, newIsAsc);
		}
	</script>
	</head><body>
	<h1>Lobby Statistics</h1>`)

	// ===== Summary Table =====
	playersInLobbies := 0
	var lobbiesCount int
	r.Lobbies.Range(func(_, value interface{}) bool {
		stat := value.(*LobbyStat)
		stat.Lock()
		playersInLobbies += stat.Players
		stat.Unlock()
		lobbiesCount++
		return true
	})

	htmlBuilder.WriteString("<h2>📊 Summary</h2><table><tr><th>Metric</th><th>Value</th></tr>")

	countMap := func(m *sync.Map) int {
		count := 0
		m.Range(func(_, _ interface{}) bool {
			count++
			return true
		})
		return count
	}

	summaryTitles := []string{
		"👥 Total Players",
		"🏟️ Total Lobbies",
		"🚀 Started Lobbies",
		"🔁 Waited Players",
		"⌛ Expired Lobbies",
		"⌛ Expired Players",
		"❌ Errored Lobbies",
		"❌ Errored Players",
		"👥 Players in Lobbies",
		"⌛️ Test Duration",
	}

	summaryValues := []string{
		fmt.Sprint(r.TotalPlayers),
		fmt.Sprint(lobbiesCount),
		fmt.Sprint(countMap(&r.Starter)),
		fmt.Sprint(countMap(&r.WaitedPlayers)),
		fmt.Sprint(countMap(&r.Expired)),
		fmt.Sprint(countMap(&r.ExpiredPlayers)),
		fmt.Sprint(countMap(&r.Errored)),
		fmt.Sprint(countMap(&r.ErroredPlayers)),
		fmt.Sprintf("%d (%.1f%%)", playersInLobbies, float64(playersInLobbies)/float64(r.TotalPlayers)*100),
		fmt.Sprintf(r.FinishedAt.Sub(r.StartedAt).Truncate(time.Second).String()),
	}

	for i := range summaryTitles {
		htmlBuilder.WriteString("<tr><td>" + summaryTitles[i] + "</td><td>" + summaryValues[i] + "</td></tr>")
	}

	htmlBuilder.WriteString("</table>")

	// ===== Mode Table =====
	var modeCount int
	r.Modes.Range(func(_, _ interface{}) bool {
		modeCount++
		return true
	})

	if modeCount > 0 {
		htmlBuilder.WriteString("<h2>🎮 Players per Mode</h2><table><tr><th onclick='toggleSort(this.parentNode.parentNode, 0)'>Mode</th><th onclick='toggleSort(this.parentNode.parentNode, 1)'>Players</th></tr>")

		var modes []string
		r.Modes.Range(func(key, value interface{}) bool {
			modes = append(modes, key.(string))
			return true
		})
		sort.Strings(modes)

		for _, mode := range modes {
			if count, ok := r.Modes.Load(mode); ok {
				htmlBuilder.WriteString("<tr><td>" + mode + "</td><td>" + fmt.Sprint(count) + "</td></tr>")
			}
		}
		htmlBuilder.WriteString("</table>")
	}

	// ===== Detailed per-mode Stats =====
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

	rowsByMode := make(map[string][]statRow)

	r.Lobbies.Range(func(key, value interface{}) bool {
		id := key.(string)
		stat := value.(*LobbyStat)
		stat.Lock()
		defer stat.Unlock()

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

	if len(rowsByMode) == 0 {
		htmlBuilder.WriteString("<p><em>No active lobbies.</em></p>")
	} else {
		for _, mode := range sortedKeys(rowsByMode) {
			htmlBuilder.WriteString("<h3>🎮 Mode: " + mode + "</h3>")
			htmlBuilder.WriteString(`<table>
			<tr><th onclick="toggleSort(this.parentNode.parentNode, 0)">Lobby ID</th><th onclick="toggleSort(this.parentNode.parentNode, 1)">Players</th><th onclick="toggleSort(this.parentNode.parentNode, 2)">Avg Rating</th><th onclick="toggleSort(this.parentNode.parentNode, 3)">Min</th><th onclick="toggleSort(this.parentNode.parentNode, 4)">Max</th><th onclick="toggleSort(this.parentNode.parentNode, 5)">Common Cats</th><th onclick="toggleSort(this.parentNode.parentNode, 6)">Unique Cats</th><th onclick="toggleSort(this.parentNode.parentNode, 7)">Status</th><th onclick="toggleSort(this.parentNode.parentNode, 8)">Wait</th></tr>`)
			for _, row := range rowsByMode[mode] {
				htmlBuilder.WriteString("<tr>")
				htmlBuilder.WriteString("<td>" + row.id + "</td>")
				htmlBuilder.WriteString("<td>" + fmt.Sprintf("%d/%d", row.count, row.max) + "</td>")
				htmlBuilder.WriteString(`<td class="yellow">` + fmt.Sprint(int(row.avgRating)) + "</td>")
				htmlBuilder.WriteString(`<td class="red">` + fmt.Sprint(row.minRating) + "</td>")
				htmlBuilder.WriteString(`<td class="green">` + fmt.Sprint(row.maxRating) + "</td>")
				htmlBuilder.WriteString("<td>" + formatCatList(row.commonCats) + "</td>")
				htmlBuilder.WriteString("<td>" + formatCatList(row.uniqueCats) + "</td>")
				switch row.status {
				case "STARTED":
					htmlBuilder.WriteString(`<td class="green">` + row.status + "</td>")
				case "EXPIRED":
					htmlBuilder.WriteString(`<td class="yellow">` + row.status + "</td>")
				case "ERROR":
					htmlBuilder.WriteString(`<td class="red">` + row.status + "</td>")
				default:
					htmlBuilder.WriteString("<td>" + row.status + "</td>")
				}
				htmlBuilder.WriteString("<td>" + row.waitDuration + "</td>")
				htmlBuilder.WriteString("</tr>")
			}
			htmlBuilder.WriteString("</table>")
		}
	}

	htmlBuilder.WriteString("</body></html>")

	_, err = file.WriteString(htmlBuilder.String())
	if err != nil {
		panic(err)
	}
}
