package creds

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDirXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := ConfigDir("agent-foo"); got != "/xdg/agent-foo" {
		t.Errorf("ConfigDir = %q, want /xdg/agent-foo", got)
	}
}

func TestConfigDirDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, _ := os.UserHomeDir()
	if got := ConfigDir("agent-foo"); got != filepath.Join(home, ".config", "agent-foo") {
		t.Errorf("ConfigDir = %q", got)
	}
}

func TestConfigDirOverrideForTest(t *testing.T) {
	restore := SetConfigBaseForTest("/tmp/base")
	defer restore()
	if got := ConfigDir("agent-foo"); got != "/tmp/base/agent-foo" {
		t.Errorf("ConfigDir = %q, want /tmp/base/agent-foo", got)
	}
}

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

func TestCacheDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/cache")
	if got := CacheDir("agent-foo"); got != "/cache/agent-foo" {
		t.Errorf("CacheDir = %q, want /cache/agent-foo", got)
	}
	t.Setenv("XDG_CACHE_HOME", "")
	home, _ := os.UserHomeDir()
	if got := CacheDir("agent-foo"); got != filepath.Join(home, ".cache", "agent-foo") {
		t.Errorf("CacheDir default = %q", got)
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
