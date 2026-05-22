package client

import (
	"context"
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

	// Wait for yamux session to close
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

	slog.Info("listening", "local", addr, "remote", fwd.ServiceID)

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
		slog.Error("failed to open stream", "remote", fwd.ServiceID, "err", err)
		return
	}
	defer stream.Close()

	// Send service name
	if _, err := fmt.Fprintf(stream, "%s\n", fwd.ServiceID); err != nil {
		slog.Error("failed to send service name", "err", err)
		return
	}

	// Bidirectional proxy
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
		// Try to connect, if fails continue backoff
		return
	}
}

func ParseForwards(s string) ([]Forward, error) {
	var forwards []Forward
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// format: localPort:serviceID
		parts := strings.SplitN(part, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid forward: %s (expected localPort:serviceID)", part)
		}
		var localPort int
		if _, err := fmt.Sscanf(parts[0], "%d", &localPort); err != nil {
			return nil, fmt.Errorf("invalid local port: %s", parts[0])
		}
		forwards = append(forwards, Forward{LocalPort: localPort, ServiceID: parts[1]})
	}
	return forwards, nil
}
