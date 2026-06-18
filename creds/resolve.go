package creds

import "os"

// FirstNonEmpty returns the first non-empty string, or "" if all are empty. It
// is the building block for the family's "flag, then env, then stored default"
// resolution order: FirstNonEmpty(flag, os.Getenv("X"), cfg.Default).
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Getenv returns the first non-empty value among the named environment
// variables — for CLIs that accept both a vendor name (FOO_TOKEN) and an
// agent-prefixed name (AGENT_FOO_TOKEN).
func Getenv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}
