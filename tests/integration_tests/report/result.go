package report

import (
	"sync"
	"time"
)

const (
	startedStatus = "started"
	waitedStatus  = "waited"
	erroredStatus = "errored"
	expiredStatus = "expired"
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
	mode          string
	playersMu     sync.Mutex
	players       int32
	maxPlayers    int32
	ratingMu      sync.RWMutex
	ratingSet     map[string]int32
	categoriesMu  sync.RWMutex
	categoriesSet map[string][]int32
	createdAt     time.Time
	startedMu     sync.Mutex
	startedAt     time.Time
	statusMu      sync.Mutex
	status        string
}

type Result struct {
	totalPlayers     int
	lobbiesMu        sync.RWMutex
	lobbies          map[string]*LobbyStat
	modesMu          sync.RWMutex
	modes            map[string]int
	startedMu        sync.RWMutex
	starter          map[string]struct{}
	waitedMu         sync.RWMutex
	waitedPlayers    map[string]struct{}
	expiredMu        sync.RWMutex
	expired          map[string]struct{}
	expiredPlayersMu sync.RWMutex
	expiredPlayers   map[string]struct{}
	erroredMu        sync.RWMutex
	errored          map[string]struct{}
	erroredPlayersMu sync.RWMutex
	erroredPlayers   map[string]struct{}

	startedAt        time.Time
	finishedAt       time.Time
	finishRequesting time.Time
}

func NewResult(playersCount int) *Result {
	return &Result{
		totalPlayers:   playersCount,
		lobbies:        make(map[string]*LobbyStat, playersCount/4),
		modes:          make(map[string]int, playersCount/4),
		starter:        make(map[string]struct{}, playersCount/10),
		waitedPlayers:  make(map[string]struct{}, playersCount/10),
		expired:        make(map[string]struct{}),
		expiredPlayers: make(map[string]struct{}),
		errored:        make(map[string]struct{}),
		erroredPlayers: make(map[string]struct{}),
		startedAt:      time.Now(),
		finishedAt:     time.Now(),
	}
}

func (r *Result) Freeze() {
	r.lobbiesMu.Lock()
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
	r.lobbiesMu.Unlock()
}

func (r *Result) GetOrCreateLobby(id string, creator func() LobbyStatCreator) *LobbyStat {
	r.lobbiesMu.RLock()
	lobby, exists := r.lobbies[id]
	r.lobbiesMu.RUnlock()
	if exists {
		return lobby
	}

	r.lobbiesMu.Lock()
	defer r.lobbiesMu.Unlock()
	if lobby, exists = r.lobbies[id]; exists {
		return lobby
	}

	newLobbySample := creator()
	newLobby := &LobbyStat{
		mode:          newLobbySample.Mode,
		players:       int32(newLobbySample.Players),
		maxPlayers:    int32(newLobbySample.MaxPlayers),
		ratingSet:     newLobbySample.RatingSet,
		categoriesSet: newLobbySample.CategoriesSet,
		createdAt:     newLobbySample.CreatedAt,
		startedAt:     newLobbySample.StartedAt,
		status:        newLobbySample.Status,
	}

	r.lobbies[id] = newLobby
	return newLobby
}

func (r *Result) Start() {
	r.startedAt = time.Now()
}

func (r *Result) Finish() {
	r.finishedAt = time.Now()
}

func (r *Result) FinishRequesting() {
	r.finishRequesting = time.Now()
}

func (r *Result) ModeInc(mode string) *Result {
	r.modesMu.Lock()
	defer r.modesMu.Unlock()
	r.modes[mode]++
	return r
}

func (r *Result) AddStartedLobby(id string) *Result {
	r.startedMu.Lock()
	defer r.startedMu.Unlock()
	r.starter[id] = struct{}{}
	return r
}

func (r *Result) AddWaitedPlayer(id string) *Result {
	r.waitedMu.Lock()
	defer r.waitedMu.Unlock()
	r.waitedPlayers[id] = struct{}{}
	return r
}

func (r *Result) AddExpiredLobby(id string) *Result {
	r.expiredMu.Lock()
	defer r.expiredMu.Unlock()
	r.expired[id] = struct{}{}
	return r
}

func (r *Result) AddExpiredPlayer(id string) *Result {
	r.expiredPlayersMu.Lock()
	defer r.expiredPlayersMu.Unlock()
	r.expiredPlayers[id] = struct{}{}
	return r
}

func (r *Result) AddErroredLobby(id string) *Result {
	r.erroredMu.Lock()
	defer r.erroredMu.Unlock()
	r.errored[id] = struct{}{}
	return r
}

func (r *Result) AddErroredPlayer(id string) *Result {
	r.erroredPlayersMu.Lock()
	defer r.erroredPlayersMu.Unlock()
	r.erroredPlayers[id] = struct{}{}
	return r
}

func (l *LobbyStat) SetCurrentPlayers(players int32) *LobbyStat {
	l.playersMu.Lock()
	l.players = players
	l.playersMu.Unlock()
	return l
}

func (l *LobbyStat) AddRating(id string, val int32) *LobbyStat {
	l.ratingMu.RLock()
	_, exists := l.ratingSet[id]
	l.ratingMu.RUnlock()

	if exists {
		return l
	}

	l.ratingMu.Lock()
	defer l.ratingMu.Unlock()
	l.ratingSet[id] = val
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
	l.startedAt = time.Now()
	l.startedMu.Unlock()

	l.statusMu.Lock()
	l.status = startedStatus
	l.statusMu.Unlock()
	return l
}

func (l *LobbyStat) SetAsWaited() *LobbyStat {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.status = waitedStatus
	return l
}

func (l *LobbyStat) SetAsExpired() *LobbyStat {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.status = expiredStatus
	return l
}

func (l *LobbyStat) SetAsErrored() *LobbyStat {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.status = erroredStatus
	return l
}
