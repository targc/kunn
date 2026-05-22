package server

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/targc/local-tunn/internal/wsconn"
)

type Server struct {
	config   *Config
	upgrader websocket.Upgrader
}

func New(config *Config) *Server {
	return &Server{
		config: config,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)
	slog.Info("server listening", "addr", s.config.Addr)
	return http.ListenAndServe(s.config.Addr, mux)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !s.config.ValidToken(token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade failed", "err", err)
		return
	}
	defer ws.Close()

	name := s.config.ClientName(token)
	slog.Info("client connected", "name", name)

	session, err := yamux.Server(wsconn.New(ws), nil)
	if err != nil {
		slog.Error("yamux server failed", "err", err)
		return
	}
	defer session.Close()

	for {
		stream, err := session.Accept()
		if err != nil {
			if err != io.EOF {
				slog.Error("stream accept failed", "name", name, "err", err)
			}
			slog.Info("client disconnected", "name", name)
			return
		}
		go s.handleStream(stream, token, name)
	}
}

func (s *Server) handleStream(stream net.Conn, token, name string) {
	defer stream.Close()

	// Read service name (first line)
	reader := bufio.NewReader(stream)
	service, err := reader.ReadString('\n')
	if err != nil {
		slog.Error("failed to read service name", "name", name, "err", err)
		return
	}
	service = strings.TrimSpace(service)

	if !s.config.ServiceAllowed(token, service) {
		slog.Warn("service not allowed", "name", name, "service", service)
		return
	}

	// Dial the k8s service
	backend, err := net.Dial("tcp", service)
	if err != nil {
		slog.Error("failed to dial service", "name", name, "service", service, "err", err)
		return
	}
	defer backend.Close()

	slog.Info("stream opened", "name", name, "service", service)

	// Bidirectional proxy
	done := make(chan struct{})
	go func() {
		io.Copy(backend, reader)
		done <- struct{}{}
	}()
	io.Copy(stream, backend)
	<-done

	slog.Info("stream closed", "name", name, "service", service)
}
