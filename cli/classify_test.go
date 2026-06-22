package cli

import (
	"errors"
	"testing"
	"time"

	output "github.com/shhac/lib-agent-output"
)

// TestClassifyResolveErr covers the get-batch continue/abort router: which
// per-item resolve failures become an @unresolved record (fatal=false) versus a
// command-level abort (fatal=true). A wrong call silently drops or over-fails
// items, so every classification arm is pinned.
func TestClassifyResolveErr(t *testing.T) {
	t.Run("unclassified is fatal", func(t *testing.T) {
		if rec, fatal := classifyResolveErr("x", errors.New("boom")); rec != nil || !fatal {
			t.Errorf("got rec=%v fatal=%v; want nil,true", rec, fatal)
		}
	})
	t.Run("agent-fixable is a per-item miss", func(t *testing.T) {
		rec, fatal := classifyResolveErr("x", output.New("bad", output.FixableByAgent))
		if rec == nil || fatal || rec.FixableBy != string(output.FixableByAgent) {
			t.Errorf("got rec=%v fatal=%v; want a non-fatal agent record", rec, fatal)
		}
	})
	t.Run("retry with retry-after is a per-item miss", func(t *testing.T) {
		e := output.New("rate", output.FixableByRetry).WithRetryAfter(5 * time.Second)
		rec, fatal := classifyResolveErr("x", e)
		if rec == nil || fatal || rec.RetryAfterSeconds != 5 {
			t.Errorf("got rec=%v fatal=%v; want a non-fatal record with retry_after=5", rec, fatal)
		}
	})
	t.Run("bare retryable is fatal", func(t *testing.T) {
		if rec, fatal := classifyResolveErr("x", output.New("net", output.FixableByRetry)); rec != nil || !fatal {
			t.Errorf("got rec=%v fatal=%v; want nil,true", rec, fatal)
		}
	})
	t.Run("human-fixable is fatal", func(t *testing.T) {
		if rec, fatal := classifyResolveErr("x", output.New("auth", output.FixableByHuman)); rec != nil || !fatal {
			t.Errorf("got rec=%v fatal=%v; want nil,true", rec, fatal)
		}
	})
}
