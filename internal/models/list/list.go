package list

import (
	"container/list"
	"math/rand"
	"sync"
	"time"
)

const (
	maxLevel    = 32
	probability = 0.25
)

type ScoredLobby struct {
	ID    string
	Score float64
}

type skipListNode struct {
	score   float64
	lobbyID string
	forward []*skipListNode
}

type nodePool struct {
	pool *list.List
	mu   sync.Mutex
}

func newNodePool() *nodePool {
	return &nodePool{
		pool: list.New(),
	}
}

func (p *nodePool) getNode(lobbyID string, score float64) *skipListNode {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pool.Len() > 0 {
		node := p.pool.Remove(p.pool.Front()).(*skipListNode)
		node.lobbyID = lobbyID
		node.score = score
		node.forward = node.forward[:0]
		return node
	}
	return &skipListNode{
		lobbyID: lobbyID,
		score:   score,
		forward: make([]*skipListNode, 0, maxLevel),
	}
}

func (p *nodePool) putNode(node *skipListNode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pool.PushBack(node)
}

type ScoredLobbyList struct {
	mu       sync.RWMutex
	header   *skipListNode
	level    int
	size     int
	nodes    map[string]*skipListNode
	randSrc  rand.Source
	nodePool *nodePool
}

func NewScoredLobbyList() *ScoredLobbyList {
	sl := &ScoredLobbyList{
		header: &skipListNode{
			score:   0,
			forward: make([]*skipListNode, maxLevel),
		},
		level:    1,
		nodes:    make(map[string]*skipListNode),
		randSrc:  rand.NewSource(time.Now().UnixNano()),
		nodePool: newNodePool(),
	}

	for i := 0; i < 1000; i++ {
		sl.nodePool.putNode(&skipListNode{})
	}

	return sl
}

func (s *ScoredLobbyList) Add(lobbyID string, score float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node, exists := s.nodes[lobbyID]; exists {
		s.removeNodeImpl(node.lobbyID, node.score)
	}
	s.insertNodeImpl(lobbyID, score)
}

func (s *ScoredLobbyList) Get(lobbyID string) (ScoredLobby, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if node, exists := s.nodes[lobbyID]; exists {
		return ScoredLobby{ID: node.lobbyID, Score: node.score}, true
	}
	return ScoredLobby{}, false
}

func (s *ScoredLobbyList) Remove(lobbyID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node, exists := s.nodes[lobbyID]; exists {
		return s.removeNodeImpl(node.lobbyID, node.score)
	}
	return false
}

func (s *ScoredLobbyList) GetTop(n int) []ScoredLobby {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ScoredLobby, 0, n)
	current := s.header.forward[0]

	for i := 0; i < n && current != nil; i++ {
		result = append(result, ScoredLobby{
			ID:    current.lobbyID,
			Score: current.score,
		})
		current = current.forward[0]
	}

	return result
}

func (s *ScoredLobbyList) GetByScoreRange(min, max float64) []ScoredLobby {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []ScoredLobby
	current := s.header

	for i := s.level - 1; i >= 0; i-- {
		for current.forward[i] != nil && current.forward[i].score < min {
			current = current.forward[i]
		}
	}

	current = current.forward[0]

	for current != nil && current.score <= max {
		result = append(result, ScoredLobby{
			ID:    current.lobbyID,
			Score: current.score,
		})
		current = current.forward[0]
	}

	return result
}

func (s *ScoredLobbyList) GetAll() []ScoredLobby {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ScoredLobby, 0, s.size)
	current := s.header.forward[0]

	for current != nil {
		result = append(result, ScoredLobby{
			ID:    current.lobbyID,
			Score: current.score,
		})
		current = current.forward[0]
	}

	return result
}

func (s *ScoredLobbyList) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.size
}

func (s *ScoredLobbyList) insertNodeImpl(lobbyID string, score float64) {
	update := make([]*skipListNode, maxLevel)
	current := s.header

	for i := s.level - 1; i >= 0; i-- {
		for current.forward[i] != nil && current.forward[i].score < score {
			current = current.forward[i]
		}
		update[i] = current
	}

	level := s.randomLevel()
	if level > s.level {
		for i := s.level; i < level; i++ {
			update[i] = s.header
		}
		s.level = level
	}

	newNode := s.nodePool.getNode(lobbyID, score)
	newNode.forward = make([]*skipListNode, level)

	for i := 0; i < level; i++ {
		newNode.forward[i] = update[i].forward[i]
		update[i].forward[i] = newNode
	}

	s.nodes[lobbyID] = newNode
	s.size++
}

func (s *ScoredLobbyList) removeNodeImpl(lobbyID string, score float64) bool {
	update := make([]*skipListNode, maxLevel)
	current := s.header

	for i := s.level - 1; i >= 0; i-- {
		for current.forward[i] != nil && current.forward[i].score < score {
			current = current.forward[i]
		}
		update[i] = current
	}

	current = current.forward[0]
	if current == nil || current.lobbyID != lobbyID {
		return false
	}

	for i := 0; i < s.level; i++ {
		if update[i].forward[i] != current {
			break
		}
		update[i].forward[i] = current.forward[i]
	}

	for s.level > 1 && s.header.forward[s.level-1] == nil {
		s.level--
	}

	s.nodePool.putNode(current)
	delete(s.nodes, lobbyID)
	s.size--
	return true
}

func (s *ScoredLobbyList) randomLevel() int {
	level := 1
	for level < maxLevel && s.randSrc.Int63() < int64(probability*float64(1<<63)) {
		level++
	}
	return level
}
