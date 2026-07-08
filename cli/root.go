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

	"github.com/shhac/lib-agent-cli/graphics"
	"github.com/shhac/lib-agent-cli/hyperlink"
	"github.com/shhac/lib-agent-cli/internal/term"
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
	// Images is the --images mode: "off" (default), "auto", or "on". A CLI that
	// renders inline images passes it to graphics.ParseMode/Active to decide
	// per stream. Bound to --images only when Options.Images is set — image
	// rendering isn't universal, so a tool with nothing to draw doesn't advertise
	// the flag; on those it stays "". See package graphics.
	Images string
	// Hyperlinks is the --hyperlinks mode: "off" (default), "auto", or "on". A
	// CLI that renders OSC 8 terminal hyperlinks passes it to
	// hyperlink.ParseMode/Active. Bound only when Options.Hyperlinks is set; on
	// other tools it stays "". See package hyperlink.
	Hyperlinks string
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
	// It receives the command being executed so a CLI can scope a persisted
	// default to a command class (e.g. a query-only format default, checked via
	// FormatAllowed) — backfilled values are then validated like the flag,
	// keeping precedence (flag > config > built-in) a boundary concern rather
	// than something the output layer re-resolves per emit.
	ConfigDefaults func(cmd *cobra.Command)
	// UnknownHint is shown by the unknown-subcommand handler (e.g. "run 'foo usage'").
	UnknownHint string
	// Redacts indicates the CLI redacts output (it constructs an output.Redactor /
	// calls output.Redact). Only then does NewRoot register the global --expose
	// flag — so a tool that never hides a field doesn't advertise a flag that
	// would do nothing. When true, --expose binds to Globals.Expose and the CLI
	// passes it to output.Redact, keeping the @redacted notes' "--expose <path>"
	// hint actionable.
	Redacts bool
	// Images indicates the CLI renders inline terminal images (via the graphics
	// package). Only then does NewRoot register the hidden global --images flag
	// (off/auto/on) into Globals.Images and validate it up front — a tool with
	// nothing to draw doesn't advertise it. Hidden because it's a niche
	// human-only knob; the CLI documents it in its own usage text.
	Images bool
	// Hyperlinks indicates the CLI renders OSC 8 terminal hyperlinks (via the
	// hyperlink package). Like Images, it opts the hidden global --hyperlinks
	// flag (off/auto/on) into Globals.Hyperlinks with up-front validation.
	Hyperlinks bool
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
				o.ConfigDefaults(cmd)
			}
			if o.Globals == nil {
				return nil
			}
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
			if err := validateToggle(o.Images, o.Globals.Images, graphics.ParseMode); err != nil {
				return err
			}
			if err := validateToggle(o.Hyperlinks, o.Globals.Hyperlinks, hyperlink.ParseMode); err != nil {
				return err
			}
			if o.Globals.Format != "" {
				if _, err := output.ParseFormat(o.Globals.Format); err != nil {
					// A command may opt into extra formats it renders itself
					// (e.g. a conversation "transcript") via AllowFormats; those
					// are not in the universal set and are valid only there.
					if !FormatAllowed(cmd, o.Globals.Format) {
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
		if o.Images {
			bindStreamToggle(root, &o.Globals.Images, "images", "Render images inline: off, auto (when the terminal supports it), or on (force)")
		}
		if o.Hyperlinks {
			bindStreamToggle(root, &o.Globals.Hyperlinks, "hyperlinks", "Render OSC 8 terminal hyperlinks: off, auto (on a TTY), or on (force)")
		}
	}
	HandleUnknownCommand(root, o.UnknownHint)
	return root
}

// validateToggle validates an off/auto/on stream-toggle flag value (--images,
// --hyperlinks) when its feature is enabled, surfacing a bad value as an
// agent-fixable error. parse is the feature's own ParseMode.
func validateToggle(enabled bool, val string, parse func(string) (term.Mode, error)) error {
	if !enabled {
		return nil
	}
	if _, err := parse(val); err != nil {
		return output.New(err.Error(), output.FixableByAgent)
	}
	return nil
}

// bindStreamToggle registers a hidden off/auto/on stream-toggle persistent flag
// defaulting off, with value completion — the shared shape of --images and
// --hyperlinks.
func bindStreamToggle(root *cobra.Command, dst *string, name, usage string) {
	pf := root.PersistentFlags()
	pf.StringVar(dst, name, "off", usage)
	_ = pf.MarkHidden(name)
	_ = root.RegisterFlagCompletionFunc(name, func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"off", "auto", "on"}, cobra.ShellCompDirectiveNoFileComp
	})
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
