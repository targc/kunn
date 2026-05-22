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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	token := client.LoadToken()
	if token == "" {
		authURL := os.Getenv("TUNN_AUTH_URL")
		if authURL == "" {
			log.Fatal("no token found. Set TUNN_TOKEN, or set TUNN_AUTH_URL to login via browser")
		}
		var err error
		token, err = client.Login(ctx, authURL)
		if err != nil {
			log.Fatalf("login failed: %v", err)
		}
		// Validate token before saving
		if _, err := client.FetchProjects(serverURL, token); err != nil {
			log.Fatalf("login failed: invalid token received")
		}
		if err := client.SaveToken(token); err != nil {
			log.Printf("warning: failed to save token: %v", err)
		}
		fmt.Println("Login successful!")
	}

	// Select project
	projects, err := client.FetchProjects(serverURL, token)
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
	services, err := client.FetchServices(serverURL, token, selectedProject.ID)
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

	c := client.New(serverURL, token, forwards)
	if err := c.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
	fmt.Println("\nDisconnected.")
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
