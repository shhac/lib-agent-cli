//go:build windows

package creds

// newBackend selects the Windows backend: the Credential Manager, via the shared
// keyringStore (go-keyring targets the credential as "<service>:<account>"). The
// reverse-DNS "app.paulie.<name>" service stays consistent with the other
// platforms.
func newBackend(service string) backend {
	return windowsBackend{keyringStore{service: service}}
}

type windowsBackend struct{ keyringStore }

// available is always true: the Credential Manager is present on every Windows
// host (no session-bus equivalent to check).
func (windowsBackend) available() bool { return true }
