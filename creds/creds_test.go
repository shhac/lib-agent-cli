package creds

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestKeychainViaInjectedRun(t *testing.T) {
	k := NewKeychain("app.paulie.test")
	if !k.Available() {
		t.Skip("keychain is macOS-only")
	}
	var lastArgs []string
	k.run = func(args ...string) (string, error) {
		lastArgs = args
		if args[0] == "find-generic-password" {
			return "the-secret", nil
		}
		return "", nil
	}

	if v, ok := k.Get("acct"); !ok || v != "the-secret" {
		t.Errorf("Get = %q, %v", v, ok)
	}
	if lastArgs[0] != "find-generic-password" || lastArgs[2] != "app.paulie.test" || lastArgs[4] != "acct" {
		t.Errorf("Get args = %v", lastArgs)
	}
	if err := k.Set("acct", "s"); err != nil {
		t.Fatal(err)
	}
	if lastArgs[0] != "add-generic-password" {
		t.Errorf("Set args = %v", lastArgs)
	}
}

func TestKeychainSetErrorIncludesDiagnostic(t *testing.T) {
	k := NewKeychain("app.paulie.test")
	if !k.Available() {
		t.Skip("keychain is macOS-only")
	}
	k.run = func(args ...string) (string, error) {
		return "security: SecKeychainItemCreateFromContent: write permission denied", errBoom
	}
	err := k.Set("acct", "s")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	// the security diagnostic, the account, and the service should all surface
	for _, want := range []string{"write permission denied", "acct", "app.paulie.test"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
	if !errors.Is(err, errBoom) {
		t.Error("wrapped error should preserve the underlying cause")
	}
}

var errBoom = errors.New("exit status 1")

func TestKeychainDeleteAllLoopsUntilEmpty(t *testing.T) {
	k := NewKeychain("app.paulie.test")
	if !k.Available() {
		t.Skip("keychain is macOS-only")
	}
	calls := 0
	k.run = func(args ...string) (string, error) {
		if args[0] != "delete-generic-password" {
			t.Fatalf("unexpected call %v", args)
		}
		calls++
		if calls <= 3 { // 3 items, then security reports empty
			return "", nil
		}
		return "security: could not be found", errBoom
	}
	if err := k.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll = %v", err)
	}
	if calls != 4 {
		t.Errorf("DeleteAll made %d calls, want 4 (3 deletes + terminating miss)", calls)
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
