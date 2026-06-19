// Package creds is the credential/config plumbing shared by agent-first CLIs:
// freedesktop XDG base-directory resolution (config, cache, data, state,
// runtime), a 0600 JSON store, a macOS keychain wrapper, and small resolution
// helpers. See https://specifications.freedesktop.org/basedir/latest/.
//
// The split is deliberate: this package owns the *mechanism* (where files live,
// how secrets are stored, the resolution order), while the CLI supplies the
// *domain inputs* — the app name, the keychain service, the environment-variable
// names, and the credential schema. Nothing here knows about any specific API.
// A CLI that wants every path at once constructs an App once (see app.go).
package creds

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	configOverride string
	cacheOverride  string
)

// SetConfigBaseForTest overrides the config base directory (the parent of the
// per-app directory) for the duration of a test and returns a restore func.
func SetConfigBaseForTest(dir string) func() {
	prev := configOverride
	configOverride = dir
	return func() { configOverride = prev }
}

// SetCacheBaseForTest overrides the cache base directory for the duration of a
// test and returns a restore func.
func SetCacheBaseForTest(dir string) func() {
	prev := cacheOverride
	cacheOverride = dir
	return func() { cacheOverride = prev }
}

// ConfigDir returns the per-app config directory: $XDG_CONFIG_HOME/<app> when
// set, otherwise ~/.config/<app>. Use it for state you keep — credentials,
// settings.
func ConfigDir(app string) string {
	return xdgDir(configOverride, "XDG_CONFIG_HOME", ".config", app)
}

// CacheDir returns the per-app cache directory: $XDG_CACHE_HOME/<app> when set,
// otherwise ~/.cache/<app>. Use it for data you can regenerate — warmed
// lookups, downloads — and never for secrets.
func CacheDir(app string) string {
	return xdgDir(cacheOverride, "XDG_CACHE_HOME", ".cache", app)
}

// DataDir returns the per-app data directory: $XDG_DATA_HOME/<app> when set,
// otherwise ~/.local/share/<app>. Use it for portable application data you keep
// (not secrets — those go in the keychain or the 0600 config store).
func DataDir(app string) string {
	return xdgDir("", "XDG_DATA_HOME", ".local/share", app)
}

// StateDir returns the per-app state directory: $XDG_STATE_HOME/<app> when set,
// otherwise ~/.local/state/<app>. Use it for logs, history, and other state that
// persists between runs but isn't portable configuration.
func StateDir(app string) string {
	return xdgDir("", "XDG_STATE_HOME", ".local/state", app)
}

// RuntimeDir returns the per-app runtime directory under $XDG_RUNTIME_DIR. Per
// the freedesktop spec there is NO portable fallback, so it returns an error
// when XDG_RUNTIME_DIR is unset — the caller decides whether to degrade (e.g.
// to os.TempDir) or fail.
func RuntimeDir(app string) (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return "", errors.New("XDG_RUNTIME_DIR is not set; the freedesktop spec defines no fallback")
	}
	return filepath.Join(dir, app), nil
}

func xdgDir(override, env, homeSubdir, app string) string {
	if override != "" {
		return filepath.Join(override, app)
	}
	if dir := os.Getenv(env); dir != "" {
		return filepath.Join(dir, app)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, homeSubdir, app)
}
