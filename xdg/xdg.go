// Package xdg resolves the freedesktop base directories — config, cache, data,
// state, and runtime — for an application, applying the spec's environment
// variables with sensible fallbacks when they are unset.
// See https://specifications.freedesktop.org/basedir/latest/.
//
// This is a filesystem concern with nothing to do with secrets; credential
// storage and the keychain live in the sibling creds package. A CLI provides
// its app name (the family convention is the plain tool name, e.g.
// "agent-foo") and gets paths back — directly, or bundled via App.
package xdg

import (
	"os"
	"path/filepath"
)

var (
	configOverride string
	cacheOverride  string
)

// SetConfigBaseForTest overrides the config base directory for a test and
// returns a restore func.
func SetConfigBaseForTest(dir string) func() {
	prev := configOverride
	configOverride = dir
	return func() { configOverride = prev }
}

// SetCacheBaseForTest overrides the cache base directory for a test and returns
// a restore func.
func SetCacheBaseForTest(dir string) func() {
	prev := cacheOverride
	cacheOverride = dir
	return func() { cacheOverride = prev }
}

// ConfigDir returns the app's config directory: $XDG_CONFIG_HOME/<app>, else
// ~/.config/<app>. For state you keep — credentials, settings.
func ConfigDir(app string) string { return dir(configOverride, "XDG_CONFIG_HOME", ".config", app) }

// CacheDir returns the app's cache directory: $XDG_CACHE_HOME/<app>, else
// ~/.cache/<app>. For data you can regenerate; never for secrets.
func CacheDir(app string) string { return dir(cacheOverride, "XDG_CACHE_HOME", ".cache", app) }

// DataDir returns the app's data directory: $XDG_DATA_HOME/<app>, else
// ~/.local/share/<app>. For portable application data you keep.
func DataDir(app string) string { return dir("", "XDG_DATA_HOME", ".local/share", app) }

// StateDir returns the app's state directory: $XDG_STATE_HOME/<app>, else
// ~/.local/state/<app>. For logs, history, and other persistent-but-not-portable state.
func StateDir(app string) string { return dir("", "XDG_STATE_HOME", ".local/state", app) }

// RuntimeDir returns the app's runtime directory: $XDG_RUNTIME_DIR/<app>. The
// spec defines no fallback, so when XDG_RUNTIME_DIR is unset this degrades to a
// per-app subdir of the OS temp dir — usable, though without the runtime dir's
// lifetime/privacy guarantees.
func RuntimeDir(app string) string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, app)
	}
	return filepath.Join(os.TempDir(), app)
}

func dir(override, env, homeSubdir, app string) string {
	if override != "" {
		return filepath.Join(override, app)
	}
	if d := os.Getenv(env); d != "" {
		return filepath.Join(d, app)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, homeSubdir, app)
}

// App bundles the base directories for one application: construct it once with
// the app name, then ask for paths.
//
//	app := xdg.App{Name: "agent-foo"}
//	cfg := app.ConfigDir() // ~/.config/agent-foo
//	cache := app.CacheDir() // ~/.cache/agent-foo
type App struct{ Name string }

func (a App) ConfigDir() string  { return ConfigDir(a.Name) }
func (a App) CacheDir() string   { return CacheDir(a.Name) }
func (a App) DataDir() string    { return DataDir(a.Name) }
func (a App) StateDir() string   { return StateDir(a.Name) }
func (a App) RuntimeDir() string { return RuntimeDir(a.Name) }
