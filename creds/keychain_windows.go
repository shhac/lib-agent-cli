//go:build windows

package creds

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// newBackend selects the Windows backend: the Credential Manager, via go-keyring
// (which targets the credential as "<service>:<account>"). The reverse-DNS
// "app.paulie.<name>" service stays consistent with the other platforms.
func newBackend(service string) backend {
	return wincredBackend{service: service}
}

type wincredBackend struct{ service string }

// available is always true: the Credential Manager is present on every Windows
// host (no session-bus equivalent to check).
func (b wincredBackend) available() bool { return true }

func (b wincredBackend) get(account string) (string, bool) {
	v, err := keyring.Get(b.service, account)
	if err != nil || v == "" {
		return "", false
	}
	return v, true
}

func (b wincredBackend) set(account, secret string) error {
	if err := keyring.Set(b.service, account, secret); err != nil {
		return keychainErr("store secret", b.service, account, "", err)
	}
	return nil
}

func (b wincredBackend) delete(account string) error {
	if err := keyring.Delete(b.service, account); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return keychainErr("delete secret", b.service, account, "", err)
	}
	return nil
}

func (b wincredBackend) deleteAll() error {
	if err := keyring.DeleteAll(b.service); err != nil {
		return keychainErr("delete all secrets", b.service, "", "", err)
	}
	return nil
}
