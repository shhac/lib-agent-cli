package graphics

import (
	"io"

	"github.com/shhac/lib-agent-cli/internal/term"
)

// Mode is the inline-image policy chosen by a CLI's --images flag (off/auto/on,
// mirroring --color). It is the shared three-state stream toggle; the constants
// and Parse/String live in internal/term so --images and --hyperlinks can't
// drift.
type Mode = term.Mode

const (
	ModeOff  = term.Off  // never emit images — the safe default
	ModeAuto = term.Auto // emit only on a TTY that speaks the protocol
	ModeOn   = term.On   // force, past TTY/detection (like --color always)
)

// ParseMode maps an --images flag value to a Mode. Empty is ModeOff; unknown
// values are an error a CLI surfaces as agent-fixable.
func ParseMode(s string) (Mode, error) { return term.Parse("images", s) }

// Active reports whether images should be emitted to w under mode — the images
// counterpart to output.Enabled for color. Auto is gated on a TTY that also
// reports a graphics protocol; on forces; off never.
func Active(w io.Writer, mode Mode) bool {
	return term.Active(w, mode, func(w io.Writer) bool {
		return term.IsTerminal(w) && Detect() != ProtocolNone
	})
}
