package server

import (
	"bufio"
	"encoding/json"
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
	addr     string
	config   IConfig
	upgrader websocket.Upgrader
}

func New(addr string, config IConfig) *Server {
	return &Server{
		addr:   addr,
		config: config,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/services", s.handleServices)
	slog.Info("server listening", "addr", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !s.config.ValidToken(token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	services, err := s.config.ClientServices(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(services)
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

	reader := bufio.NewReader(stream)
	service, err := reader.ReadString('\n')
	if err != nil {
		slog.Error("failed to read service name", "name", name, "err", err)
		return
	}
	service = strings.TrimSpace(service)

	address, err := s.config.ResolveService(token, service)
	if err != nil {
		slog.Warn("service not allowed", "name", name, "service", service, "err", err)
		return
	}

	backend, err := net.Dial("tcp", address)
	if err != nil {
		slog.Error("failed to dial service", "name", name, "service", service, "address", address, "err", err)
		return
	}
	defer backend.Close()

	slog.Info("stream opened", "name", name, "service", service, "address", address)

	done := make(chan struct{})
	go func() {
		io.Copy(backend, reader)
		done <- struct{}{}
	}()
	io.Copy(stream, backend)
	<-done

	slog.Info("stream closed", "name", name, "service", service)
}
