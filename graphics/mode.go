package graphics

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// Mode is the inline-image policy chosen by a CLI's --images flag. It mirrors
// the shape of --color (off/auto/on ≈ never/auto/always): a conservative
// default, an environment-gated middle, and a forcing override.
type Mode int

const (
	// ModeOff never emits images — the safe default. Output stays plain text,
	// so machine/LLM consumers are never handed escape bytes.
	ModeOff Mode = iota
	// ModeAuto emits images only when the stream is a terminal that speaks the
	// protocol (a TTY and Detect() != ModeNone). The "just works for a human,
	// stays plain when piped" setting.
	ModeAuto
	// ModeOn forces images regardless of TTY or capability detection — the
	// escape for the false-negative case (a capable terminal Detect's env
	// heuristic doesn't recognize). Like --color always, it will write escapes
	// into a pipe; that footgun is the user's explicit choice.
	ModeOn
)

// ParseMode maps an --images flag value to a Mode. Empty is ModeOff (the
// default). Unknown values are an error a CLI surfaces as agent-fixable.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off":
		return ModeOff, nil
	case "auto":
		return ModeAuto, nil
	case "on":
		return ModeOn, nil
	}
	return ModeOff, fmt.Errorf("invalid images mode %q (want off, auto, or on)", s)
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

// Active reports whether images should be emitted to w under mode. This is the
// images counterpart to output.Enabled for color: the per-stream decision a
// renderer consults before choosing the image branch over a text fallback.
//   - off  → never
//   - auto → only when w is a TTY and Detect() reports a graphics protocol
//   - on   → always (the user forces past TTY/detection)
func Active(w io.Writer, mode Mode) bool {
	switch mode {
	case ModeOn:
		return true
	case ModeAuto:
		return isTerminal(w) && Detect() != ProtocolNone
	default:
		return false
	}
}

// isTerminal reports whether w is a terminal. Only an *os.File can be one; any
// other writer (pipe, buffer in tests) is treated as non-terminal, so auto mode
// keeps piped/captured output plain.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
