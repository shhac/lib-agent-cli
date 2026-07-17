package creds

import (
	"io"
	"strings"

	"github.com/shhac/lib-agent-cli/internal/term"
)

// ReadSecret resolves a single secret without ever putting it on argv — where
// it would otherwise land in shell history, `ps`/`/proc`, and any agent
// transcript. It is the non-interactive companion to the dialog package's
// --form path: a secret piped on stdin stays out of the command line.
//
// Precedence:
//
//  1. flagVal, if non-empty — the explicit --flag path (convenient, but visible)
//  2. the trimmed contents of in, if in is a piped/redirected stream
//  3. "" — nothing supplied; the caller enforces required-ness and typically
//     surfaces a "provide via --flag, pipe on stdin, or use --form" hint
//
// When in is an interactive terminal and flagVal is empty, ReadSecret returns
// "" immediately rather than blocking on a tty read: a human with no secret to
// offer should be steered to --form, not left hanging on stdin waiting for EOF.
// A non-nil error indicates only a read failure. Callers pass cmd.InOrStdin().
func ReadSecret(in io.Reader, flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if term.IsTerminalReader(in) {
		return "", nil
	}
	b, err := io.ReadAll(in)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// ReadSecrets is the multi-secret companion to ReadSecret, for the rare command
// that accepts more than one piped secret in one shot (e.g. an API key and an
// application key). It fills each flag-backed field pointer from a single piped
// stream, owning the two invariants such a command needs so no consumer has to
// re-derive them:
//
//   - all-or-nothing: if ANY field is already set (its --flag was supplied),
//     stdin is not consulted at all — flags win and mixing sources is refused;
//   - positional mapping: the piped stream's non-empty line i fills fields[i].
//
// So `printf '%s\n%s' "$API" "$APP" | tool add` fills two &-passed fields in
// order. When in is an interactive terminal, or the stream has fewer lines than
// fields, the remaining fields are left unchanged and the caller enforces
// required-ness (or falls back to --form). A non-nil error indicates only a
// read failure.
func ReadSecrets(in io.Reader, fields ...*string) error {
	for _, f := range fields {
		if *f != "" {
			return nil // a --flag was supplied — flags win, don't touch stdin
		}
	}
	lines, err := readSecretLines(in)
	if err != nil {
		return err
	}
	for i, f := range fields {
		if i < len(lines) {
			*f = lines[i]
		}
	}
	return nil
}

// readSecretLines reads the piped stream once and returns its non-empty,
// trimmed lines. Returns nil when in is an interactive terminal (no pipe).
func readSecretLines(in io.Reader) ([]string, error) {
	if term.IsTerminalReader(in) {
		return nil, nil
	}
	b, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, ln := range strings.Split(string(b), "\n") {
		if s := strings.TrimSpace(ln); s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}
