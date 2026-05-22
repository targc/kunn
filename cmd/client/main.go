package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/targc/local-tunn/internal/client"
)

func main() {
	serverURL := os.Getenv("TUNN_SERVER")
	if serverURL == "" {
		log.Fatal("TUNN_SERVER is required")
	}

	token := os.Getenv("TUNN_TOKEN")
	if token == "" {
		log.Fatal("TUNN_TOKEN is required")
	}

	forwardsStr := os.Getenv("TUNN_FORWARDS")
	if forwardsStr == "" {
		log.Fatal("TUNN_FORWARDS is required")
	}

	forwards, err := client.ParseForwards(forwardsStr)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	c := client.New(serverURL, token, forwards)
	if err := c.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
