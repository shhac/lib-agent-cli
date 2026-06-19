// Package dialog prompts for secrets via a native OS dialog, so a token never
// transits argv or the agent's conversation — the boilerplate behind a `--form`
// flag. It uses ncruces/zenity (osascript on macOS, zenity/kdialog on Linux,
// the native API on Windows) and refuses gracefully, with a structured error,
// when no GUI is reachable (e.g. a headless SSH session).
package dialog

import (
	"context"
	"os"
	"runtime"

	"github.com/ncruces/zenity"
	output "github.com/shhac/lib-agent-output"
)

// Field is one input in a form.
type Field struct {
	ID      string // key in the returned map
	Label   string // prompt shown to the user
	Hidden  bool   // mask the input (tokens, passwords, cookies)
	Initial string // pre-filled value (e.g. an existing token to edit); optional
}

// Spec is a titled form of one or more fields.
type Spec struct {
	Title  string
	Fields []Field
}

// promptOne is the dialog backend; a package var so tests can replace it and
// never actually open a window.
var promptOne = zenityPromptOne

func zenityPromptOne(ctx context.Context, title string, f Field) (string, error) {
	opts := []zenity.Option{zenity.Title(title), zenity.Context(ctx)}
	if f.Hidden {
		opts = append(opts, zenity.HideText())
	}
	if f.Initial != "" {
		opts = append(opts, zenity.EntryText(f.Initial))
	}
	return zenity.Entry(f.Label, opts...)
}

// Prompt shows the form and returns the entered values keyed by field ID. It
// returns a structured error when no GUI is available (see Available) or when
// the user cancels.
func Prompt(ctx context.Context, spec Spec) (map[string]string, error) {
	if err := Available(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(spec.Fields))
	for _, f := range spec.Fields {
		v, err := promptOne(ctx, spec.Title, f)
		if err != nil {
			return nil, output.Wrap(err, output.FixableByHuman).
				WithHint("the secret prompt was cancelled or failed")
		}
		out[f.ID] = v
	}
	return out, nil
}

// PromptSecret is the single-hidden-field convenience used by most `--form`
// flows: one masked prompt, one returned value.
func PromptSecret(ctx context.Context, title, label string) (string, error) {
	res, err := Prompt(ctx, Spec{Title: title, Fields: []Field{{ID: "secret", Label: label, Hidden: true}}})
	if err != nil {
		return "", err
	}
	return res["secret"], nil
}

// Available reports whether a GUI dialog can be shown, returning a structured
// fixable_by:human error (not a panic) when not — so a CLI can fall back to an
// env var or flag on a headless host.
func Available() error {
	switch runtime.GOOS {
	case "darwin":
		if os.Getenv("SSH_CONNECTION") != "" && os.Getenv("TERM_PROGRAM") == "" {
			return noGUI("appears to be an SSH session with no local GUI")
		}
	case "linux":
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return noGUI("no DISPLAY/WAYLAND_DISPLAY is set")
		}
	}
	return nil
}

func noGUI(reason string) error {
	return output.New("cannot open a secret dialog: "+reason, output.FixableByHuman).
		WithHint("run on a machine with a GUI, or provide the secret via an environment variable")
}
