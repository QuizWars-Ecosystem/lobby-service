package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	LobbyWaitTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lobby_wait_seconds",
		Help:    "Time spent waiting for lobby to fill",
		Buckets: []float64{5, 10, 30, 60, 120, 300},
	}, []string{"mode"})

	LobbyPlayersCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lobby_players_current",
		Help: "Current number of players in lobby",
	}, []string{"lobby_id", "mode"})

	LobbyStatusChanges = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lobby_status_changes_total",
		Help: "Total lobby status changes",
	}, []string{"status"}) // waiting, starting, timeout, error

	ModeLobbiesCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mode_lobbies_active",
		Help: "Active lobbies count per mode",
	}, []string{"mode"})

	ModePlayersQueued = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mode_players_queued",
		Help: "Players waiting in queue per mode",
	}, []string{"mode"})

	ActiveGRPCStreams = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "grpc_streams_active",
		Help: "Current active gRPC streams",
	})

	GRPCStreamErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_stream_errors_total",
		Help: "Total gRPC stream errors",
	}, []string{"error_type"})
)

func Initialize() {
	prometheus.MustRegister(LobbyWaitTime)
	prometheus.MustRegister(LobbyPlayersCount)
	prometheus.MustRegister(LobbyStatusChanges)
	prometheus.MustRegister(ModeLobbiesCount)
	prometheus.MustRegister(ModePlayersQueued)
	prometheus.MustRegister(ActiveGRPCStreams)
	prometheus.MustRegister(GRPCStreamErrors)
}
