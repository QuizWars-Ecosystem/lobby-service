package report

import (
	"sync"
	"time"
)

const (
	StartedStatus = "started"
	WaitedStatus  = "waited"
	ErroredStatus = "errored"
	ExpiredStatus = "expired"
)

type LobbyStat struct {
	Mode          string
	Players       int
	MaxPlayers    int
	RatingSet     map[string]int32
	CategoriesSet map[string][]int32
	CreatedAt     time.Time
	StartedAt     time.Time
	Status        string
}

type Result struct {
	TotalPlayers   int
	Lobbies        map[string]LobbyStat
	Modes          map[string]int
	Starter        map[string]struct{}
	WaitedPlayers  map[string]struct{}
	Expired        map[string]struct{}
	ExpiredPlayers map[string]struct{}
	Errored        map[string]struct{}
	ErroredPlayers map[string]struct{}
	sync.Mutex
}
