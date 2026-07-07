package cli

import (
	"sort"
	"strings"

	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// ConfigKey describes one settable configuration key. The Name and the typed
// Get/Set/Unset closures are the CLI's domain; ConfigCommand owns only the cobra
// scaffolding and dispatch.
type ConfigKey struct {
	Name        string
	Description string
	// Get returns the current value and whether it is set (vs. defaulted/absent).
	Get func() (value string, set bool)
	// Set parses, validates, and stores the value; a nil Set marks the key
	// read-only.
	Set func(value string) error
	// Unset clears the key back to its default; a nil Unset marks it un-clearable.
	Unset func() error
}

// ConfigCommand builds a `config` command group with get/set/unset/list over the
// given keys — the get/set/list/unset boilerplate ~8 family CLIs hand-roll.
//
// Every verb emits the same {key, value, set} record — the key's state after
// the command ran (list adds description) — honoring --format via g: NDJSON by
// default, the bare object (get/set/unset) or {"data":[…]} envelope (list)
// under json|yaml. A nil g always emits NDJSON. Unknown keys produce a
// fixable_by:agent error listing the valid ones.
func ConfigCommand(g *Globals, keys []ConfigKey) *cobra.Command {
	byName := make(map[string]ConfigKey, len(keys))
	for _, k := range keys {
		byName[k.Name] = k
	}
	names := sortedNames(keys)

	cfg := &cobra.Command{Use: "config", Short: "Get and set persisted configuration"}

	// lookup resolves the key argument, returning the family's structured
	// unknown-key error — so each subcommand starts with the same one-liner.
	lookup := func(arg string) (ConfigKey, error) {
		k, ok := byName[arg]
		if !ok {
			return ConfigKey{}, unknownKey(arg, names)
		}
		return k, nil
	}

	get := &cobra.Command{
		Use:   "get <key>",
		Short: "Show a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := lookup(args[0])
			if err != nil {
				return err
			}
			return EmitItem(cmd.OutOrStdout(), g.format(), configState(k))
		},
	}

	set := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := lookup(args[0])
			if err != nil {
				return err
			}
			if k.Set == nil {
				return output.New("config key is read-only: "+k.Name, output.FixableByAgent)
			}
			if err := k.Set(args[1]); err != nil {
				return output.Wrap(err, output.FixableByAgent)
			}
			// Echo the written value rather than re-reading: a nil-Get
			// (write-only) key would otherwise report itself unset.
			rec := map[string]any{"key": k.Name, "value": args[1], "set": true}
			return EmitItem(cmd.OutOrStdout(), g.format(), rec)
		},
	}

	unset := &cobra.Command{
		Use:   "unset <key>",
		Short: "Reset a configuration value to its default",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := lookup(args[0])
			if err != nil {
				return err
			}
			if k.Unset == nil {
				return output.New("config key cannot be unset: "+k.Name, output.FixableByAgent)
			}
			if err := k.Unset(); err != nil {
				return output.Wrap(err, output.FixableByAgent)
			}
			return EmitItem(cmd.OutOrStdout(), g.format(), configState(k))
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List all configuration keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := output.ResolveFormat(g.format(), output.FormatNDJSON)
			if err != nil {
				return err
			}
			items := make([]any, 0, len(names))
			for _, name := range names {
				k := byName[name]
				rec := configState(k)
				rec["description"] = k.Description
				items = append(items, rec)
			}
			return output.WriteList(cmd.OutOrStdout(), f, items, nil, nil)
		},
	}

	cfg.AddCommand(get, set, unset, list)
	return cfg
}

// format is nil-safe so ConfigCommand can take an optional *Globals — a CLI
// without shared flags gets the NDJSON default.
func (g *Globals) format() string {
	if g == nil {
		return ""
	}
	return g.Format
}

// configState is the record every config verb emits: the key's current state.
func configState(k ConfigKey) map[string]any {
	v, set := k.value()
	return map[string]any{"key": k.Name, "value": v, "set": set}
}

// value returns the key's current value and whether it is set, treating a nil
// Get (a write-only/derived key) as unset rather than panicking.
func (k ConfigKey) value() (string, bool) {
	if k.Get == nil {
		return "", false
	}
	return k.Get()
}

func sortedNames(keys []ConfigKey) []string {
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		names = append(names, k.Name)
	}
	sort.Strings(names)
	return names
}

func unknownKey(name string, valid []string) error {
	return output.Newf(output.FixableByAgent, "unknown config key %q", name).
		WithHint("valid keys: " + strings.Join(valid, ", "))
}
