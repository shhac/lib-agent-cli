// Package cli is the cobra scaffolding shared by agent-first CLIs: a root
// command builder that wires the family's persistent flags
// (--format/--timeout/--debug), validates --format up front, installs a
// structured unknown-subcommand handler, and offers a one-line run/error helper.
//
// It builds on lib-agent-output for the error and format contract. The CLI
// supplies its name, version, domain flags, and an optional config-defaults
// hook; everything else is the copied boilerplate this package removes.
package cli

import (
	"strings"

	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// Globals holds the persistent flags shared by every command. A CLI keeps one,
// passes it to NewRoot, and adds its own domain flags to the returned root.
type Globals struct {
	Format    string
	TimeoutMS int
	Debug     bool
}

// Options configures NewRoot.
type Options struct {
	Use     string
	Short   string
	Version string
	// Globals is bound to --format/--timeout/--debug. When nil, no shared flags
	// are registered.
	Globals *Globals
	// DefaultFormat documents the format used when --format is empty (the CLI
	// applies it when calling output.ResolveFormat). Optional.
	DefaultFormat output.Format
	// ConfigDefaults, if set, runs in PersistentPreRunE before --format is
	// validated — apply persisted config defaults to Globals / domain flags here.
	ConfigDefaults func()
	// UnknownHint is shown by the unknown-subcommand handler (e.g. "run 'foo usage'").
	UnknownHint string
}

// NewRoot builds the root command with the family conventions: SilenceUsage and
// SilenceErrors on, the shared persistent flags bound, --format validated up
// front, and a structured unknown-subcommand handler installed. Add domain
// persistent flags and subcommands to the returned command.
func NewRoot(o Options) *cobra.Command {
	root := &cobra.Command{
		Use:           o.Use,
		Short:         o.Short,
		Version:       o.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if o.ConfigDefaults != nil {
				o.ConfigDefaults()
			}
			if o.Globals != nil && o.Globals.Format != "" {
				if _, err := output.ParseFormat(o.Globals.Format); err != nil {
					return output.Wrap(err, output.FixableByAgent)
				}
			}
			return nil
		},
	}
	if o.Globals != nil {
		pf := root.PersistentFlags()
		pf.StringVarP(&o.Globals.Format, "format", "f", "", "Output format: json, yaml, jsonl")
		pf.IntVarP(&o.Globals.TimeoutMS, "timeout", "t", 0, "Request timeout in milliseconds")
		pf.BoolVarP(&o.Globals.Debug, "debug", "d", false, "Log debug diagnostics to stderr")
	}
	HandleUnknownCommand(root, o.UnknownHint)
	return root
}

// HandleUnknownCommand makes cmd return a structured fixable_by:agent error
// listing the valid subcommands when given an unknown one (instead of cobra's
// usage text), and falls back to help when given no args.
func HandleUnknownCommand(cmd *cobra.Command, hint string) {
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 {
			return c.Help()
		}
		var names []string
		for _, sub := range c.Commands() {
			if sub.Hidden || sub.Name() == "help" || sub.Name() == "completion" {
				continue
			}
			names = append(names, sub.Name())
		}
		err := output.Newf(output.FixableByAgent, "unknown command %q, valid commands: %s", args[0], strings.Join(names, ", "))
		if hint != "" {
			err = err.WithHint(hint)
		}
		return err
	}
}
