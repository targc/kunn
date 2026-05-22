package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/targc/local-tunn/internal/wsconn"
)

type Forward struct {
	LocalPort int
	ProjectID string
	ServiceID string
}

type Client struct {
	serverURL string
	token     string
	forwards  []Forward
}

func New(serverURL, token string, forwards []Forward) *Client {
	return &Client{
		serverURL: serverURL,
		token:     token,
		forwards:  forwards,
	}
}

type ProjectInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ServiceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func httpBaseURL(serverURL string) string {
	u := strings.Replace(serverURL, "/ws", "", 1)
	u = strings.Replace(u, "ws://", "http://", 1)
	u = strings.Replace(u, "wss://", "https://", 1)
	return u
}

// FetchProjects calls GET /projects on the server.
func FetchProjects(serverURL, token string) ([]ProjectInfo, error) {
	httpURL := httpBaseURL(serverURL) + "/projects"

	req, err := http.NewRequest("GET", httpURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch projects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid token")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var projects []ProjectInfo
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, fmt.Errorf("failed to decode projects: %w", err)
	}
	return projects, nil
}

// FetchServices calls GET /services?project=<id> on the server.
func FetchServices(serverURL, token, projectID string) ([]ServiceInfo, error) {
	httpURL := httpBaseURL(serverURL) + "/services?project=" + projectID

	req, err := http.NewRequest("GET", httpURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch services: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid token")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var services []ServiceInfo
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return nil, fmt.Errorf("failed to decode services: %w", err)
	}
	return services, nil
}

func (c *Client) Run(ctx context.Context) error {
	for {
		err := c.connect(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Error("connection lost, reconnecting", "err", err)
		c.backoff(ctx)
	}
}

func (c *Client) connect(ctx context.Context) error {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.token)

	ws, _, err := websocket.DefaultDialer.DialContext(ctx, c.serverURL, header)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer ws.Close()

	session, err := yamux.Client(wsconn.New(ws), nil)
	if err != nil {
		return fmt.Errorf("failed to create yamux session: %w", err)
	}
	defer session.Close()

	slog.Info("tunnel established", "server", c.serverURL)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, fwd := range c.forwards {
		wg.Add(1)
		go func(f Forward) {
			defer wg.Done()
			c.listen(ctx, session, f)
		}(fwd)
	}

	<-session.CloseChan()
	cancel()
	wg.Wait()
	return fmt.Errorf("session closed")
}

func (c *Client) listen(ctx context.Context, session *yamux.Session, fwd Forward) {
	addr := fmt.Sprintf("0.0.0.0:%d", fwd.LocalPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to listen", "addr", addr, "err", err)
		return
	}
	defer ln.Close()

	slog.Info("listening", "local", addr, "service", fwd.ServiceID)

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("accept failed", "err", err)
			continue
		}
		go c.handleConn(conn, session, fwd)
	}
}

func (c *Client) handleConn(conn net.Conn, session *yamux.Session, fwd Forward) {
	defer conn.Close()

	stream, err := session.Open()
	if err != nil {
		slog.Error("failed to open stream", "service", fwd.ServiceID, "err", err)
		return
	}
	defer stream.Close()

	// Send projectID/serviceID
	if _, err := fmt.Fprintf(stream, "%s/%s\n", fwd.ProjectID, fwd.ServiceID); err != nil {
		slog.Error("failed to send stream header", "err", err)
		return
	}

	done := make(chan struct{})
	go func() {
		io.Copy(stream, conn)
		done <- struct{}{}
	}()
	io.Copy(conn, stream)
	<-done
}

func (c *Client) backoff(ctx context.Context) {
	delays := []time.Duration{1, 2, 4, 8, 16, 30}
	for _, d := range delays {
		select {
		case <-ctx.Done():
			return
		case <-time.After(d * time.Second):
		}
		return
	}
}
