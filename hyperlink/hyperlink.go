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
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// Mode is the hyperlink policy chosen by a CLI's --hyperlinks flag.
type Mode int

const (
	// ModeOff never emits hyperlinks — the safe default; output stays plain.
	ModeOff Mode = iota
	// ModeAuto emits hyperlinks when the stream is a terminal (OSC 8 degrades to
	// plain label text where unsupported, so a TTY check suffices).
	ModeAuto
	// ModeOn forces hyperlinks regardless of TTY (e.g. piping into a pager that
	// renders them). Like --color always, the user owns that choice.
	ModeOn
)

// ParseMode maps a --hyperlinks flag value to a Mode. Empty is ModeOff.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off":
		return ModeOff, nil
	case "auto":
		return ModeAuto, nil
	case "on":
		return ModeOn, nil
	}
	return ModeOff, fmt.Errorf("invalid hyperlinks mode %q (want off, auto, or on)", s)
}

func (m Mode) String() string {
	switch m {
	case ModeAuto:
		return "auto"
	case ModeOn:
		return "on"
	default:
		return "off"
	}
}

// Active reports whether hyperlinks should be emitted to w under mode: off never,
// auto when w is a terminal, on always.
func Active(w io.Writer, mode Mode) bool {
	switch mode {
	case ModeOn:
		return true
	case ModeAuto:
		return isTerminal(w)
	default:
		return false
	}
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

// isTerminal reports whether w is a terminal; only an *os.File can be one.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
