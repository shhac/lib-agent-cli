//go:build linux

package creds

import (
	"errors"
	"os"

	"github.com/zalando/go-keyring"
)

// newBackend selects the Linux backend: the Secret Service (libsecret) over
// D-Bus, via go-keyring. The service id is used verbatim as the collection
// attribute, so the reverse-DNS "app.paulie.<name>" stays consistent with macOS.
func newBackend(service string) backend {
	return keyringBackend{service: service}
}

type keyringBackend struct{ service string }

// available reports whether a D-Bus session bus (where the Secret Service lives)
// is reachable. Headless Linux — CI, containers, SSH without a session — has no
// session bus, so we report unavailable and let callers use the file store
// instead of blocking on an unreachable daemon.
func (b keyringBackend) available() bool {
	return os.Getenv("DBUS_SESSION_BUS_ADDRESS") != ""
}

func (b keyringBackend) get(account string) (string, bool) {
	v, err := keyring.Get(b.service, account)
	if err != nil || v == "" {
		return "", false
	}
	return v, true
}

func (b keyringBackend) set(account, secret string) error {
	if err := keyring.Set(b.service, account, secret); err != nil {
		return keychainErr("store secret", b.service, account, "", err)
	}
	return nil
}

func (b keyringBackend) delete(account string) error {
	if err := keyring.Delete(b.service, account); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return keychainErr("delete secret", b.service, account, "", err)
	}
	return nil
}

func (b keyringBackend) deleteAll() error {
	if err := keyring.DeleteAll(b.service); err != nil {
		return keychainErr("delete all secrets", b.service, "", "", err)
	}
	return nil
}
