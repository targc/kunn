package main

import (
	"log"
	"os"

	"github.com/targc/local-tunn/internal/server"
)

func main() {
	configPath := os.Getenv("TUNN_CONFIG")
	if configPath == "" {
		configPath = "/etc/tunn/config.yaml"
	}

	cfg, err := server.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	if addr := os.Getenv("TUNN_ADDR"); addr != "" {
		cfg.Addr = addr
	}

	srv := server.New(cfg)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
