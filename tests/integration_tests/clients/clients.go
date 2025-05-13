package clients

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/QuizWars-Ecosystem/go-common/pkg/abstractions"
	test "github.com/QuizWars-Ecosystem/go-common/pkg/testing/server"
	lobbyv1 "github.com/QuizWars-Ecosystem/lobby-service/gen/external/lobby/v1"
	"google.golang.org/grpc"
)

type ServerSet struct {
	Server abstractions.Server
	Port   int
	stop   func()
}

type Manager struct {
	ctx     context.Context
	mu      sync.Mutex
	clients []lobbyv1.LobbyServiceClient
	coons   []*grpc.ClientConn
	current int
	servers []ServerSet
}

func NewManager(ctx context.Context, servers ...ServerSet) *Manager {
	return &Manager{
		ctx:     ctx,
		servers: servers,
	}
}

func (m *Manager) Start(t *testing.T) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var err error
	if m.servers == nil {
		return errors.New("servers is nil")
	}

	for i, set := range m.servers {
		conn, stop := test.RunServer(t, set.Server, set.Port)
		m.servers[i].stop = stop

		client := lobbyv1.NewLobbyServiceClient(conn)

		m.coons = append(m.coons, conn)
		m.clients = append(m.clients, client)
		time.Sleep(time.Millisecond * 250)
	}

	return err
}

func (m *Manager) GetClient() lobbyv1.LobbyServiceClient {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.clients) == 0 {
		return nil
	}

	client := m.clients[m.current]
	m.current = (m.current + 1) % len(m.clients)
	return client
}

func (m *Manager) AddServer(server ServerSet) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers = append(m.servers, server)
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.clients = nil

	var err error
	for _, conn := range m.coons {
		errors.Join(err, conn.Close())
	}

	stopCtx, cancel := context.WithTimeout(m.ctx, time.Second*30)
	defer cancel()

	for _, server := range m.servers {
		errors.Join(err, server.Server.Shutdown(stopCtx))
	}

	return err
}
