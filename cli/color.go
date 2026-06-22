package cli

import (
	output "github.com/shhac/lib-agent-output"

	"github.com/shhac/lib-agent-cli/internal/term"
)

// init injects the terminal detector into lib-agent-output so its color
// auto-detection works for any CLI built on this package — including before a
// --color flag is parsed (e.g. a cobra parse error rendered in auto mode). The
// output package stays dependency-free; the isatty dependency lives in
// internal/term (shared with graphics/hyperlink), injected here the same way the
// YAML encoder is injected via RegisterEncoder.
func init() {
	output.SetTerminalDetector(term.IsTerminal)
}
