package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/huh"
	"github.com/sethvargo/go-envconfig"
	"github.com/targc/kunn/internal/client"
)

type Config struct {
	Server  string `env:"KUNN_SERVER"`
	Token   string `env:"KUNN_TOKEN"`
	AuthURL string `env:"KUNN_AUTH_URL"`
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "logout":
			client.WipeToken()
			fmt.Println("Logged out.")
			return
		case "--help", "-h":
			fmt.Print(`kunn - Kubernetes tunnel client

Usage:
  kunn              Start tunnel (interactive)
  kunn logout       Remove saved token
  kunn --help       Show this help

Environment:
  KUNN_SERVER       WebSocket server URL (required)
  KUNN_TOKEN        Auth token (optional, overrides saved token)
  KUNN_AUTH_URL     Login page URL for browser auth (optional)
`)
			return
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatal(err)
	}
	if cfg.Server == "" {
		log.Fatal("KUNN_SERVER is required")
	}

	token := cfg.Token
	if token == "" {
		token = client.LoadToken()
	}
	if token == "" {
		token = login(ctx, cfg.AuthURL)
	}

	// Select project
	projects, err := client.FetchProjects(cfg.Server, token)
	if errors.Is(err, client.ErrUnauthorized) {
		client.WipeToken()
		fmt.Println("Token expired or invalid, re-authenticating...")
		token = login(ctx, cfg.AuthURL)
		projects, err = client.FetchProjects(cfg.Server, token)
	}
	if err != nil {
		log.Fatalf("failed to fetch projects: %v", err)
	}
	if len(projects) == 0 {
		log.Fatal("no projects available for this token")
	}

	projOptions := make([]huh.Option[int], len(projects))
	for i, p := range projects {
		projOptions[i] = huh.NewOption(fmt.Sprintf("%s (%s)", p.Name, p.ID), i)
	}

	var projIdx int
	err = huh.NewSelect[int]().
		Title("Select project").
		Options(projOptions...).
		Value(&projIdx).
		Run()
	if err != nil {
		log.Fatal(err)
	}
	selectedProject := projects[projIdx]

	// Select service
	services, err := client.FetchServices(cfg.Server, token, selectedProject.ID)
	if err != nil {
		log.Fatalf("failed to fetch services: %v", err)
	}
	if len(services) == 0 {
		log.Fatal("no services available in this project")
	}

	svcOptions := make([]huh.Option[int], len(services))
	for i, s := range services {
		svcOptions[i] = huh.NewOption(fmt.Sprintf("%s (%s)", s.Name, s.ID), i)
	}

	var svcIdx int
	err = huh.NewSelect[int]().
		Title("Select service").
		Options(svcOptions...).
		Value(&svcIdx).
		Run()
	if err != nil {
		log.Fatal(err)
	}
	selectedService := services[svcIdx]

	port := findAvailablePort(6060)
	fmt.Printf("Tunneling %s → %s on localhost:%d\n", selectedProject.Name, selectedService.Name, port)

	forwards := []client.Forward{{
		LocalPort: port,
		ProjectID: selectedProject.ID,
		ServiceID: selectedService.ID,
	}}

	c := client.New(cfg.Server, token, forwards)
	if err := c.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
	fmt.Println("\nDisconnected.")
}

func login(ctx context.Context, authURL string) string {
	if authURL == "" {
		log.Fatal("no token found. Set KUNN_TOKEN, or set KUNN_AUTH_URL to login via browser")
	}
	token, err := client.Login(ctx, authURL)
	if err != nil {
		log.Fatalf("login failed: %v", err)
	}
	if err := client.SaveToken(token); err != nil {
		log.Printf("warning: failed to save token: %v", err)
	}
	fmt.Println("Login successful!")
	return token
}

func findAvailablePort(start int) int {
	for port := start; port < start+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	log.Fatal("no available port found")
	return 0
}
