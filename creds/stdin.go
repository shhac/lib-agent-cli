package creds

import (
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
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
	if isInteractive(in) {
		return "", nil
	}
	b, err := io.ReadAll(in)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// ReadSecretLines is ReadSecret for the rare command that accepts more than one
// piped secret in one shot (e.g. an API key and an application key). It reads
// the piped stream once and returns its non-empty lines, trimmed, so the caller
// can map them onto fields in a documented order (line 1 → first field, …).
//
// Returns nil (no error) when in is an interactive terminal. To avoid the
// ambiguity of mixing sources, the multi-secret contract is all-or-nothing:
// callers should consult stdin only when every such field's --flag was empty,
// and otherwise resolve each field from its flag with FirstNonEmpty.
func ReadSecretLines(in io.Reader) ([]string, error) {
	if isInteractive(in) {
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

// isInteractive reports whether in is a terminal. Only an *os.File can be one;
// a pipe, a redirected file, or a test buffer is treated as non-interactive, so
// piped secrets are read while an interactive shell is not blocked on stdin.
func isInteractive(in io.Reader) bool {
	f, ok := in.(*os.File)
	if !ok {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
