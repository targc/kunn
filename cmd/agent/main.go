package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/targc/kunn/internal/agent"
)

func main() {
	serverURL := os.Getenv("KUNN_SERVER")
	if serverURL == "" {
		log.Fatal("KUNN_SERVER is required")
	}

	token := os.Getenv("KUNN_AGENT_TOKEN")
	if token == "" {
		log.Fatal("KUNN_AGENT_TOKEN is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	a := agent.New(serverURL, token)
	if err := a.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
