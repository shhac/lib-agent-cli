package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shhac/lib-agent-cli/creds"
	"github.com/spf13/cobra"
)

type docConfig struct {
	User  string `json:"user,omitempty"`
	Depth int    `json:"depth,omitempty"` // modelled, deliberately not registered
}

// docFixture writes a document holding one registered key and one the schema
// does not model, and returns a config command over it.
func docFixture(t *testing.T, doc string) (*cobra.Command, creds.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	store := creds.Store{Path: path, Overlay: true}
	keys := []ConfigKey{{
		Name: "user",
		Get: func() (string, bool) {
			var c docConfig
			_ = store.Load(&c)
			return c.User, c.User != ""
		},
		Set: func(v string) error {
			var c docConfig
			return store.Update(&c, func() error { c.User = v; return nil })
		},
		Unset: func() error {
			var c docConfig
			return store.Update(&c, func() error { c.User = ""; return nil })
		},
	}}
	return ConfigCommand(nil, keys, WithDocument(store, docConfig{})), store
}

func runCmd(t *testing.T, cmd *cobra.Command, args ...string) (map[string]any, error) {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err != nil {
		return nil, err
	}
	var rec map[string]any
	if body := strings.TrimSpace(out.String()); body != "" {
		if jsonErr := json.Unmarshal([]byte(body), &rec); jsonErr != nil {
			t.Fatalf("unparseable output %q: %v", body, jsonErr)
		}
	}
	return rec, nil
}

// The key somebody was told about and cannot otherwise look at.
func TestWithDocumentGetsAKeyTheRegistryLacks(t *testing.T) {
	cmd, _ := docFixture(t, `{"user": "ada", "retired": {"percent": 30}}`)
	rec, err := runCmd(t, cmd, "get", "retired")
	if err != nil {
		t.Fatal(err)
	}
	if rec["known_key"] != false {
		t.Errorf("the record must mark this as outside the schema: %+v", rec)
	}
	if v, _ := rec["value"].(string); !strings.Contains(v, "30") {
		t.Errorf("value = %q, want what the document holds", v)
	}
}

// Removing it must leave the rest of the file, annotations included, alone.
func TestWithDocumentUnsetsOnlyThatKey(t *testing.T) {
	cmd, store := docFixture(t, `{"//user_note": "who we act as", "user": "ada", "retired": 1}`)
	rec, err := runCmd(t, cmd, "unset", "retired")
	if err != nil {
		t.Fatal(err)
	}
	if rec["unset"] != true || rec["known_key"] != false {
		t.Errorf("record = %+v, want unset and known_key false", rec)
	}
	if _, found := store.RawValue("retired"); found {
		t.Error("the key is still in the document")
	}
	if note, found := store.RawValue("//user_note"); !found || note == "" {
		t.Error("removing an unknown key must not take the annotations with it")
	}
	if user, _ := store.RawValue("user"); user != "ada" {
		t.Error("removing an unknown key must not disturb a registered one")
	}
}

// A registered key keeps the library's exact path: same record shape, no
// known_key marker, and the write still goes through the struct.
func TestWithDocumentLeavesRegisteredKeysAlone(t *testing.T) {
	cmd, _ := docFixture(t, `{"user": "ada", "retired": 1}`)
	rec, err := runCmd(t, cmd, "get", "user")
	if err != nil {
		t.Fatal(err)
	}
	if _, marked := rec["known_key"]; marked {
		t.Errorf("a registered key must not be marked: %+v", rec)
	}
	if rec["value"] != "ada" {
		t.Errorf("record = %+v, want the registered key's value", rec)
	}
}

// A typo is in neither the registry nor the document, and needs the library's
// error with its list of valid names — not a report that it is unset.
func TestWithDocumentStillErrorsOnATypo(t *testing.T) {
	cmd, _ := docFixture(t, `{"user": "ada"}`)
	if _, err := runCmd(t, cmd, "get", "usr"); err == nil {
		t.Error("a key in neither the registry nor the document must error")
	}
}

// Without the option nothing changes, which is what makes it safe to add to a
// library every CLI already calls.
func TestWithoutDocumentTheRegistryIsTheWholeAnswer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"retired": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := ConfigCommand(nil, []ConfigKey{{Name: "user", Get: func() (string, bool) { return "", false }}})
	if _, err := runCmd(t, cmd, "get", "retired"); err == nil {
		t.Error("an unregistered key must error when the fallback is not enabled")
	}
}

// set stays registry-only: writing a key nothing reads would recreate the
// state the fallback exists to clear.
func TestWithDocumentDoesNotExtendSet(t *testing.T) {
	cmd, store := docFixture(t, `{"retired": 1}`)
	if _, err := runCmd(t, cmd, "set", "retired", "2"); err == nil {
		t.Error("set on an unregistered key must fail even when the document holds it")
	}
	if v, _ := store.RawValue("retired"); v != "1" {
		t.Errorf("the refused set must not have written: %q", v)
	}
}

func TestSectionKeyRefusesSetAndClearsOnUnset(t *testing.T) {
	cleared := false
	key := SectionKey("engine.limits", "the engine's limits",
		func() (string, bool) { return "5h=30", true },
		func() error { cleared = true; return nil })

	if err := key.Set("20"); err == nil {
		t.Error("a section is not a value, so set must fail")
	} else if !strings.Contains(err.Error(), "section") {
		t.Errorf("error = %q, want it to say the key is a section", err)
	}
	if err := key.Unset(); err != nil || !cleared {
		t.Errorf("unset must clear the section: cleared=%v err=%v", cleared, err)
	}
}

// known_key is the schema's answer, not the registry's: a key the struct has
// a field for is in effect even when nothing registers it, and only a path the
// schema cannot place (or anything inside one) is reported as unknown.
func TestWithDocumentKnownKeyFollowsTheSchema(t *testing.T) {
	const doc = `{"user": "ada", "depth": 3, "retired": {"percent": 30}}`
	for _, tc := range []struct {
		verb, key string
		known     bool
	}{
		{"get", "depth", true},
		{"get", "retired", false},
		{"get", "retired.percent", false},
		{"unset", "depth", true},
		{"unset", "retired", false},
	} {
		t.Run(tc.verb+" "+tc.key, func(t *testing.T) {
			cmd, _ := docFixture(t, doc)
			rec, err := runCmd(t, cmd, tc.verb, tc.key)
			if err != nil {
				t.Fatal(err)
			}
			if rec["known_key"] != tc.known {
				t.Errorf("known_key = %v, want %v: %+v", rec["known_key"], tc.known, rec)
			}
		})
	}
}
