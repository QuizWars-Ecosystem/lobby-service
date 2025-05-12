package report

import (
	"github.com/QuizWars-Ecosystem/lobby-service/tests/integration_tests/config"
	"sync"
	"time"
)

const (
	startedStatus = "started"
	waitedStatus  = "waited"
	erroredStatus = "Errored"
	expiredStatus = "Expired"
)

type LobbyStatCreator struct {
	Mode          string
	Players       int
	MaxPlayers    int
	RatingSet     map[string]int32
	CategoriesSet map[string][]int32
	CreatedAt     time.Time
	StartedAt     time.Time
	Status        string
}

type LobbyStat struct {
	Mode          string
	playersMu     sync.Mutex
	Players       int32
	MaxPlayers    int32
	ratingMu      sync.RWMutex
	RatingSet     map[string]int32
	categoriesMu  sync.RWMutex
	categoriesSet map[string][]int32
	CreatedAt     time.Time
	startedMu     sync.Mutex
	StartedAt     time.Time
	statusMu      sync.Mutex
	Status        string
}

type Result struct {
	TotalPlayers     int
	LobbiesMu        sync.RWMutex
	Lobbies          map[string]*LobbyStat
	modesMu          sync.RWMutex
	Modes            map[string]int
	startedMu        sync.RWMutex
	Starter          map[string]struct{}
	waitedMu         sync.RWMutex
	WaitedPlayers    map[string]struct{}
	expiredMu        sync.RWMutex
	Expired          map[string]struct{}
	expiredPlayersMu sync.RWMutex
	ExpiredPlayers   map[string]struct{}
	erroredMu        sync.RWMutex
	Errored          map[string]struct{}
	erroredPlayersMu sync.RWMutex
	ErroredPlayers   map[string]struct{}

	Cfg *config.TestConfig

	StartedAt        time.Time
	FinishedAt       time.Time
	FinishRequesting time.Time
}

func NewResult(playersCount int, cfg *config.TestConfig) *Result {
	return &Result{
		TotalPlayers:   playersCount,
		Lobbies:        make(map[string]*LobbyStat, playersCount/4),
		Modes:          make(map[string]int, playersCount/4),
		Starter:        make(map[string]struct{}, playersCount/10),
		WaitedPlayers:  make(map[string]struct{}, playersCount/10),
		Expired:        make(map[string]struct{}),
		ExpiredPlayers: make(map[string]struct{}),
		Errored:        make(map[string]struct{}),
		ErroredPlayers: make(map[string]struct{}),
		Cfg:            cfg,
		StartedAt:      time.Now(),
		FinishedAt:     time.Now(),
	}
}

func (r *Result) Freeze() {
	r.LobbiesMu.Lock()
	r.modesMu.Lock()
	r.startedMu.Lock()
	r.waitedMu.Lock()
	r.expiredMu.Lock()
	r.expiredPlayersMu.Lock()
	r.erroredMu.Lock()
	r.erroredPlayersMu.Lock()
}

func (r *Result) Unfreeze() {
	r.erroredPlayersMu.Unlock()
	r.erroredMu.Unlock()
	r.expiredPlayersMu.Unlock()
	r.expiredMu.Unlock()
	r.waitedMu.Unlock()
	r.startedMu.Unlock()
	r.modesMu.Unlock()
	r.LobbiesMu.Unlock()
}

func (r *Result) GetOrCreateLobby(id string, creator func() LobbyStatCreator) *LobbyStat {
	r.LobbiesMu.RLock()
	lobby, exists := r.Lobbies[id]
	r.LobbiesMu.RUnlock()
	if exists {
		return lobby
	}

	r.LobbiesMu.Lock()
	defer r.LobbiesMu.Unlock()
	if lobby, exists = r.Lobbies[id]; exists {
		return lobby
	}

	newLobbySample := creator()
	newLobby := &LobbyStat{
		Mode:          newLobbySample.Mode,
		Players:       int32(newLobbySample.Players),
		MaxPlayers:    int32(newLobbySample.MaxPlayers),
		RatingSet:     newLobbySample.RatingSet,
		categoriesSet: newLobbySample.CategoriesSet,
		CreatedAt:     newLobbySample.CreatedAt,
		StartedAt:     newLobbySample.StartedAt,
		Status:        newLobbySample.Status,
	}

	r.Lobbies[id] = newLobby
	return newLobby
}

func (r *Result) Start() {
	r.StartedAt = time.Now()
}

func (r *Result) Finish() {
	r.FinishedAt = time.Now()
}

func (r *Result) FinishRequestingMethod() {
	r.FinishRequesting = time.Now()
}

func (r *Result) ModeInc(mode string) *Result {
	r.modesMu.Lock()
	defer r.modesMu.Unlock()
	r.Modes[mode]++
	return r
}

func (r *Result) AddStartedLobby(id string) *Result {
	r.startedMu.Lock()
	defer r.startedMu.Unlock()
	r.Starter[id] = struct{}{}
	return r
}

func (r *Result) AddWaitedPlayer(id string) *Result {
	r.waitedMu.Lock()
	defer r.waitedMu.Unlock()
	r.WaitedPlayers[id] = struct{}{}
	return r
}

func (r *Result) AddExpiredLobby(id string) *Result {
	r.expiredMu.Lock()
	defer r.expiredMu.Unlock()
	r.Expired[id] = struct{}{}
	return r
}

func (r *Result) AddExpiredPlayer(id string) *Result {
	r.expiredPlayersMu.Lock()
	defer r.expiredPlayersMu.Unlock()
	r.ExpiredPlayers[id] = struct{}{}
	return r
}

func (r *Result) AddErroredLobby(id string) *Result {
	r.erroredMu.Lock()
	defer r.erroredMu.Unlock()
	r.Errored[id] = struct{}{}
	return r
}

func (r *Result) AddErroredPlayer(id string) *Result {
	r.erroredPlayersMu.Lock()
	defer r.erroredPlayersMu.Unlock()
	r.ErroredPlayers[id] = struct{}{}
	return r
}

func (l *LobbyStat) SetCurrentPlayers(players int32) *LobbyStat {
	l.playersMu.Lock()
	l.Players = players
	l.playersMu.Unlock()
	return l
}

func (l *LobbyStat) AddRating(id string, val int32) *LobbyStat {
	l.ratingMu.RLock()
	_, exists := l.RatingSet[id]
	l.ratingMu.RUnlock()

	if exists {
		return l
	}

	l.ratingMu.Lock()
	defer l.ratingMu.Unlock()
	l.RatingSet[id] = val
	return l
}

func (l *LobbyStat) AddCategories(id string, vals []int32) *LobbyStat {
	l.categoriesMu.RLock()
	_, exists := l.categoriesSet[id]
	l.categoriesMu.RUnlock()

	if exists {
		return l
	}

	l.categoriesMu.Lock()
	defer l.categoriesMu.Unlock()
	l.categoriesSet[id] = vals
	return l
}

func (l *LobbyStat) SetAsStarted() *LobbyStat {
	l.startedMu.Lock()
	l.StartedAt = time.Now()
	l.startedMu.Unlock()

	l.statusMu.Lock()
	l.Status = startedStatus
	l.statusMu.Unlock()
	return l
}

func (l *LobbyStat) SetAsWaited() *LobbyStat {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.Status = waitedStatus
	return l
}

func (l *LobbyStat) SetAsExpired() *LobbyStat {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.Status = expiredStatus
	return l
}

func (l *LobbyStat) SetAsErrored() *LobbyStat {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.Status = erroredStatus
	return l
}
