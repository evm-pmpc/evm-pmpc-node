package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

type Server struct {
	node host.Host
	port int
}

func NewServer(node host.Host, port int) *Server {
	return &Server{
		node: node,
		port: port,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/addresses", s.handleGetAddresses)

	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	zap.L().Info("starting API server", zap.String("address", addr))

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
			zap.L().Error("api server crashed", zap.Error(err))
		}
	}()

	return nil
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
