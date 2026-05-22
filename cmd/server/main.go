package main

import (
	"log"
	"os"

	"github.com/targc/local-tunn/internal/server"
)

func main() {
	addr := os.Getenv("TUNN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	var config server.IConfig

	if webhookURL := os.Getenv("TUNN_WEBHOOK_URL"); webhookURL != "" {
		config = &server.WebhookConfig{
			Addr:       addr,
			WebhookURL: webhookURL,
		}
		log.Printf("using webhook config: %s", webhookURL)
	} else {
		configPath := os.Getenv("TUNN_CONFIG")
		if configPath == "" {
			configPath = "/etc/tunn/config.yaml"
		}
		cfg, err := server.LoadYAMLConfig(configPath)
		if err != nil {
			log.Fatal(err)
		}
		if cfg.Addr != "" {
			addr = cfg.Addr
		}
		config = cfg
	}

	srv := server.New(addr, config)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
