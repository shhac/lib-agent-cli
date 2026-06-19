package creds

// App bundles a CLI's identity so it can resolve every standard directory — and
// the keychain — from one place: construct it once, then ask it for paths.
//
//	app := creds.App{Name: "agent-foo", KeychainService: "app.paulie.agent-foo"}
//	cfg := app.ConfigDir()   // ~/.config/agent-foo (or $XDG_CONFIG_HOME/agent-foo)
//	kc  := app.Keychain()    // macOS keychain for app.paulie.agent-foo
//
// Name is the per-app directory segment under each XDG base, following the
// family convention (the plain tool name). KeychainService is the reverse-domain
// keychain id — kept separate because the secret store and the directory layout
// are different namespaces.
type App struct {
	Name            string
	KeychainService string
}

// ConfigDir returns the app's config directory ($XDG_CONFIG_HOME/<Name>, else ~/.config/<Name>).
func (a App) ConfigDir() string { return ConfigDir(a.Name) }

// CacheDir returns the app's cache directory ($XDG_CACHE_HOME/<Name>, else ~/.cache/<Name>).
func (a App) CacheDir() string { return CacheDir(a.Name) }

// DataDir returns the app's data directory ($XDG_DATA_HOME/<Name>, else ~/.local/share/<Name>).
func (a App) DataDir() string { return DataDir(a.Name) }

// StateDir returns the app's state directory ($XDG_STATE_HOME/<Name>, else ~/.local/state/<Name>).
func (a App) StateDir() string { return StateDir(a.Name) }

// RuntimeDir returns the app's runtime directory under $XDG_RUNTIME_DIR, or an
// error when that variable is unset (the spec defines no fallback).
func (a App) RuntimeDir() (string, error) { return RuntimeDir(a.Name) }

// Keychain returns a Keychain bound to KeychainService.
func (a App) Keychain() *Keychain { return NewKeychain(a.KeychainService) }
