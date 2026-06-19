// Package creds is the credential/config plumbing shared by agent-first CLIs:
// XDG config/cache-dir resolution, a 0600 JSON store, a macOS keychain wrapper,
// and small resolution helpers.
//
// The split is deliberate: this package owns the *mechanism* (where files live,
// how secrets are stored, the resolution order), while the CLI supplies the
// *domain inputs* — the app name, the keychain service, the environment-variable
// names, and the credential schema. Nothing here knows about any specific API.
package creds

import (
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
