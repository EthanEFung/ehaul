package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"

	"golang.org/x/oauth2"

	"github.com/ethanefung/ehaul/internal/provider"
)

// BrowserFlow runs the OAuth 2.0 installed-app loopback flow: it starts a
// one-shot HTTP server on 127.0.0.1, opens the user's browser to the
// provider's consent URL, and exchanges the returned authorization code for a
// token. CSRF state and PKCE (S256) are included.
func BrowserFlow(ctx context.Context, prov *provider.Provider) (*oauth2.Token, error) {
	id, secret := MustLoad()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start loopback listener: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	cfg := &oauth2.Config{
		ClientID:     id,
		ClientSecret: secret,
		Scopes:       prov.OAuthScopes,
		Endpoint:     prov.Endpoint,
		RedirectURL:  redirectURL,
	}

	state, err := randomState()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}
	verifier := oauth2.GenerateVerifier()

	authURL := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
		oauth2.S256ChallengeOption(verifier),
	)

	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)

	var once sync.Once
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() {
			q := r.URL.Query()
			if got := q.Get("state"); got != state {
				http.Error(w, "state mismatch", http.StatusBadRequest)
				done <- result{err: fmt.Errorf("oauth: state mismatch")}
				return
			}
			if errParam := q.Get("error"); errParam != "" {
				http.Error(w, "authorization denied", http.StatusBadRequest)
				done <- result{err: fmt.Errorf("oauth: %s", errParam)}
				return
			}
			code := q.Get("code")
			if code == "" {
				http.Error(w, "missing code", http.StatusBadRequest)
				done <- result{err: fmt.Errorf("oauth: missing code in callback")}
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, successHTML)
			done <- result{code: code}
		})
	})

	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Open this URL in your browser to authenticate:\n%s\n", authURL)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-done:
		if res.err != nil {
			return nil, res.err
		}
		tok, err := cfg.Exchange(ctx, res.code, oauth2.VerifierOption(verifier))
		if err != nil {
			return nil, fmt.Errorf("exchange code: %w", err)
		}
		return tok, nil
	}
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

const successHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>ehaul</title></head>
<body style="font-family:-apple-system,system-ui,sans-serif;text-align:center;padding:4em;">
<h2>Authenticated.</h2>
<p>You can close this tab.</p>
<script>window.close();</script>
</body></html>`
