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
	// Color is the --color mode: "auto" (default), "always", or "never". It is
	// resolved in PersistentPreRunE into the output package's process-wide color
	// mode; the actual decision is per-stream (a piped stdout stays uncolored even
	// when stderr is a terminal).
	Color string
	// Expose is the --expose allowlist: paths/keys (or "all") that reveal an
	// otherwise-redacted field. A CLI that redacts output passes it straight to
	// output.Redactor. It is bound to --expose only when Options.Redacts is set, so
	// a non-redacting CLI doesn't advertise a no-op flag; on those it stays nil.
	Expose []string
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
	// Redacts indicates the CLI redacts output (it constructs an output.Redactor /
	// calls output.Redact). Only then does NewRoot register the global --expose
	// flag — so a tool that never hides a field doesn't advertise a flag that
	// would do nothing. When true, --expose binds to Globals.Expose and the CLI
	// passes it to output.Redact, keeping the @redacted notes' "--expose <path>"
	// hint actionable.
	Redacts bool
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
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if o.ConfigDefaults != nil {
				o.ConfigDefaults()
			}
			if o.Globals != nil {
				// Resolve --color first so even a subsequent format error renders
				// with the chosen color policy. An unknown value is agent-fixable.
				// ParseColorMode returns the safe ColorAuto default on error, and we
				// set it unconditionally so a bad value never leaves a previously-set
				// (e.g. earlier in-process) mode in force — the error then renders
				// under auto, not stale state.
				mode, err := output.ParseColorMode(o.Globals.Color)
				output.SetColorMode(mode)
				if err != nil {
					return err
				}
			}
			if o.Globals != nil && o.Globals.Format != "" {
				if _, err := output.ParseFormat(o.Globals.Format); err != nil {
					// A command may opt into extra formats it renders itself
					// (e.g. a conversation "transcript") via AllowFormats; those
					// are not in the universal set and are valid only there.
					if !formatAllowedFor(cmd, o.Globals.Format) {
						return unknownFormatError(cmd, o.Globals.Format)
					}
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
		pf.StringVar(&o.Globals.Color, "color", "auto", "Colorize output: auto (color when the stream is a terminal), always, or never")
		_ = root.RegisterFlagCompletionFunc("color", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return []string{"auto", "always", "never"}, cobra.ShellCompDirectiveNoFileComp
		})
		if o.Redacts {
			pf.StringSliceVar(&o.Globals.Expose, "expose", nil, "Reveal redacted fields by path or key (repeatable; 'all' to reveal everything)")
		}
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
