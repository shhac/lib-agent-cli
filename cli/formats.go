package cli

import (
	"strings"

	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// extraFormatsAnnotation stores a command's opted-in extra --format values
// (comma-separated) in its cobra Annotations.
const extraFormatsAnnotation = "libcli.extra-formats"

// AllowFormats registers --format values accepted on cmd (and its subcommands)
// beyond the universal json|yaml|jsonl set. These are NOT universal formats:
// the command renders them itself. Use for domain-specific display formats that
// only make sense on certain commands — e.g. a conversation "transcript". The
// --format validator installed by NewRoot consults these, so an opted-in value
// is accepted on those commands and rejected (with a structured error)
// everywhere else. The command branches on the resolved format to render it.
//
// transcript-style formats deliberately stay out of lib-agent-output's universal
// enum (which every CLI honors); this keeps domain renderings opt-in and local.
func AllowFormats(cmd *cobra.Command, formats ...string) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	var all []string
	if cur := cmd.Annotations[extraFormatsAnnotation]; cur != "" {
		all = strings.Split(cur, ",")
	}
	for _, f := range formats {
		if f != "" {
			all = append(all, f)
		}
	}
	cmd.Annotations[extraFormatsAnnotation] = strings.Join(all, ",")
}

// extraFormatsFor returns the extra formats opted into by cmd or any ancestor,
// so AllowFormats can be set on a leaf command or a whole command group.
func extraFormatsFor(cmd *cobra.Command) []string {
	var out []string
	for c := cmd; c != nil; c = c.Parent() {
		if v := c.Annotations[extraFormatsAnnotation]; v != "" {
			out = append(out, strings.Split(v, ",")...)
		}
	}
	return out
}

// FormatAllowed reports whether format was opted into by cmd or an ancestor
// via AllowFormats. Beyond the validator's own use, it lets a CLI scope other
// decisions to the same command classes — e.g. a ConfigDefaults hook applying
// a persisted csv default only where csv is a legal --format value, so one
// annotation is the single source of truth for a domain format's reach.
func FormatAllowed(cmd *cobra.Command, format string) bool {
	for _, f := range extraFormatsFor(cmd) {
		if f == format {
			return true
		}
	}
	return false
}

// unknownFormatError is the structured fixable_by:agent error for a --format
// value that is neither universal nor opted into by the target command. It
// lists the universal set plus any extras the command does accept.
func unknownFormatError(cmd *cobra.Command, format string) error {
	allowed := []string{"json", "yaml", "jsonl"}
	allowed = append(allowed, extraFormatsFor(cmd)...)
	return output.Newf(output.FixableByAgent, "unknown format %q, expected: %s", format, strings.Join(allowed, ", "))
}
