package cli

import (
	"strings"
	"testing"

	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func TestAllowFormats(t *testing.T) {
	build := func() *cobra.Command {
		root := NewRoot(Options{Use: "x", Globals: &Globals{}})
		conv := &cobra.Command{Use: "conv", RunE: func(*cobra.Command, []string) error { return nil }}
		AllowFormats(conv, "transcript")
		plain := &cobra.Command{Use: "plain", RunE: func(*cobra.Command, []string) error { return nil }}
		root.AddCommand(conv, plain)
		return root
	}

	t.Run("opted-in format accepted on its command", func(t *testing.T) {
		root := build()
		root.SetArgs([]string{"--format", "transcript", "conv"})
		if err := root.Execute(); err != nil {
			t.Fatalf("transcript should be accepted on conv: %v", err)
		}
	})

	t.Run("opted-in format rejected on a command that didn't opt in", func(t *testing.T) {
		root := build()
		root.SetArgs([]string{"--format", "transcript", "plain"})
		if err := root.Execute(); err == nil {
			t.Fatal("transcript must be rejected on 'plain'")
		}
	})

	t.Run("universal formats still work everywhere", func(t *testing.T) {
		root := build()
		root.SetArgs([]string{"--format", "jsonl", "plain"})
		if err := root.Execute(); err != nil {
			t.Fatalf("jsonl should work on any command: %v", err)
		}
	})

	t.Run("unknown format error lists universal set plus the command's extras", func(t *testing.T) {
		root := build()
		root.SetArgs([]string{"--format", "bogus", "conv"})
		err := root.Execute()
		if err == nil {
			t.Fatal("bogus should error")
		}
		var apiErr *output.Error
		if !output.As(err, &apiErr) {
			t.Fatalf("want *output.Error, got %T", err)
		}
		if apiErr.FixableBy != output.FixableByAgent {
			t.Errorf("want fixable_by=agent, got %v", apiErr.FixableBy)
		}
		for _, want := range []string{"json", "yaml", "jsonl", "transcript"} {
			if !strings.Contains(apiErr.Message, want) {
				t.Errorf("error should list %q; got: %s", want, apiErr.Message)
			}
		}
	})

	t.Run("AllowFormats on a group applies to its subcommands", func(t *testing.T) {
		root := NewRoot(Options{Use: "x", Globals: &Globals{}})
		group := &cobra.Command{Use: "msg"}
		AllowFormats(group, "transcript")
		leaf := &cobra.Command{Use: "list", RunE: func(*cobra.Command, []string) error { return nil }}
		group.AddCommand(leaf)
		root.AddCommand(group)
		root.SetArgs([]string{"--format", "transcript", "msg", "list"})
		if err := root.Execute(); err != nil {
			t.Fatalf("group opt-in should reach subcommand: %v", err)
		}
	})
}
