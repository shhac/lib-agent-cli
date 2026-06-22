//go:build linux

package creds

import "os"

// newBackend selects the Linux backend: the Secret Service (libsecret) over
// D-Bus, via the shared keyringStore. The service id is used verbatim, so the
// reverse-DNS "app.paulie.<name>" stays consistent with macOS and Windows.
func newBackend(service string) backend {
	return linuxBackend{keyringStore{service: service}}
}

type linuxBackend struct{ keyringStore }

// available reports whether a D-Bus session bus (where the Secret Service lives)
// is reachable. Headless Linux — CI, containers, SSH without a session — has no
// session bus, so we report unavailable and let callers use the file store
// instead of blocking on an unreachable daemon.
func (linuxBackend) available() bool {
	return os.Getenv("DBUS_SESSION_BUS_ADDRESS") != ""
}
