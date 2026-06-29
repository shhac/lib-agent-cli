package creds

import keyring "github.com/shhac/lib-agent-keyring"

// The OS keychain backend now lives in the shared github.com/shhac/lib-agent-keyring
// module, so lib-agent-mcp (and anything else) can use it without depending on the
// rest of lib-agent-cli. These aliases keep the long-standing creds.Keychain API
// (and the NO_KEYCHAIN opt-out env vars) working unchanged for existing callers.

// Keychain stores secrets in the host's OS secret store. It is an alias for
// keyring.Keyring; see that package for details.
type Keychain = keyring.Keyring

// NewKeychain returns a Keychain for the given service.
func NewKeychain(service string) *keyring.Keyring { return keyring.New(service) }

// NewKeychainWithEnvPrefix is NewKeychain with an explicit env prefix.
func NewKeychainWithEnvPrefix(service, prefix string) *keyring.Keyring {
	return keyring.NewWithEnvPrefix(service, prefix)
}

// ErrKeychainUnavailable is returned by keychain mutations when no OS secret
// store is available.
var ErrKeychainUnavailable = keyring.ErrUnavailable

// NoKeychainKey is the env-namespace key that opts out of the OS keychain.
const NoKeychainKey = keyring.NoKeychainKey

// NoKeychainEnv is the family-wide opt-out variable (LIB_AGENT_NO_KEYCHAIN).
const NoKeychainEnv = keyring.NoKeychainEnv
