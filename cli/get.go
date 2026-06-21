package cli

import (
	"io"

	output "github.com/shhac/lib-agent-output"
)

// EmitItem writes a single, already-resolved record per the family's get-output
// contract: NDJSON by default (one compact line), or the pretty object under
// --format json|yaml. Use it for single-only gets — composite-key (e.g. a
// message by channel+ts), singletons (e.g. a balance), or multi-field lookups —
// that don't fit EntityGet's 1..N id model but should still default to NDJSON
// like every other get.
//
// Unlike EntityGet, --format json|yaml yields the BARE object, not a
// {"data":[…]} envelope, because there is exactly one thing. Reserve this for
// commands that emit a structured record; raw passthrough/content responses
// (e.g. `api get <path>`, a request verb's envelope) should keep their own
// rendering and are NOT get-wrapped.
func EmitItem(w io.Writer, format string, item any) error {
	f, err := output.ResolveFormat(format, output.FormatNDJSON)
	if err != nil {
		return err
	}
	if f == output.FormatNDJSON {
		return output.NewNDJSONWriter(w).WriteItem(item)
	}
	return output.Print(w, item, f, nil)
}

// unresolvedRecord is the per-id marker emitted for an input that a
// multi-capable get could not resolve. It is carried on stdout — NEVER on
// stderr — as an `@unresolved` control line (NDJSON) or under the envelope's
// `@unresolved` key (json/yaml). The fields mirror the error contract so an
// agent can act on each miss exactly as it would on a stderr error.
type unresolvedRecord struct {
	ID                string `json:"id"`
	Reason            string `json:"reason,omitempty"`
	FixableBy         string `json:"fixable_by,omitempty"`
	Hint              string `json:"hint,omitempty"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
}

// EntityGet runs the family's get contract for 1..N ids against a resolver and
// writes the result to w. It is the single home of the "single and multi get
// look the same" behavior:
//
//   - One result per input, in input order. The default format (NDJSON) emits
//     one line per input: either the resolved record, or a
//     `{"@unresolved":{id,reason,fixable_by,…}}` control line in that position.
//   - --format json|yaml collapse to a single document
//     `{"data":[…resolved…], "@unresolved":[…]}` (data in input order; misses
//     aggregated), matching the family's list envelope.
//   - Exit code reflects whether the command RAN, not whether every id
//     resolved: an item-level miss (not-found / rate-limited) is an
//     `@unresolved` record on stdout and EntityGet returns nil (exit 0). Only a
//     command-level failure (auth/permission, or a non-rate-limit transient)
//     is returned to the caller, which renders it on stderr and exits 1.
//   - Resolution is buffered before any write, so a command-level failure
//     leaves stdout empty — preserving the single-sink invariant.
//
// The resolver returns the record for an id, or an error. Errors are classified
// via the family's *output.Error.FixableBy: agent → item-level; human → fatal;
// retry with a retry-after (429) → item-level; retry without (network/5xx) →
// fatal. An error that is not an *output.Error is treated as fatal.
//
// Records are written as the resolver returns them; pre-shape (compact/prune)
// inside the resolver if needed.
func EntityGet(w io.Writer, format string, args []string, resolve func(id string) (any, error)) error {
	type slot struct {
		record     any
		unresolved *unresolvedRecord
	}
	slots := make([]slot, 0, len(args))
	for _, id := range args {
		v, err := resolve(id)
		if err == nil {
			slots = append(slots, slot{record: v})
			continue
		}
		rec, fatal := classifyResolveErr(id, err)
		if fatal {
			return err
		}
		slots = append(slots, slot{unresolved: rec})
	}

	f, err := output.ResolveFormat(format, output.FormatNDJSON)
	if err != nil {
		return err
	}

	if f == output.FormatNDJSON {
		nw := output.NewNDJSONWriter(w)
		for _, s := range slots {
			item := s.record
			if s.unresolved != nil {
				item = map[string]any{"@unresolved": s.unresolved}
			}
			if werr := nw.WriteItem(item); werr != nil {
				return werr
			}
		}
		return nil
	}

	data := make([]any, 0, len(slots))
	var unresolved []any
	for _, s := range slots {
		if s.unresolved != nil {
			unresolved = append(unresolved, s.unresolved)
			continue
		}
		data = append(data, s.record)
	}
	env := map[string]any{"data": data}
	if len(unresolved) > 0 {
		env["@unresolved"] = unresolved
	}
	return output.Print(w, env, f, nil)
}

// classifyResolveErr maps a resolver error to either an item-level unresolved
// record (continue the batch) or a command-level fatal (fatal=true → the caller
// bubbles the original error to the single sink). See EntityGet for the rules.
func classifyResolveErr(id string, err error) (rec *unresolvedRecord, fatal bool) {
	var apiErr *output.Error
	if !output.As(err, &apiErr) {
		return nil, true // unclassified → safest to treat as a real failure
	}
	switch apiErr.FixableBy {
	case output.FixableByAgent:
		return &unresolvedRecord{
			ID:        id,
			Reason:    apiErr.Message,
			FixableBy: string(output.FixableByAgent),
			Hint:      apiErr.Hint,
		}, false
	case output.FixableByRetry:
		// A rate-limit (429) carries a retry-after and is a per-item miss; a
		// bare retryable (network/5xx) is a command-level failure of the run.
		if apiErr.RetryAfterSeconds > 0 {
			return &unresolvedRecord{
				ID:                id,
				Reason:            apiErr.Message,
				FixableBy:         string(output.FixableByRetry),
				Hint:              apiErr.Hint,
				RetryAfterSeconds: apiErr.RetryAfterSeconds,
			}, false
		}
		return nil, true
	default: // FixableByHuman, or any other classification → command-level
		return nil, true
	}
}
