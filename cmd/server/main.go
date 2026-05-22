package main

import (
	"log"
	"os"

	"github.com/targc/kunn/internal/server"
)

func main() {
	addr := os.Getenv("KUNN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	var config server.IConfig

	if webhookURL := os.Getenv("KUNN_WEBHOOK_URL"); webhookURL != "" {
		config = &server.WebhookConfig{
			Addr:       addr,
			WebhookURL: webhookURL,
		}
		log.Printf("using webhook config: %s", webhookURL)
	} else {
		configPath := os.Getenv("KUNN_CONFIG")
		if configPath == "" {
			configPath = "/etc/kunn/config.yaml"
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
