package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/targc/local-tunn/internal/agent"
)

func main() {
	serverURL := os.Getenv("TUNN_SERVER")
	if serverURL == "" {
		log.Fatal("TUNN_SERVER is required")
	}

	token := os.Getenv("TUNN_AGENT_TOKEN")
	if token == "" {
		log.Fatal("TUNN_AGENT_TOKEN is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	a := agent.New(serverURL, token)
	if err := a.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
