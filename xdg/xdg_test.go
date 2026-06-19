package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := ConfigDir("agent-foo"); got != "/xdg/agent-foo" {
		t.Errorf("ConfigDir = %q", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	home, _ := os.UserHomeDir()
	if got := ConfigDir("agent-foo"); got != filepath.Join(home, ".config", "agent-foo") {
		t.Errorf("ConfigDir default = %q", got)
	}
}

func TestCacheDataState(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/cache")
	t.Setenv("XDG_DATA_HOME", "/data")
	t.Setenv("XDG_STATE_HOME", "/state")
	if got := CacheDir("a"); got != "/cache/a" {
		t.Errorf("CacheDir = %q", got)
	}
	if got := DataDir("a"); got != "/data/a" {
		t.Errorf("DataDir = %q", got)
	}
	if got := StateDir("a"); got != "/state/a" {
		t.Errorf("StateDir = %q", got)
	}
}

func TestRuntimeDirFallsBack(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	if got := RuntimeDir("a"); got != "/run/user/1000/a" {
		t.Errorf("RuntimeDir = %q", got)
	}
	t.Setenv("XDG_RUNTIME_DIR", "")
	if got := RuntimeDir("a"); got != filepath.Join(os.TempDir(), "a") {
		t.Errorf("RuntimeDir fallback = %q, want temp/a", got)
	}
}

func TestOverrideForTest(t *testing.T) {
	restore := SetConfigBaseForTest("/tmp/base")
	defer restore()
	if got := ConfigDir("a"); got != "/tmp/base/a" {
		t.Errorf("ConfigDir override = %q", got)
	}
}

func TestApp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	t.Setenv("XDG_CACHE_HOME", "/cache")
	app := App{Name: "agent-foo"}
	if app.ConfigDir() != "/cfg/agent-foo" || app.CacheDir() != "/cache/agent-foo" {
		t.Errorf("App dirs = %q / %q", app.ConfigDir(), app.CacheDir())
	}
}
