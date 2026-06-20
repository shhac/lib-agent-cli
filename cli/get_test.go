package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	output "github.com/shhac/lib-agent-output"
)

// resolverFrom builds a resolver from a fixed map; a missing key returns the
// supplied error (or a default not-found if errs has no entry).
func resolverFrom(recs map[string]any, errs map[string]error) func(string) (any, error) {
	return func(id string) (any, error) {
		if v, ok := recs[id]; ok {
			return v, nil
		}
		if e, ok := errs[id]; ok {
			return nil, e
		}
		return nil, output.New("not found", output.FixableByAgent)
	}
}

// lines splits NDJSON output into parsed objects.
func lines(t *testing.T, b []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if ln == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("line not JSON: %q (%v)", ln, err)
		}
		out = append(out, m)
	}
	return out
}

func TestEntityGet_SingleOK(t *testing.T) {
	var buf bytes.Buffer
	err := EntityGet(&buf, "", []string{"a"}, resolverFrom(map[string]any{"a": map[string]any{"id": "a"}}, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := lines(t, buf.Bytes())
	if len(got) != 1 || got[0]["id"] != "a" {
		t.Fatalf("want one record for a, got %v", got)
	}
}

func TestEntityGet_SingleMiss_IsUnresolvedNotError(t *testing.T) {
	var buf bytes.Buffer
	// A sole miss must NOT be a command error: nil return (exit 0) + an
	// @unresolved line on stdout.
	err := EntityGet(&buf, "", []string{"x"}, resolverFrom(nil, nil))
	if err != nil {
		t.Fatalf("single miss must return nil (exit 0), got %v", err)
	}
	got := lines(t, buf.Bytes())
	if len(got) != 1 || got[0]["@unresolved"] == nil {
		t.Fatalf("want one @unresolved line, got %v", got)
	}
	u := got[0]["@unresolved"].(map[string]any)
	if u["id"] != "x" || u["fixable_by"] != "agent" {
		t.Fatalf("unresolved record wrong: %v", u)
	}
}

func TestEntityGet_MixedPreservesInputOrder(t *testing.T) {
	var buf bytes.Buffer
	recs := map[string]any{"a": map[string]any{"id": "a"}, "c": map[string]any{"id": "c"}}
	err := EntityGet(&buf, "", []string{"a", "b", "c"}, resolverFrom(recs, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := lines(t, buf.Bytes())
	if len(got) != 3 {
		t.Fatalf("want 3 lines, got %d: %v", len(got), got)
	}
	if got[0]["id"] != "a" {
		t.Errorf("line0 should be record a, got %v", got[0])
	}
	if u, ok := got[1]["@unresolved"].(map[string]any); !ok || u["id"] != "b" {
		t.Errorf("line1 should be @unresolved b, got %v", got[1])
	}
	if got[2]["id"] != "c" {
		t.Errorf("line2 should be record c, got %v", got[2])
	}
}

func TestEntityGet_CommandLevelFailureAborts(t *testing.T) {
	var buf bytes.Buffer
	// A human-fixable error (auth) is command-level: EntityGet returns it and
	// writes NOTHING (single-sink: stderr+exit1 handled by the caller).
	authErr := output.New("not authenticated", output.FixableByHuman)
	err := EntityGet(&buf, "", []string{"a", "b"}, resolverFrom(
		map[string]any{"a": map[string]any{"id": "a"}},
		map[string]error{"b": authErr},
	))
	if err == nil {
		t.Fatal("command-level failure must return an error")
	}
	if buf.Len() != 0 {
		t.Fatalf("stdout must be empty on command-level failure, got %q", buf.String())
	}
}

func TestEntityGet_RateLimitIsItemLevel_NetworkIsFatal(t *testing.T) {
	// 429 (retry + retry-after) → item-level @unresolved.
	rate := output.New("rate limited", output.FixableByRetry).WithRetryAfter(3 * time.Second)
	var buf bytes.Buffer
	if err := EntityGet(&buf, "", []string{"x"}, resolverFrom(nil, map[string]error{"x": rate})); err != nil {
		t.Fatalf("429 should be item-level (nil error), got %v", err)
	}
	u := lines(t, buf.Bytes())[0]["@unresolved"].(map[string]any)
	if u["fixable_by"] != "retry" || u["retry_after_seconds"] == nil {
		t.Fatalf("429 unresolved should carry retry_after_seconds: %v", u)
	}

	// Bare retryable (network/5xx, no retry-after) → command-level fatal.
	netErr := output.New("connection reset", output.FixableByRetry)
	var buf2 bytes.Buffer
	if err := EntityGet(&buf2, "", []string{"x"}, resolverFrom(nil, map[string]error{"x": netErr})); err == nil {
		t.Fatal("network/5xx retryable should be command-level fatal")
	}
	if buf2.Len() != 0 {
		t.Fatalf("stdout must be empty on fatal, got %q", buf2.String())
	}
}

func TestEntityGet_JSONFormatEnvelope(t *testing.T) {
	var buf bytes.Buffer
	recs := map[string]any{"a": map[string]any{"id": "a"}}
	err := EntityGet(&buf, "json", []string{"a", "b"}, resolverFrom(recs, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("json format should be one document: %v\n%s", err, buf.String())
	}
	data, ok := env["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("want data:[1 record], got %v", env["data"])
	}
	un, ok := env["@unresolved"].([]any)
	if !ok || len(un) != 1 {
		t.Fatalf("want @unresolved:[1], got %v", env["@unresolved"])
	}
}
