package cli

import (
	"testing"

	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func TestNewRootRegistersSharedFlags(t *testing.T) {
	g := &Globals{}
	root := NewRoot(Options{Use: "demo", Globals: g})
	for _, name := range []string{"format", "timeout", "debug"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("missing persistent flag --%s", name)
		}
	}
	if !root.SilenceUsage || !root.SilenceErrors {
		t.Error("SilenceUsage/SilenceErrors should be set")
	}
}

func TestPersistentPreRunValidatesFormat(t *testing.T) {
	g := &Globals{}
	root := NewRoot(Options{Use: "demo", Globals: g})

	g.Format = "bogus"
	if err := root.PersistentPreRunE(root, nil); err == nil {
		t.Error("bad --format should error")
	}
	g.Format = "jsonl"
	if err := root.PersistentPreRunE(root, nil); err != nil {
		t.Errorf("valid --format should pass, got %v", err)
	}
}

func TestImagesFlagOptIn(t *testing.T) {
	// Not opted in: no --images flag, so a non-imaging tool doesn't advertise it.
	plain := NewRoot(Options{Use: "demo", Globals: &Globals{}})
	if plain.PersistentFlags().Lookup("images") != nil {
		t.Error("--images should not be registered without Options.Images")
	}

	// Opted in: flag exists, is hidden, and validates its value.
	g := &Globals{}
	root := NewRoot(Options{Use: "demo", Globals: g, Images: true})
	f := root.PersistentFlags().Lookup("images")
	if f == nil {
		t.Fatal("--images should be registered when Options.Images is set")
	}
	if !f.Hidden {
		t.Error("--images should be hidden")
	}
	if f.DefValue != "off" {
		t.Errorf("--images default = %q, want off", f.DefValue)
	}

	g.Images = "bogus"
	if err := root.PersistentPreRunE(root, nil); err == nil {
		t.Error("invalid --images should error")
	}
	g.Images = "auto"
	if err := root.PersistentPreRunE(root, nil); err != nil {
		t.Errorf("valid --images should pass, got %v", err)
	}
}

func TestHyperlinksFlagOptIn(t *testing.T) {
	if NewRoot(Options{Use: "demo", Globals: &Globals{}}).PersistentFlags().Lookup("hyperlinks") != nil {
		t.Error("--hyperlinks should not be registered without Options.Hyperlinks")
	}
	g := &Globals{}
	root := NewRoot(Options{Use: "demo", Globals: g, Hyperlinks: true})
	f := root.PersistentFlags().Lookup("hyperlinks")
	if f == nil || !f.Hidden || f.DefValue != "off" {
		t.Fatalf("--hyperlinks should be registered, hidden, default off; got %+v", f)
	}
	g.Hyperlinks = "bogus"
	if err := root.PersistentPreRunE(root, nil); err == nil {
		t.Error("invalid --hyperlinks should error")
	}
	g.Hyperlinks = "auto"
	if err := root.PersistentPreRunE(root, nil); err != nil {
		t.Errorf("valid --hyperlinks should pass, got %v", err)
	}
}

func TestConfigDefaultsHookRuns(t *testing.T) {
	called := false
	root := NewRoot(Options{Use: "demo", Globals: &Globals{}, ConfigDefaults: func(*cobra.Command) { called = true }})
	_ = root.PersistentPreRunE(root, nil)
	if !called {
		t.Error("ConfigDefaults hook should run in PersistentPreRunE")
	}
}

func TestHandleUnknownCommand(t *testing.T) {
	root := NewRoot(Options{Use: "demo", Globals: &Globals{}, UnknownHint: "run 'demo --help'"})
	root.AddCommand(&cobra.Command{Use: "foo", RunE: func(*cobra.Command, []string) error { return nil }})

	err := root.RunE(root, []string{"bar"})
	if err == nil {
		t.Fatal("unknown subcommand should error")
	}
	var oe *output.Error
	if !output.As(err, &oe) {
		t.Fatalf("error should be *output.Error, got %T", err)
	}
	if oe.FixableBy != output.FixableByAgent {
		t.Errorf("fixable_by = %q, want agent", oe.FixableBy)
	}
	if oe.Hint != "run 'demo --help'" {
		t.Errorf("hint = %q", oe.Hint)
	}
}
