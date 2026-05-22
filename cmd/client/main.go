package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
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

	services, err := client.FetchServices(serverURL, token)
	if err != nil {
		log.Fatalf("failed to fetch services: %v", err)
	}

	if len(services) == 0 {
		log.Fatal("no services available for this token")
	}

	fmt.Println("Available services:")
	for i, s := range services {
		fmt.Printf("  %d) %s (%s)\n", i+1, s.Name, s.ID)
	}
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Select service: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(services) {
		log.Fatalf("invalid selection: %s", input)
	}
	selected := services[idx-1]

	fmt.Printf("Local port for '%s': ", selected.Name)
	portStr, _ := reader.ReadString('\n')
	portStr = strings.TrimSpace(portStr)
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("invalid port: %s", portStr)
	}

	forwards := []client.Forward{{LocalPort: port, ServiceID: selected.ID}}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	c := client.New(serverURL, token, forwards)
	if err := c.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
