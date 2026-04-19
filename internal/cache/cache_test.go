package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	cacheDir := t.TempDir()
	email := "user@example.com"
	mailbox := "INBOX"
	var want uint32 = 12345

	if err := SaveUIDValidity(cacheDir, email, mailbox, want); err != nil {
		t.Fatalf("SaveUIDValidity: %v", err)
	}

	got, ok, err := LoadUIDValidity(cacheDir, email, mailbox)
	if err != nil {
		t.Fatalf("LoadUIDValidity: %v", err)
	}
	if !ok {
		t.Fatal("LoadUIDValidity: ok = false, want true")
	}
	if got != want {
		t.Errorf("LoadUIDValidity = %d, want %d", got, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cacheDir := t.TempDir()

	val, ok, err := LoadUIDValidity(cacheDir, "nobody@example.com", "INBOX")
	if err != nil {
		t.Fatalf("LoadUIDValidity: unexpected error: %v", err)
	}
	if ok {
		t.Error("LoadUIDValidity: ok = true, want false")
	}
	if val != 0 {
		t.Errorf("LoadUIDValidity = %d, want 0", val)
	}
}

func TestLoadCorruptContent(t *testing.T) {
	cacheDir := t.TempDir()
	email := "user@example.com"
	mailbox := "INBOX"

	dir := filepath.Join(cacheDir, email)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, mailbox), []byte("not-a-number\n"), 0600); err != nil {
		t.Fatal(err)
	}

	_, _, err := LoadUIDValidity(cacheDir, email, mailbox)
	if err == nil {
		t.Fatal("LoadUIDValidity: expected error for corrupt content, got nil")
	}
}

func TestSaveCreatesDirectories(t *testing.T) {
	cacheDir := t.TempDir()
	email := "deep@example.com"
	mailbox := "INBOX"

	// The email subdirectory does not exist yet.
	dir := filepath.Join(cacheDir, email)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected directory %q to not exist before save", dir)
	}

	if err := SaveUIDValidity(cacheDir, email, mailbox, 42); err != nil {
		t.Fatalf("SaveUIDValidity: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat directory after save: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("%q is not a directory", dir)
	}
}
