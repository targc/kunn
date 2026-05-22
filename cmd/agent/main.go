package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/sethvargo/go-envconfig"
	"github.com/targc/kunn/internal/agent"
)

type Config struct {
	Server     string `env:"KUNN_SERVER,required"`
	AgentToken string `env:"KUNN_AGENT_TOKEN,required"`
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatal(err)
	}

	a := agent.New(cfg.Server, cfg.AgentToken)
	if err := a.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
