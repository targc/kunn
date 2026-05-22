package client

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	return strings.TrimSpace(string(data))
}

// WipeToken removes ~/.kunn/token.
func WipeToken() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	os.Remove(filepath.Join(home, tokenFile))
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

// Login starts a loopback server and walks the user through the
// auth server's /user/auth/?state&callback flow. Returns the token on success.
func Login(ctx context.Context, authURL string) (string, error) {
	state, err := randomState()
	if err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to start callback server: %w", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	callback := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	type result struct {
		token string
		err   error
	}
	resultCh := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		gotState := r.URL.Query().Get("state")
		code := r.URL.Query().Get("code")
		if gotState != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			resultCh <- result{err: errors.New("login: state mismatch")}
			return
		}
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			resultCh <- result{err: errors.New("login: empty code")}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8"><title>Logged in</title></head><body><h2>Logged in.</h2><p id="msg">Attempting to close this window…</p><script>(function(){try{window.open('','_self');window.close();}catch(e){}setTimeout(function(){var m=document.getElementById('msg');if(m){m.textContent='You can close this window.';}},500);})();</script></body></html>`)
		resultCh <- result{token: strings.TrimSpace(code)}
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	defer srv.Close()

	loginURL := fmt.Sprintf("%s?state=%s&callback=%s",
		authURL, url.QueryEscape(state), url.QueryEscape(callback))

	if err := openBrowser(loginURL); err != nil {
		fmt.Printf("\nOpen this URL to login:\n  %s\n", loginURL)
	}
	fmt.Println("\nWaiting for login...")

	select {
	case res := <-resultCh:
		if res.err != nil {
			return "", res.err
		}
		return res.token, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func randomState() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
