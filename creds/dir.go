// Package creds is the credential/config plumbing shared by agent-first CLIs:
// XDG config-dir resolution, a 0600 JSON store, a macOS keychain wrapper, and
// small resolution helpers.
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

var overrideBase string

// SetConfigBaseForTest overrides the config base directory (the parent of the
// per-app directory) for the duration of a test and returns a restore func.
func SetConfigBaseForTest(dir string) func() {
	prev := overrideBase
	overrideBase = dir
	return func() { overrideBase = prev }
}

// ConfigDir returns the per-app config directory: $XDG_CONFIG_HOME/<app> when
// XDG_CONFIG_HOME is set, otherwise ~/.config/<app>.
func ConfigDir(app string) string {
	if overrideBase != "" {
		return filepath.Join(overrideBase, app)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, app)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", app)
}
