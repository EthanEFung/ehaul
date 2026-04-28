// Package auth handles OAuth2 authentication against email providers and
// persists the resulting tokens in the OS keyring.
package auth

import (
	"fmt"
	"os"
)

// clientID and clientSecret are populated at build time via
// `go build -ldflags "-X github.com/ethanefung/ehaul/internal/auth.clientID=... -X github.com/ethanefung/ehaul/internal/auth.clientSecret=..."`.
// They remain empty when built without the Makefile.
var (
	clientID     string
	clientSecret string
)

// MustLoad returns the baked-in OAuth client credentials, or prints a clear
// error and exits the process with status 1 if they are missing. It is
// intended to be called once at program startup so that a misbuilt binary
// fails immediately rather than part-way through a user flow.
func MustLoad() (id, secret string) {
	if clientID == "" || clientSecret == "" {
		fmt.Fprintln(os.Stderr, "ehaul: OAuth client credentials are not set.")
		fmt.Fprintln(os.Stderr, "      Populate .credentials with GMAIL_CLIENT_ID and")
		fmt.Fprintln(os.Stderr, "      GMAIL_CLIENT_SECRET, then re-run `make build`.")
		os.Exit(1)
	}
	return clientID, clientSecret
}
