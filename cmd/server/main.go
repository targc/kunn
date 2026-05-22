package main

import (
	"context"
	"log"

	"github.com/sethvargo/go-envconfig"
	"github.com/targc/kunn/internal/server"
)

type Config struct {
	Addr       string `env:"KUNN_ADDR,default=:8080"`
	WebhookURL string `env:"KUNN_WEBHOOK_URL"`
	ConfigPath string `env:"KUNN_CONFIG,default=/etc/kunn/config.yaml"`
}

func main() {
	var cfg Config
	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		log.Fatal(err)
	}

	var config server.IConfig

	if cfg.WebhookURL != "" {
		config = &server.WebhookConfig{
			Addr:       cfg.Addr,
			WebhookURL: cfg.WebhookURL,
		}
		log.Printf("using webhook config: %s", cfg.WebhookURL)
	} else {
		yamlCfg, err := server.LoadYAMLConfig(cfg.ConfigPath)
		if err != nil {
			log.Fatal(err)
		}
		if yamlCfg.Addr != "" {
			cfg.Addr = yamlCfg.Addr
		}
		config = yamlCfg
	}

	srv := server.New(cfg.Addr, config)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
