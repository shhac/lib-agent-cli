package cli

import (
	"bytes"
	"errors"
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

func assertAgentErr(t *testing.T, err error) {
	t.Helper()
	var oe *output.Error
	if err == nil || !output.As(err, &oe) || oe.FixableBy != output.FixableByAgent {
		t.Errorf("want fixable_by:agent error, got %v", err)
	}
}

func testConfigKeys(store map[string]string) []ConfigKey {
	return []ConfigKey{{
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
}

func TestConfigCommand(t *testing.T) {
	store := map[string]string{}
	cmd := ConfigCommand(nil, testConfigKeys(store))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	get, set, unset, list := findSub(cmd, "get"), findSub(cmd, "set"), findSub(cmd, "unset"), findSub(cmd, "list")
	run := func(c *cobra.Command, args []string) (string, error) {
		buf.Reset()
		err := c.RunE(c, args)
		return buf.String(), err
	}

	out, err := run(set, []string{"k", "v1"})
	if err != nil || store["k"] != "v1" {
		t.Fatalf("set: out=%q err=%v store=%v", out, err, store)
	}
	if !strings.Contains(out, `"key":"k"`) || !strings.Contains(out, `"value":"v1"`) || !strings.Contains(out, `"set":true`) {
		t.Errorf("set ack should be the {key,value,set} record, got %q", out)
	}

	out, err = run(get, []string{"k"})
	if err != nil || !strings.Contains(out, `"key":"k"`) || !strings.Contains(out, `"value":"v1"`) || !strings.Contains(out, `"set":true`) {
		t.Errorf("get: out=%q err=%v", out, err)
	}

	out, err = run(list, nil)
	if err != nil || !strings.Contains(out, `"key":"k"`) || !strings.Contains(out, "a key") {
		t.Errorf("list: out=%q", out)
	}
	if strings.Contains(out, `"data"`) {
		t.Errorf("NDJSON list should not be enveloped, got %q", out)
	}

	// validation failure and unknown keys are fixable_by:agent and write nothing.
	assertAgentErr(t, set.RunE(set, []string{"k", "bad"}))
	assertAgentErr(t, get.RunE(get, []string{"nope"}))
	assertAgentErr(t, set.RunE(set, []string{"nope", "x"}))
	assertAgentErr(t, unset.RunE(unset, []string{"nope"}))

	out, err = run(unset, []string{"k"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store["k"]; ok {
		t.Error("unset should have cleared the key")
	}
	if !strings.Contains(out, `"set":false`) {
		t.Errorf("unset ack should report the cleared state, got %q", out)
	}
}

func TestConfigCommandHonorsFormat(t *testing.T) {
	store := map[string]string{"k": "v1"}
	g := &Globals{Format: "json"}
	cmd := ConfigCommand(g, testConfigKeys(store))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	get, list := findSub(cmd, "get"), findSub(cmd, "list")

	if err := get.RunE(get, []string{"k"}); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "\n  ") || !strings.Contains(out, `"value": "v1"`) {
		t.Errorf("json get should pretty-print the bare object, got %q", out)
	}

	buf.Reset()
	if err := list.RunE(list, nil); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"data"`) {
		t.Errorf("json list should use the {\"data\":[…]} envelope, got %q", out)
	}

	g.Format = "bogus"
	if err := get.RunE(get, []string{"k"}); err == nil {
		t.Error("bogus format should error")
	}
}
