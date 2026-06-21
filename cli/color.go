package cli

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"
	output "github.com/shhac/lib-agent-output"
)

// init injects the terminal detector into lib-agent-output so its color
// auto-detection works for any CLI built on this package — including before a
// --color flag is parsed (e.g. a cobra parse error rendered in auto mode). The
// output package stays dependency-free; the isatty dependency lives here, the
// same way the YAML encoder is injected via RegisterEncoder.
func init() {
	output.SetTerminalDetector(isTerminal)
}

// isTerminal reports whether w is a terminal. Only *os.File can be one; any other
// writer (a pipe, a buffer in tests) is treated as non-terminal, so auto mode
// keeps machine-piped and captured output clean.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
