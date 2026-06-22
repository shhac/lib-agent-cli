package creds

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreSaveLoadRoundTripAnd0600(t *testing.T) {
	dir := t.TempDir()
	s := Store{Path: filepath.Join(dir, "nested", "credentials.json")}

	type creds struct {
		Default string            `json:"default,omitempty"`
		Tokens  map[string]string `json:"tokens,omitempty"`
	}
	in := creds{Default: "a", Tokens: map[string]string{"a": "secret"}}
	if err := s.Save(in); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perms = %o, want 600", perm)
	}

	var out creds
	if err := s.Load(&out); err != nil {
		t.Fatal(err)
	}
	if out.Default != "a" || out.Tokens["a"] != "secret" {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

func TestStoreLoadMissingFileIsEmpty(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "does-not-exist.json")}
	var v map[string]any
	if err := s.Load(&v); err != nil {
		t.Errorf("missing file should not error, got %v", err)
	}
}

func TestFirstNonEmptyAndGetenv(t *testing.T) {
	if got := FirstNonEmpty("", "", "x", "y"); got != "x" {
		t.Errorf("FirstNonEmpty = %q", got)
	}
	if got := FirstNonEmpty("", ""); got != "" {
		t.Errorf("FirstNonEmpty all-empty = %q", got)
	}
	t.Setenv("AGENT_FOO_TOKEN", "tok")
	if got := Getenv("FOO_TOKEN", "AGENT_FOO_TOKEN"); got != "tok" {
		t.Errorf("Getenv = %q", got)
	}
}

func TestFirstNonZero(t *testing.T) {
	if got := FirstNonZero(0, 0, 7, 9); got != 7 {
		t.Errorf("FirstNonZero = %d, want 7", got)
	}
	if got := FirstNonZero(0, 0); got != 0 {
		t.Errorf("FirstNonZero all-zero = %d, want 0", got)
	}
}
