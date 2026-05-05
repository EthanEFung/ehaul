package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeConfigFile creates the ehaul credentials file inside a temporary HOME
// directory and returns the HOME path. On Darwin, os.UserConfigDir resolves to
// $HOME/Library/Application Support; on other Unix systems it honours
// $XDG_CONFIG_HOME (or falls back to $HOME/.config).
func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()

	var cfgDir string
	switch runtime.GOOS {
	case "darwin":
		cfgDir = filepath.Join(home, "Library", "Application Support", "ehaul")
	default:
		cfgDir = filepath.Join(home, ".config", "ehaul")
	}
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "credentials"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

// setHomeEnv sets the environment variable(s) that os.UserConfigDir reads so
// that the config file path resolves under the given home directory.
func setHomeEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	// On non-Darwin Unix, also clear XDG_CONFIG_HOME so the default
	// $HOME/.config is used.
	if runtime.GOOS != "darwin" {
		t.Setenv("XDG_CONFIG_HOME", "")
	}
}

func TestLoad_EnvVars(t *testing.T) {
	t.Setenv("GMAIL_CLIENT_ID", "env-id")
	t.Setenv("GMAIL_CLIENT_SECRET", "env-secret")

	id, secret, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "env-id" {
		t.Errorf("id = %q, want %q", id, "env-id")
	}
	if secret != "env-secret" {
		t.Errorf("secret = %q, want %q", secret, "env-secret")
	}
}

func TestLoad_EnvVarsPartial(t *testing.T) {
	// Only one env var set — should fall through to config file.
	// With no config file either, expect an error.
	t.Setenv("GMAIL_CLIENT_ID", "env-id")
	t.Setenv("GMAIL_CLIENT_SECRET", "")

	home := t.TempDir()
	setHomeEnv(t, home)

	_, _, err := Load()
	if err == nil {
		t.Fatal("expected error when only one env var is set and no config file")
	}
}

func TestLoad_ConfigFile(t *testing.T) {
	t.Setenv("GMAIL_CLIENT_ID", "")
	t.Setenv("GMAIL_CLIENT_SECRET", "")

	home := writeConfigFile(t, "GMAIL_CLIENT_ID=file-id\nGMAIL_CLIENT_SECRET=file-secret\n")
	setHomeEnv(t, home)

	id, secret, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "file-id" {
		t.Errorf("id = %q, want %q", id, "file-id")
	}
	if secret != "file-secret" {
		t.Errorf("secret = %q, want %q", secret, "file-secret")
	}
}

func TestLoad_ConfigFileMissing(t *testing.T) {
	t.Setenv("GMAIL_CLIENT_ID", "")
	t.Setenv("GMAIL_CLIENT_SECRET", "")

	home := t.TempDir()
	setHomeEnv(t, home)

	_, _, err := Load()
	if err == nil {
		t.Fatal("expected error when config file is missing")
	}
}

func TestLoad_ConfigFileIncompleteValues(t *testing.T) {
	t.Setenv("GMAIL_CLIENT_ID", "")
	t.Setenv("GMAIL_CLIENT_SECRET", "")

	// Only client ID present, secret missing.
	home := writeConfigFile(t, "GMAIL_CLIENT_ID=file-id\n")
	setHomeEnv(t, home)

	_, _, err := Load()
	if err == nil {
		t.Fatal("expected error when credentials are incomplete")
	}
}

func TestLoad_ConfigFileCommentsAndBlanks(t *testing.T) {
	t.Setenv("GMAIL_CLIENT_ID", "")
	t.Setenv("GMAIL_CLIENT_SECRET", "")

	content := `# This is a comment
GMAIL_CLIENT_ID=commented-id

# Another comment

GMAIL_CLIENT_SECRET=commented-secret
`
	home := writeConfigFile(t, content)
	setHomeEnv(t, home)

	id, secret, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "commented-id" {
		t.Errorf("id = %q, want %q", id, "commented-id")
	}
	if secret != "commented-secret" {
		t.Errorf("secret = %q, want %q", secret, "commented-secret")
	}
}

func TestLoad_EnvVarsOverrideConfigFile(t *testing.T) {
	t.Setenv("GMAIL_CLIENT_ID", "env-id")
	t.Setenv("GMAIL_CLIENT_SECRET", "env-secret")

	home := writeConfigFile(t, "GMAIL_CLIENT_ID=file-id\nGMAIL_CLIENT_SECRET=file-secret\n")
	setHomeEnv(t, home)

	id, secret, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "env-id" {
		t.Errorf("id = %q, want %q (env should override file)", id, "env-id")
	}
	if secret != "env-secret" {
		t.Errorf("secret = %q, want %q (env should override file)", secret, "env-secret")
	}
}

func TestLoad_ConfigFileWhitespace(t *testing.T) {
	t.Setenv("GMAIL_CLIENT_ID", "")
	t.Setenv("GMAIL_CLIENT_SECRET", "")

	content := "  GMAIL_CLIENT_ID  =  spaced-id  \n  GMAIL_CLIENT_SECRET  =  spaced-secret  \n"
	home := writeConfigFile(t, content)
	setHomeEnv(t, home)

	id, secret, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "spaced-id" {
		t.Errorf("id = %q, want %q", id, "spaced-id")
	}
	if secret != "spaced-secret" {
		t.Errorf("secret = %q, want %q", secret, "spaced-secret")
	}
}
