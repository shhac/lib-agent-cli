package graphics

import (
	"os"
	"strings"
)

// Protocol is the inline-image protocol a terminal understands.
type Protocol int

const (
	// ProtocolNone means no inline-image protocol was detected; callers should
	// fall back to plain text.
	ProtocolNone Protocol = iota
	// ProtocolKitty is the Kitty graphics protocol (Ghostty, kitty, WezTerm).
	ProtocolKitty
)

func (p Protocol) String() string {
	switch p {
	case ProtocolKitty:
		return "kitty"
	default:
		return "none"
	}
}

// Detect reports the inline-image protocol the current terminal supports, from
// the environment. It does NOT check whether output is a TTY — that is a
// separate decision the caller already owns (the same isatty seam color uses),
// because the terminal can be graphics-capable while stdout is a pipe.
//
// Detection is a heuristic, not a handshake: querying the terminal would need
// raw mode and a stdin read, which is too invasive for a library default. The
// env signals below identify the three terminals that speak the Kitty protocol.
// Note Ghostty advertises TERM=xterm-256color, so TERM_PROGRAM is the reliable
// signal there, not TERM.
func Detect() Protocol {
	return detect(os.Getenv)
}

func detect(getenv func(string) string) Protocol {
	switch strings.ToLower(getenv("TERM_PROGRAM")) {
	case "ghostty", "wezterm":
		return ProtocolKitty
	}
	if strings.Contains(getenv("TERM"), "kitty") {
		return ProtocolKitty
	}
	if getenv("KITTY_WINDOW_ID") != "" {
		return ProtocolKitty
	}
	return ProtocolNone
}
