package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultIdleTimeout  = 60 * time.Second
)

type Server struct {
	node      host.Host
	port      int
	authToken string
	startTime time.Time

	httpServer *http.Server
	listener   net.Listener
	errCh      chan error
}

func NewServer(node host.Host, port int, authToken string) *Server {
	return &Server{
		node:      node,
		port:      port,
		authToken: authToken,
		startTime: time.Now(),
	}
}

// Start binds the TCP listener and runs the HTTP server in a background
// goroutine. It returns synchronously once the listener is bound, so callers
// know the port is open before they continue. Use Stop for graceful shutdown.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/addresses", s.authenticate(s.handleGetAddresses))
	mux.HandleFunc("/api/v1/health", s.handleHealth)

	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("api listen on %s: %w", addr, err)
	}
	s.listener = listener

	s.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: defaultReadTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}
	s.errCh = make(chan error, 1)

	zap.L().Info("starting API server", zap.String("address", listener.Addr().String()))

	go func() {
		err := s.httpServer.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			zap.L().Error("api server crashed", zap.Error(err))
			s.errCh <- err
		}
		close(s.errCh)
	}()

	return nil
}

// Stop gracefully shuts the HTTP server down, draining in-flight handlers
// up to the given context's deadline. Safe to call multiple times.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// Err returns a channel that emits the server's terminal error (if any) and
// is closed when the serve goroutine exits. Useful for `select` in the main
// loop alongside ctx.Done().
func (s *Server) Err() <-chan error {
	return s.errCh
}

func (s *Server) authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authToken == "" {
			next(w, r)
			return
		}

		header := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(s.authToken)) != 1 {
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

	resp := map[string]any{
		"status":     "ok",
		"peer_id":    s.node.ID().String(),
		"peer_count": len(peers),
		"uptime_sec": int(time.Since(s.startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		zap.L().Error("failed to encode health response", zap.Error(err))
	}
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

	stringAddrs := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		stringAddrs = append(stringAddrs, addr.String())
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stringAddrs); err != nil {
		zap.L().Error("failed to encode api response", zap.Error(err))
	}
}
