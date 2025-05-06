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
	sync.Mutex
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
	sync.RWMutex
	TotalPlayers   int
	Lobbies        sync.Map // map[string]*LobbyStat (thread-safe)
	Modes          sync.Map // map[string]int
	Starter        sync.Map // map[string]bool
	WaitedPlayers  sync.Map // map[string]bool
	Expired        sync.Map
	ExpiredPlayers sync.Map
	Errored        sync.Map
	ErroredPlayers sync.Map

	StartedAt  time.Time
	FinishedAt time.Time
}

func NewResult(playersCount int) *Result {
	return &Result{
		TotalPlayers:   playersCount,
		Lobbies:        sync.Map{},
		Modes:          sync.Map{},
		Starter:        sync.Map{},
		WaitedPlayers:  sync.Map{},
		ExpiredPlayers: sync.Map{},
		Errored:        sync.Map{},
		ErroredPlayers: sync.Map{},
		StartedAt:      time.Now(),
		FinishedAt:     time.Now(),
	}
}

func (r *Result) IncMode(mode string) {
	val, _ := r.Modes.LoadOrStore(mode, 0)
	r.Modes.Store(mode, val.(int)+1)
}

func (r *Result) AddToMap(m *sync.Map, key string) {
	m.Store(key, true)
}

func (r *Result) GetOrCreateLobby(lobbyID string, creator func() *LobbyStat) *LobbyStat {
	if l, ok := r.Lobbies.Load(lobbyID); ok {
		return l.(*LobbyStat)
	}
	lobby := creator()
	actual, _ := r.Lobbies.LoadOrStore(lobbyID, lobby)
	return actual.(*LobbyStat)
}
