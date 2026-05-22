package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/targc/local-tunn/internal/wsconn"
)

type Agent struct {
	serverURL string
	token     string
}

func New(serverURL, token string) *Agent {
	return &Agent{
		serverURL: serverURL,
		token:     token,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	for {
		err := a.connect(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Error("connection lost, reconnecting", "err", err)
		a.backoff(ctx)
	}
}

func (a *Agent) connect(ctx context.Context) error {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+a.token)

	ws, _, err := websocket.DefaultDialer.DialContext(ctx, a.serverURL, header)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer ws.Close()

	// Agent is yamux.Server (accepts streams from server)
	session, err := yamux.Server(wsconn.New(ws), nil)
	if err != nil {
		return fmt.Errorf("failed to create yamux session: %w", err)
	}
	defer session.Close()

	slog.Info("agent connected to server", "server", a.serverURL)

	for {
		stream, err := session.Accept()
		if err != nil {
			if err != io.EOF {
				slog.Error("stream accept failed", "err", err)
			}
			return fmt.Errorf("session closed")
		}
		go a.handleStream(stream)
	}
}

func (a *Agent) handleStream(stream net.Conn) {
	defer stream.Close()

	// Read target address (first line from server)
	reader := bufio.NewReader(stream)
	address, err := reader.ReadString('\n')
	if err != nil {
		slog.Error("failed to read target address", "err", err)
		return
	}
	address = strings.TrimSpace(address)

	// Dial the target service
	backend, err := net.Dial("tcp", address)
	if err != nil {
		slog.Error("failed to dial service", "address", address, "err", err)
		return
	}
	defer backend.Close()

	slog.Info("stream opened", "address", address)

	// Bidirectional proxy
	done := make(chan struct{})
	go func() {
		io.Copy(backend, reader)
		done <- struct{}{}
	}()
	io.Copy(stream, backend)
	<-done

	slog.Info("stream closed", "address", address)
}

func (a *Agent) backoff(ctx context.Context) {
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
