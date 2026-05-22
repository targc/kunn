package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

const tokenFile = ".kunn/token"

// LoadToken checks KUNN_TOKEN env var, then ~/.kunn/token file.
func LoadToken() string {
	if t := os.Getenv("KUNN_TOKEN"); t != "" {
		return t
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, tokenFile))
	if err != nil {
		return ""
	}
	return string(data)
}

// SaveToken writes token to ~/.kunn/token.
func SaveToken(token string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}
	dir := filepath.Join(home, ".kunn")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "token"), []byte(token), 0600)
}

// Login starts a localhost callback server, prints the login URL,
// and waits for the auth provider to redirect back with a token.
func Login(ctx context.Context, authURL string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to start callback server: %w", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	tokenCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>Login successful!</h2><p>You can close this tab.</p></body></html>")
		tokenCh <- token
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer srv.Shutdown(context.Background())

	loginURL := fmt.Sprintf("%s?port=%d", authURL, port)
	fmt.Printf("\nOpen this URL to login:\n  %s\n\nWaiting for login...\n", loginURL)

	select {
	case token := <-tokenCh:
		return token, nil
	case err := <-errCh:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
