package cli

import (
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"

	"github.com/shhac/lib-agent-cli/creds"
)

// Reaching a key that is in the FILE but not in the registry.
//
// ConfigCommand resolves against a fixed key list, which is right for set —
// writing a key nothing reads would only manufacture the problem — and wrong
// for get and unset. A config outlives the release that wrote it, so it can
// hold a key this build renamed or never knew. Those are exactly the keys
// somebody needs to look at and remove, and the registry answers "unknown
// config key" for all of them: true of the schema, unhelpful about the
// document.
//
// Opt-in, because ConfigCommand cannot do this unasked: it knows the key
// closures, not the store behind them. A CLI that wants different behaviour
// simply does not pass the option, and nothing changes.

// ConfigOption adjusts ConfigCommand. Options are additive and independent;
// an unrecognised combination is not a thing that exists.
type ConfigOption func(*configOptions)

type configOptions struct {
	store  *creds.Store
	schema any
}

// WithDocument lets `get` and `unset` reach keys the stored document holds but
// the registry does not, reading and editing s directly. schema is a prototype
// of the config struct, used only for its type.
//
// `set` is deliberately not extended. A key nothing reads is not a setting,
// and writing one would recreate the state this exists to clear.
//
// A key in neither the registry nor the document still takes the library's
// path, so a plain typo keeps its "unknown config key" error and the list of
// valid names — which is what a typo actually needs.
func WithDocument(s creds.Store, schema any) ConfigOption {
	return func(o *configOptions) {
		o.store, o.schema = &s, schema
	}
}

// documentFallback returns fn wrapped so that a key the registry cannot
// resolve, but the document holds, is handled by handle instead.
//
// The original RunE is taken as a parameter rather than read back off the
// command, so the wrapper cannot reach for the field it is about to overwrite
// and recurse forever.
func documentFallback(
	lib func(*cobra.Command, []string) error,
	known map[string]bool,
	store *creds.Store,
	handle func(cmd *cobra.Command, key, value string) error,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		key := args[0] // every one of these subcommands is cobra.ExactArgs(1)
		if known[key] {
			return lib(cmd, args)
		}
		value, found := store.RawValue(key)
		if !found {
			return lib(cmd, args)
		}
		return handle(cmd, key, value)
	}
}

// getUnknown reports a value the wrapper already read, so a get on a stray key
// parses the document once rather than twice. known_key marks the record as
// describing something the schema does not model.
func (o *configOptions) getUnknown(g *Globals) func(*cobra.Command, string, string) error {
	return func(cmd *cobra.Command, key, value string) error {
		return EmitItem(cmd.OutOrStdout(), g.format(),
			map[string]any{"key": key, "value": value, "set": true, "known_key": false})
	}
}

func (o *configOptions) unsetUnknown(g *Globals) func(*cobra.Command, string, string) error {
	return func(cmd *cobra.Command, key, _ string) error {
		removed, err := o.store.RawDelete(key)
		if err != nil {
			return output.Wrap(err, output.FixableByHuman)
		}
		return EmitItem(cmd.OutOrStdout(), g.format(),
			map[string]any{"key": key, "unset": removed, "known_key": false})
	}
}

// SectionKey describes a group of keys rather than a value: `unset` clears the
// whole section back to defaults, and `set` explains that it is not a value
// rather than reporting it read-only, which it is not.
//
// Clearing has to go through the struct, which is why unset is the CLI's
// closure and not something this package can derive: deleting the section from
// the document alone would last exactly until the next save wrote it back.
func SectionKey(name, description string, get func() (string, bool), unset func() error) ConfigKey {
	return ConfigKey{
		Name:        name,
		Description: description,
		Get:         get,
		Set: func(string) error {
			return output.Newf(output.FixableByAgent, "%s is a section, not a value", name).
				WithHint("set one of the keys inside it, or unset " + name + " to restore its defaults")
		},
		Unset: unset,
	}
}
