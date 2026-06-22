// Package hyperlink renders OSC 8 terminal hyperlinks — clickable text whose
// URL is hidden behind a label, the way a browser shows link text rather than
// the href.
//
// It is the sibling of the graphics package: a human-terminal runtime concern
// (not the zero-dependency wire contract), exposing the same two-seam shape — a
// Mode (off/auto/on, the --hyperlinks flag, mirroring --images/--color) with
// Active(w, mode) as the per-stream decision, and Encode as the mechanism.
//
// Unlike pixel graphics, OSC 8 has no reliable capability probe: it is widely
// supported (iTerm2, Ghostty, kitty, WezTerm, GNOME Terminal, recent VTE, …) and
// degrades gracefully on terminals that don't grok it — they ignore the OSC
// sequence and print the label text. So auto gates on a TTY alone rather than a
// terminal-specific handshake; a user whose terminal mangles it can pass off.
package hyperlink

import (
	"io"
	"strings"

	"github.com/shhac/lib-agent-cli/internal/term"
)

// Mode is the hyperlink policy chosen by a CLI's --hyperlinks flag (off/auto/on),
// the shared three-state stream toggle defined in internal/term.
type Mode = term.Mode

const (
	ModeOff  = term.Off  // never emit hyperlinks — the safe default
	ModeAuto = term.Auto // emit when the stream is a terminal
	ModeOn   = term.On    // force regardless of TTY (like --color always)
)

// ParseMode maps a --hyperlinks flag value to a Mode. Empty is ModeOff.
func ParseMode(s string) (Mode, error) { return term.Parse("hyperlinks", s) }

// Active reports whether hyperlinks should be emitted to w under mode. OSC 8 has
// no reliable capability probe and degrades to plain label text, so the auto
// gate is a bare TTY check.
func Active(w io.Writer, mode Mode) bool {
	return term.Active(w, mode, term.IsTerminal)
}

// Encode wraps text as an OSC 8 hyperlink to url:
// ESC ] 8 ; ; URL ST  text  ESC ] 8 ; ; ST. It returns text unchanged when url
// is empty or carries control bytes that would break the sequence (ESC, BEL,
// newline) — a malformed link must never corrupt the surrounding stream.
func Encode(url, text string) string {
	if url == "" || strings.ContainsAny(url, "\x1b\x07\n\r") {
		return text
	}
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}
