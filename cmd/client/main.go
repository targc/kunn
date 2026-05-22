package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/huh"
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

	services, err := client.FetchServices(serverURL, token)
	if err != nil {
		log.Fatalf("failed to fetch services: %v", err)
	}

	if len(services) == 0 {
		log.Fatal("no services available for this token")
	}

	options := make([]huh.Option[int], len(services))
	for i, s := range services {
		options[i] = huh.NewOption(fmt.Sprintf("%s (%s)", s.Name, s.ID), i)
	}

	var selectedIdx int
	err = huh.NewSelect[int]().
		Title("Select service").
		Options(options...).
		Value(&selectedIdx).
		Run()
	if err != nil {
		log.Fatal(err)
	}

	selected := services[selectedIdx]
	port := findAvailablePort(6060)

	fmt.Printf("Tunneling %s on localhost:%d\n", selected.Name, port)

	forwards := []client.Forward{{LocalPort: port, ServiceID: selected.ID}}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	c := client.New(serverURL, token, forwards)
	if err := c.Run(ctx); err != nil {
		log.Fatal(err)
	}
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
