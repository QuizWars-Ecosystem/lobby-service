package models

type Player struct {
	ID         string  `json:"id"`
	Rating     int32   `json:"rating"`
	Categories []int32 `json:"categories"`
}

type Lobby struct {
	ID         string    `json:"id"`
	Mode       string    `json:"mode"`
	Categories []int32   `json:"categories"`
	Players    []*Player `json:"players"`
	AvgRating  int32     `json:"avg_rating"`
	MinPlayers int       `json:"min_players"`
	MaxPlayers int       `json:"max_players"`
}
