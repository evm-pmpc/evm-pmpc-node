package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

type Server struct {
	node      host.Host
	port      int
	authToken string
	startTime time.Time
}

func NewServer(node host.Host, port int, authToken string) *Server {
	return &Server{
		node:      node,
		port:      port,
		authToken: authToken,
		startTime: time.Now(),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/addresses", s.authenticate(s.handleGetAddresses))
	mux.HandleFunc("/api/v1/health", s.handleHealth)

	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	zap.L().Info("starting API server", zap.String("address", addr))

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
			zap.L().Error("api server crashed", zap.Error(err))
		}
	}()

	return nil
}

func (s *Server) authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authToken == "" {
			next(w, r)
			return
		}

		header := r.Header.Get("Authorization")
		if header == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		if token != s.authToken {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	peers := s.node.Network().Peers()

	resp := map[string]interface{}{
		"status":     "ok",
		"peer_id":    s.node.ID().String(),
		"peer_count": len(peers),
		"uptime_sec": int(time.Since(s.startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetAddresses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	peerInfo := peer.AddrInfo{
		ID:    s.node.ID(),
		Addrs: s.node.Addrs(),
	}

	addrs, err := peer.AddrInfoToP2pAddrs(&peerInfo)
	if err != nil {
		http.Error(w, "Failed to resolve multiaddresses", http.StatusInternalServerError)
		return
	}

	var stringAddrs []string
	for _, addr := range addrs {
		stringAddrs = append(stringAddrs, addr.String())
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stringAddrs); err != nil {
		zap.L().Error("failed to encode api response", zap.Error(err))
	}
}
