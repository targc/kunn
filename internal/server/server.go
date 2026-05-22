package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/targc/kunn/internal/wsconn"
)

type Server struct {
	addr     string
	config   IConfig
	upgrader websocket.Upgrader

	mu     sync.RWMutex
	agents map[string]*yamux.Session // clusterID → agent yamux session
}

func New(addr string, config IConfig) *Server {
	return &Server{
		addr:   addr,
		config: config,
		agents: make(map[string]*yamux.Session),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/client", s.handleClientWS)
	mux.HandleFunc("/ws/agent", s.handleAgentWS)
	mux.HandleFunc("/projects", s.handleProjects)
	mux.HandleFunc("/services", s.handleServices)
	slog.Info("server listening", "addr", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !s.config.ValidToken(token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	projects, err := s.config.ClientProjects(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !s.config.ValidToken(token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	projectID := r.URL.Query().Get("project")
	if projectID == "" {
		http.Error(w, "project query param required", http.StatusBadRequest)
		return
	}
	services, err := s.config.ClientServices(token, projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(services)
}

// --- Agent WebSocket ---

func (s *Server) handleAgentWS(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	clusterID, ok := s.config.ValidAgentToken(token)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("agent ws upgrade failed", "err", err)
		return
	}
	defer ws.Close()

	// Server is yamux.Client toward agent (server opens streams to agent)
	session, err := yamux.Client(wsconn.New(ws), nil)
	if err != nil {
		slog.Error("agent yamux failed", "err", err)
		return
	}
	defer session.Close()

	// Replace existing agent for this cluster
	s.mu.Lock()
	if old, exists := s.agents[clusterID]; exists {
		old.Close()
	}
	s.agents[clusterID] = session
	s.mu.Unlock()

	slog.Info("agent connected", "cluster", clusterID)

	// Block until session closes
	<-session.CloseChan()

	s.mu.Lock()
	if s.agents[clusterID] == session {
		delete(s.agents, clusterID)
	}
	s.mu.Unlock()

	slog.Info("agent disconnected", "cluster", clusterID)
}

func (s *Server) getAgent(clusterID string) *yamux.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agents[clusterID]
}

// --- Client WebSocket ---

func (s *Server) handleClientWS(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !s.config.ValidToken(token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("client ws upgrade failed", "err", err)
		return
	}
	defer ws.Close()

	name := s.config.ClientName(token)
	slog.Info("client connected", "name", name)

	// Server is yamux.Server for clients (clients open streams to server)
	session, err := yamux.Server(wsconn.New(ws), nil)
	if err != nil {
		slog.Error("client yamux failed", "err", err)
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

func (s *Server) handleStream(clientStream net.Conn, token, name string) {
	defer clientStream.Close()

	// Read projectID/serviceID from client
	reader := bufio.NewReader(clientStream)
	line, err := reader.ReadString('\n')
	if err != nil {
		slog.Error("failed to read stream header", "name", name, "err", err)
		return
	}
	line = strings.TrimSpace(line)

	parts := strings.SplitN(line, "/", 2)
	if len(parts) != 2 {
		slog.Warn("invalid stream header", "name", name, "line", line)
		return
	}
	projectID, serviceID := parts[0], parts[1]

	// Resolve to cluster + address
	route, err := s.config.ResolveService(token, projectID, serviceID)
	if err != nil {
		slog.Warn("service not allowed", "name", name, "project", projectID, "service", serviceID, "err", err)
		return
	}

	// Get agent for this cluster
	agent := s.getAgent(route.Cluster)
	if agent == nil {
		slog.Warn("agent offline", "name", name, "cluster", route.Cluster, "service", serviceID)
		return
	}

	// Open stream to agent
	agentStream, err := agent.Open()
	if err != nil {
		slog.Error("failed to open agent stream", "name", name, "cluster", route.Cluster, "err", err)
		return
	}
	defer agentStream.Close()

	// Send target address to agent
	if _, err := fmt.Fprintf(agentStream, "%s\n", route.Address); err != nil {
		slog.Error("failed to send address to agent", "err", err)
		return
	}

	slog.Info("stream opened", "name", name, "project", projectID, "service", serviceID, "cluster", route.Cluster)

	// Bridge client stream ↔ agent stream
	done := make(chan struct{})
	go func() {
		io.Copy(agentStream, reader)
		done <- struct{}{}
	}()
	io.Copy(clientStream, agentStream)
	<-done

	slog.Info("stream closed", "name", name, "project", projectID, "service", serviceID)
}
