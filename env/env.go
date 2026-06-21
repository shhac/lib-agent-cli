// Package env is the family's environment-variable namespace: a small resolver
// that looks up a key under a project-specific prefix first, then a family-wide
// fallback. It's the single funnel every lib-agent-cli env read should go
// through, so the precedence rules below hold everywhere the same way.
//
// A lookup for KEY checks, in order:
//
//	{Prefix}_{KEY}      e.g. AGENT_SLACK_NO_KEYCHAIN  (this CLI only)
//	LIB_AGENT_{KEY}     e.g. LIB_AGENT_NO_KEYCHAIN     (the whole family)
//
// So an operator can flip one tool by name, or set a single LIB_AGENT_* var to
// flip the entire family at once. Because the specific var is consulted by
// presence (not truthiness), setting it can also *override* a family-wide value
// — including re-enabling a behaviour the family var disabled.
package env

import (
	"os"
	"strings"
)

// FamilyPrefix is the namespace shared by every agent-* CLI and lin. A key K is
// reachable family-wide as FamilyPrefix + "_" + K.
const FamilyPrefix = "LIB_AGENT"

// Namespace resolves env vars for one CLI. The zero value (empty Prefix)
// consults only the family-wide fallback, which is a safe default.
type Namespace struct {
	// Prefix is the SCREAMING_SNAKE token for this CLI, e.g. "AGENT_SLACK".
	// Empty means "family-wide only".
	Prefix string
}

// New returns a Namespace for a CLI/binary name, deriving the prefix via
// PrefixFromName ("agent-slack" → "AGENT_SLACK").
func New(name string) Namespace {
	return Namespace{Prefix: PrefixFromName(name)}
}

// PrefixFromName turns a CLI/binary name into an env prefix: it uppercases and
// replaces the separators "-", ".", and " " with "_". So "agent-slack" →
// "AGENT_SLACK" and "lin" → "LIN". A dotted service id like "app.paulie.lin"
// would become "APP_PAULIE_LIN"; pass the bare name, not a service id.
func PrefixFromName(name string) string {
	r := strings.NewReplacer("-", "_", ".", "_", " ", "_")
	return strings.ToUpper(r.Replace(name))
}

// Lookup returns the value of the first set of {Prefix}_{key} or
// LIB_AGENT_{key}, and whether either was present. The specific var wins on
// presence, so an empty-but-set specific var still shadows the family var.
func (n Namespace) Lookup(key string) (string, bool) {
	if n.Prefix != "" {
		if v, ok := os.LookupEnv(n.Prefix + "_" + key); ok {
			return v, true
		}
	}
	return os.LookupEnv(FamilyPrefix + "_" + key)
}

// Flag reports key as a boolean: true for any present value other than "", "0",
// or "false" (case-insensitive). Absent → false. Because it builds on Lookup, a
// specific var set to a falsey value overrides a truthy family var.
func (n Namespace) Flag(key string) bool {
	v, ok := n.Lookup(key)
	if !ok {
		return false
	}
	switch strings.ToLower(v) {
	case "", "0", "false":
		return false
	default:
		return true
	}
}
