package report

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

func (r *Result) LogStatsHTML() {
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
	var playersInLobbies, lobbiesCount int32

	for _, lobby := range r.lobbies {
		playersInLobbies += lobby.players
		lobbiesCount++
	}

	htmlBuilder.WriteString("<h2>📊 Summary</h2><table><tr><th>Metric</th><th>Value</th></tr>")

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
		"⌛️ Requesting Duration",
		"⌛️ Test Duration",
	}

	summaryValues := []string{
		fmt.Sprint(r.totalPlayers),
		fmt.Sprint(lobbiesCount),
		fmt.Sprint(len(r.starter)),
		fmt.Sprint(len(r.waitedPlayers)),
		fmt.Sprint(len(r.expired)),
		fmt.Sprint(len(r.expiredPlayers)),
		fmt.Sprint(len(r.errored)),
		fmt.Sprint(len(r.erroredPlayers)),
		fmt.Sprintf("%d (%.1f%%)", playersInLobbies, float64(playersInLobbies)/float64(r.totalPlayers)*100),
		fmt.Sprintf(r.finishRequesting.Sub(r.startedAt).Truncate(time.Second).String()),
		fmt.Sprintf(r.finishedAt.Sub(r.startedAt).Truncate(time.Second).String()),
	}

	for i := range summaryTitles {
		htmlBuilder.WriteString("<tr><td>" + summaryTitles[i] + "</td><td>" + summaryValues[i] + "</td></tr>")
	}

	htmlBuilder.WriteString("</table>")

	// ===== Mode Table =====
	var modeCount = len(r.modes)

	if modeCount > 0 {
		htmlBuilder.WriteString("<h2>🎮 Players per Mode</h2><table><tr><th onclick='toggleSort(this.parentNode.parentNode, 0)'>Mode</th><th onclick='toggleSort(this.parentNode.parentNode, 1)'>Players</th></tr>")

		var modes []string

		for mode := range r.modes {
			modes = append(modes, mode)
		}

		sort.Strings(modes)

		for _, mode := range modes {
			if count, ok := r.modes[mode]; ok {
				htmlBuilder.WriteString("<tr><td>" + mode + "</td><td>" + fmt.Sprint(count) + "</td></tr>")
			}
		}
		htmlBuilder.WriteString("</table>")
	}

	// ===== Detailed per-mode Stats =====
	rowsByMode := make(map[string][]statRow)
	threshold := 0.6

	for id, lobby := range r.lobbies {
		lobbiesCount++
		playersInLobbies += lobby.players

		// Rating stats
		var sum int32
		minInt := int32(math.MaxInt32)
		maxInt := int32(math.MinInt32)

		for _, rating := range lobby.ratingSet {
			sum += rating
			if rating < minInt {
				minInt = rating
			}
			if rating > maxInt {
				maxInt = rating
			}
		}

		avg := 0.0
		if len(lobby.ratingSet) > 0 {
			avg = float64(sum) / float64(len(lobby.ratingSet))
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
		if !lobby.startedAt.IsZero() && !lobby.createdAt.IsZero() {
			dur := lobby.startedAt.Sub(lobby.createdAt)
			if dur < time.Second {
				waitDur = "<1s"
			} else {
				waitDur = dur.Truncate(time.Second).String()
			}
		}

		statusDef := "-"
		switch lobby.status {
		case startedStatus:
			statusDef = "STARTED"
		case expiredStatus:
			statusDef = "EXPIRED"
		case erroredStatus:
			statusDef = "ERROR"
		}

		row := statRow{
			id:           id,
			mode:         lobby.mode,
			count:        int(lobby.players),
			max:          int(lobby.maxPlayers),
			avgRating:    avg,
			minRating:    minInt,
			maxRating:    maxInt,
			commonCats:   commonSlice,
			uniqueCats:   uniqueSlice,
			waitDuration: waitDur,
			status:       statusDef,
		}

		rowsByMode[lobby.mode] = append(rowsByMode[lobby.mode], row)
	}

	if len(rowsByMode) == 0 {
		htmlBuilder.WriteString("<p><em>No active lobbies.</em></p>")
	} else {
		for _, mode := range sortedKeys(rowsByMode) {
			htmlBuilder.WriteString("<h3>🎮 Mode: " + mode + "</h3>")
			htmlBuilder.WriteString(`<table>
			<tr><th onclick="toggleSort(this.parentNode.parentNode, 0)">Lobby ID</th><th onclick="toggleSort(this.parentNode.parentNode, 1)">Players</th><th onclick="toggleSort(this.parentNode.parentNode, 2)">Avg Rating</th><th onclick="toggleSort(this.parentNode.parentNode, 3)">Min</th><th onclick="toggleSort(this.parentNode.parentNode, 4)">Max</th><th onclick="toggleSort(this.parentNode.parentNode, 5)">Common Cats (` + fmt.Sprintf(">=%d", int(threshold*100)) + `%)</th><th onclick="toggleSort(this.parentNode.parentNode, 6)">Unique Cats</th><th onclick="toggleSort(this.parentNode.parentNode, 7)">Status</th><th onclick="toggleSort(this.parentNode.parentNode, 8)">Wait</th></tr>`)
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
