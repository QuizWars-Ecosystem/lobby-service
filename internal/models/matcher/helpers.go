package matcher

import "math"

func ratingScore(playerRating, avgRating int32, maxDiff float64) float64 {
	diff := math.Abs(float64(playerRating - avgRating))
	score := 1.0 - (diff / maxDiff)
	if score < 0 {
		return 0
	}
	return score
}

func categoryScore(playerCategories, lobbyCategories []int32) float64 {
	return jaccardIndex(playerCategories, lobbyCategories)
}

func fillScore(playersCount, maxPlayers int) float64 {
	if maxPlayers == 0 {
		return 0
	}
	return float64(playersCount) / float64(maxPlayers)
}

func jaccardIndex(a, b []int32) float64 {
	setA := make(map[int32]struct{}, len(a))
	setB := make(map[int32]struct{}, len(b))

	for _, v := range a {
		setA[v] = struct{}{}
	}
	for _, v := range b {
		setB[v] = struct{}{}
	}

	inter := 0
	for v := range setA {
		if _, ok := setB[v]; ok {
			inter++
		}
	}

	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
