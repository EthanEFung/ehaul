// Package auth handles OAuth2 authentication against email providers and
// persists the resulting tokens in the OS keyring.
package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// configPath returns the platform-appropriate path to the ehaul credentials
// file. It delegates to os.UserConfigDir so the path follows each OS's
// convention (macOS: ~/Library/Application Support, Linux: ~/.config or
// $XDG_CONFIG_HOME, Windows: %AppData%).
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ehaul", "credentials"), nil
}

// parseCredentialsFile reads a KEY=VALUE credentials file at path and returns
// the GMAIL_CLIENT_ID and GMAIL_CLIENT_SECRET values. Blank lines and lines
// starting with '#' are skipped. Unknown keys are silently ignored.
func parseCredentialsFile(path string) (id, secret string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "GMAIL_CLIENT_ID":
			id = strings.TrimSpace(val)
		case "GMAIL_CLIENT_SECRET":
			secret = strings.TrimSpace(val)
		}
	}
	return id, secret, nil
}

// Load returns the OAuth client ID and secret. It checks environment variables
// first (GMAIL_CLIENT_ID, GMAIL_CLIENT_SECRET); if both are set and non-empty
// they are returned immediately without reading the config file. Otherwise it
// reads the config file at os.UserConfigDir()/ehaul/credentials. An error is
// returned if neither source provides both values.
func Load() (id, secret string, err error) {
	// 1. Environment variables take priority.
	id = os.Getenv("GMAIL_CLIENT_ID")
	secret = os.Getenv("GMAIL_CLIENT_SECRET")
	if id != "" && secret != "" {
		return id, secret, nil
	}

	// 2. Config file.
	path, err := configPath()
	if err != nil {
		return "", "", fmt.Errorf("resolve config dir: %w", err)
	}
	id, secret, err = parseCredentialsFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", path, err)
	}
	if id == "" || secret == "" {
		return "", "", fmt.Errorf("credentials incomplete in %s", path)
	}
	return id, secret, nil
}

const mustLoadErrFmt = `ehaul: OAuth client credentials not found.

Create a credentials file at:
  %s

With contents:
  GMAIL_CLIENT_ID=your-client-id
  GMAIL_CLIENT_SECRET=your-client-secret

See the project README for setup instructions.
`

// MustLoad returns the OAuth client credentials, or prints a descriptive
// error with setup instructions and exits the process with status 1. It is
// intended to be called once at program startup so that missing credentials
// are caught immediately rather than part-way through a user flow.
func MustLoad() (id, secret string) {
	id, secret, err := Load()
	if err != nil {
		path, pathErr := configPath()
		if pathErr != nil {
			path = "(unable to resolve config directory)"
		}
		fmt.Fprintf(os.Stderr, mustLoadErrFmt, path)
		os.Exit(1)
	}
	return id, secret
}
