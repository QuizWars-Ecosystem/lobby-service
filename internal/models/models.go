package models

import "time"

type Player struct {
	ID         string    `json:"id"`
	Rating     int32     `json:"rating"`
	Categories []int32   `json:"categories"`
	JoinedAt   time.Time `json:"joined_at"`
}

type Lobby struct {
	ID           string    `json:"id"`
	Mode         string    `json:"mode"`
	Categories   []int32   `json:"categories"`
	Players      []*Player `json:"players"`
	MinPlayers   int16     `json:"min_players"`
	MaxPlayers   int16     `json:"max_players"`
	AvgRating    int32     `json:"avg_rating"`
	CreatedAt    time.Time `json:"created_at"`
	LastJoinedAt time.Time `json:"last_joined_at"`
	ExpireAt     time.Time `json:"expire_at"`
	Version      int16     `json:"version"`
}

func (l *Lobby) AddPlayer(player *Player) bool {
	if int16(len(l.Players)) >= l.MaxPlayers {
		return false
	}

	l.Players = append(l.Players, player)
	l.AvgRating = countAvgRating(l.Players)
	l.Categories = mergeCategories(l.Categories, player.Categories)
	player.JoinedAt = time.Now()
	l.LastJoinedAt = time.Now()
	l.Version++

	return true
}

func (l *Lobby) IncVersion() {
	l.Version++
}

func (l *Lobby) CanAddPlayer() bool {
	return int16(len(l.Players)) < l.MaxPlayers
}

func countAvgRating(players []*Player) int32 {
	var total int32

	for _, player := range players {
		total += player.Rating
	}

	return total / int32(len(players))
}

func mergeCategories(a, b []int32) []int32 {
	set := make(map[int32]struct{}, len(a)+len(b))

	for _, v := range a {
		set[v] = struct{}{}
	}

	for _, v := range b {
		set[v] = struct{}{}
	}

	result := make([]int32, 0, len(set))
	for k := range set {
		result = append(result, k)
	}

	return result
}
