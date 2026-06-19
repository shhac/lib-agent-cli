package cli

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func findSub(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := fn()
	_ = w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b), err
}

func assertAgentErr(t *testing.T, err error) {
	t.Helper()
	var oe *output.Error
	if err == nil || !output.As(err, &oe) || oe.FixableBy != output.FixableByAgent {
		t.Errorf("want fixable_by:agent error, got %v", err)
	}
}

func TestConfigCommand(t *testing.T) {
	store := map[string]string{}
	keys := []ConfigKey{{
		Name:        "k",
		Description: "a key",
		Get:         func() (string, bool) { v, ok := store["k"]; return v, ok },
		Set: func(v string) error {
			if v == "bad" {
				return errors.New("bad value")
			}
			store["k"] = v
			return nil
		},
		Unset: func() error { delete(store, "k"); return nil },
	}}
	cmd := ConfigCommand(keys)
	get, set, unset, list := findSub(cmd, "get"), findSub(cmd, "set"), findSub(cmd, "unset"), findSub(cmd, "list")

	out, err := captureStdout(t, func() error { return set.RunE(set, []string{"k", "v1"}) })
	if err != nil || store["k"] != "v1" || !strings.Contains(out, "v1") {
		t.Fatalf("set: out=%q err=%v store=%v", out, err, store)
	}

	out, err = captureStdout(t, func() error { return get.RunE(get, []string{"k"}) })
	if err != nil || !strings.Contains(out, `"value":"v1"`) {
		t.Errorf("get: out=%q err=%v", out, err)
	}

	out, err = captureStdout(t, func() error { return list.RunE(list, nil) })
	if err != nil || !strings.Contains(out, `"key":"k"`) || !strings.Contains(out, "a key") {
		t.Errorf("list: out=%q", out)
	}

	// validation failure and unknown keys are fixable_by:agent and write nothing.
	assertAgentErr(t, set.RunE(set, []string{"k", "bad"}))
	assertAgentErr(t, get.RunE(get, []string{"nope"}))
	assertAgentErr(t, set.RunE(set, []string{"nope", "x"}))
	assertAgentErr(t, unset.RunE(unset, []string{"nope"}))

	if _, err := captureStdout(t, func() error { return unset.RunE(unset, []string{"k"}) }); err != nil {
		t.Fatal(err)
	}
	if _, ok := store["k"]; ok {
		t.Error("unset should have cleared the key")
	}
}
