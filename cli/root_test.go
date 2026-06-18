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

func TestConfigDefaultsHookRuns(t *testing.T) {
	called := false
	root := NewRoot(Options{Use: "demo", Globals: &Globals{}, ConfigDefaults: func() { called = true }})
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
