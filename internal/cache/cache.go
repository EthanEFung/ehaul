// Package cache persists IMAP metadata (e.g., UIDVALIDITY) to the
// filesystem so future commands can detect stale UIDs.
package cache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Dir returns the platform-appropriate cache directory for this application.
// On macOS: ~/Library/Caches/mail. On Linux: ~/.cache/mail (or $XDG_CACHE_HOME/mail).
func Dir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cache dir: %w", err)
	}
	return filepath.Join(base, "mail"), nil
}

// SaveUIDValidity writes the UIDVALIDITY value to <cacheDir>/<email>/<mailbox>.
// It creates intermediate directories as needed (0700) and writes with 0600
// permissions. The file content is the decimal string representation followed
// by a newline.
func SaveUIDValidity(cacheDir, email, mailbox string, val uint32) error {
	dir := filepath.Join(cacheDir, email)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("save uidvalidity: mkdir: %w", err)
	}
	data := []byte(strconv.FormatUint(uint64(val), 10) + "\n")
	path := filepath.Join(dir, mailbox)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("save uidvalidity: write: %w", err)
	}
	return nil
}

// LoadUIDValidity reads a previously cached UIDVALIDITY from
// <cacheDir>/<email>/<mailbox>. It returns (0, false, nil) when the file does
// not exist (first run). Any other I/O or parse error is returned as err.
func LoadUIDValidity(cacheDir, email, mailbox string) (val uint32, ok bool, err error) {
	path := filepath.Join(cacheDir, email, mailbox)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("load uidvalidity: read: %w", err)
	}
	s := strings.TrimSpace(string(data))
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, false, fmt.Errorf("load uidvalidity: parse %q: %w", s, err)
	}
	return uint32(n), true, nil
}
