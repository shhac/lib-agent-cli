// Package term is the shared home for terminal-output capability checks and the
// off/auto/on stream-toggle policy that the color, graphics, and hyperlink
// features all consult. It exists so the TTY check and the three-state Mode
// contract live in exactly one place instead of being copied into each feature
// package (where they drifted as siblings were added).
//
// It is a leaf: cli, graphics, and hyperlink all depend downward onto it. It
// deliberately does NOT live in lib-agent-output (which is dependency-free and
// merely *receives* the detector via SetTerminalDetector) nor in cli (which
// imports graphics/hyperlink, so hosting the shared helper there would invert
// the dependency).
package term

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// IsTerminal reports whether w is a terminal. Only an *os.File can be one; any
// other writer (a pipe, a buffer in tests) is treated as non-terminal, so an
// auto decision keeps machine-piped and captured output clean.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && fileIsTerminal(f)
}

// IsTerminalReader is the reader-typed companion to IsTerminal, for stdin-side
// checks — e.g. deciding whether to read a piped secret or steer the user to an
// interactive prompt. Only an *os.File can be a terminal; any other reader (a
// pipe, a bytes.Buffer or strings.Reader in tests) is treated as non-terminal.
func IsTerminalReader(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && fileIsTerminal(f)
}

// fileIsTerminal is the single isatty seam shared by the writer- and
// reader-typed entry points, keeping the go-isatty dependency in this one
// package (see cli/color.go on why that dependency is quarantined here).
func fileIsTerminal(f *os.File) bool {
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// Mode is the off/auto/on policy a --images/--hyperlinks-style flag chooses,
// mirroring the shape of --color (off/auto/on ≈ never/auto/always): a
// conservative default, an environment-gated middle, and a forcing override.
type Mode int

const (
	// Off never emits the feature — the safe default; output stays plain, so
	// machine/LLM consumers are never handed escape bytes.
	Off Mode = iota
	// Auto emits only when the stream is a capable terminal (per the feature's
	// own auto predicate). The "just works for a human, stays plain when piped"
	// setting.
	Auto
	// On forces the feature regardless of TTY/capability — the escape hatch for a
	// capable terminal the auto predicate doesn't recognize. Like --color always,
	// the user owns the footgun of forcing it into a pipe.
	On
)

// Parse maps a flag value to a Mode. Empty is Off. noun names the feature for
// the error message ("images", "hyperlinks"); unknown values are an error a CLI
// surfaces as agent-fixable.
func Parse(noun, s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off":
		return Off, nil
	case "auto":
		return Auto, nil
	case "on":
		return On, nil
	}
	return Off, fmt.Errorf("invalid %s mode %q (want off, auto, or on)", noun, s)
}

func (m Mode) String() string {
	switch m {
	case Auto:
		return "auto"
	case On:
		return "on"
	default:
		return "off"
	}
}

// Active reports whether the feature should be emitted to w under mode: Off
// never, Auto when the feature's auto predicate passes for w, On always. The
// predicate is the one piece each feature supplies (e.g. "a TTY" for hyperlinks,
// "a TTY that speaks the Kitty protocol" for graphics).
func Active(w io.Writer, mode Mode, auto func(io.Writer) bool) bool {
	switch mode {
	case On:
		return true
	case Auto:
		return auto(w)
	default:
		return false
	}
}
