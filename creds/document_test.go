package creds

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type engine struct {
	Bin   string `json:"bin,omitempty"`
	Model string `json:"model,omitempty"`
}

type cohort struct {
	Review string `json:"review,omitempty"`
}

type testConfig struct {
	User   string            `json:"user,omitempty"`
	Repos  []string          `json:"repos,omitempty"`
	Engine engine            `json:"engine,omitempty"`
	Groups map[string]cohort `json:"groups,omitempty"`
}

// writeStore puts a document on disk and returns an overlay Store over it.
func writeStore(t *testing.T, doc string) Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if doc != "" {
		if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return Store{Path: path, Overlay: true}
}

func reload(t *testing.T, s Store) map[string]any {
	t.Helper()
	data, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := decodeDoc(data)
	if err != nil {
		t.Fatalf("wrote unparseable JSON: %v\n%s", err, data)
	}
	return doc
}

// The bug the whole feature exists for: a config that documents itself loses
// that documentation the first time any unrelated setting is saved.
func TestOverlayKeepsAnnotationsAndUnknownKeys(t *testing.T) {
	s := writeStore(t, `{
	  "//why": "commentary on the whole file",
	  "//user_note": "who the tool acts as",
	  "user": "ada",
	  "engine": {"bin": "codex", "turbo": true},
	  "from_a_newer_release": 7
	}`)
	if err := s.Save(testConfig{User: "grace", Engine: engine{Bin: "codex"}}); err != nil {
		t.Fatal(err)
	}

	doc := reload(t, s)
	if doc["user"] != "grace" {
		t.Errorf("the edit did not land: %+v", doc)
	}
	if doc["//why"] == nil || doc["//user_note"] == nil {
		t.Errorf("annotations did not survive the write: %+v", doc)
	}
	if doc["from_a_newer_release"] != float64(7) {
		t.Errorf("a key from another version was dropped: %+v", doc)
	}
	if eng := doc["engine"].(map[string]any); eng["turbo"] != true {
		t.Errorf("a nested unknown key was dropped: %+v", eng)
	}
}

// The other half of the rule, and what keeps unset working: a key the schema
// OWNS but the value no longer states was cleared, and must go.
func TestOverlayRemovesWhatTheStructCleared(t *testing.T) {
	s := writeStore(t, `{"user": "ada", "//user_note": "kept", "engine": {"bin": "codex", "model": "opus"}}`)
	if err := s.Save(testConfig{Engine: engine{Bin: "codex"}}); err != nil {
		t.Fatal(err)
	}

	doc := reload(t, s)
	if _, present := doc["user"]; present {
		t.Error("a cleared field must be removed, not preserved")
	}
	if doc["//user_note"] == nil {
		t.Error("the note documenting a reset dial should outlive the reset")
	}
	if eng := doc["engine"].(map[string]any); eng["model"] != nil {
		t.Errorf("a cleared nested field must be removed: %+v", eng)
	}
}

// Map keys are data, not schema, so the struct owns which entries exist —
// but not what is inside one.
func TestOverlayOwnsMapEntriesButNotTheirContents(t *testing.T) {
	s := writeStore(t, `{"groups": {
	  "core": {"review": "approve", "vibes": "good"},
	  "gone": {"review": "comment"}
	}}`)
	if err := s.Save(testConfig{Groups: map[string]cohort{"core": {Review: "approve"}}}); err != nil {
		t.Fatal(err)
	}

	groups := reload(t, s)["groups"].(map[string]any)
	if _, present := groups["gone"]; present {
		t.Error("an entry the struct no longer lists must not survive")
	}
	if core := groups["core"].(map[string]any); core["vibes"] != "good" {
		t.Errorf("an unknown key inside an entry must survive: %+v", core)
	}
}

// Off by default, so every existing caller — every credential store — keeps
// the behaviour it has today.
func TestOverlayIsOptIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"//note": "x", "stray": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	plain := Store{Path: path}
	if err := plain.Save(testConfig{User: "ada"}); err != nil {
		t.Fatal(err)
	}
	doc := reload(t, plain)
	if _, present := doc["//note"]; present {
		t.Error("a plain Store must keep replacing the document wholesale")
	}
	if _, present := doc["stray"]; present {
		t.Error("a plain Store must not start preserving unknown keys")
	}
}

func TestOverlayLaysNotesBesideTheirKeys(t *testing.T) {
	s := writeStore(t, `{"//why": "c", "//user_note": "n", "user": "ada", "repos": ["o/r"]}`)
	if err := s.Save(testConfig{User: "ada", Repos: []string{"o/r"}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	// Free-standing commentary leads; a pinned note sits immediately before
	// its key, with "repos" sorting between them by name and not between the
	// pair.
	for _, pair := range [][2]string{{`"//why"`, `"repos"`}, {`"repos"`, `"//user_note"`}, {`"//user_note"`, `"user"`}} {
		if strings.Index(text, pair[0]) > strings.Index(text, pair[1]) {
			t.Errorf("%s must precede %s:\n%s", pair[0], pair[1], text)
		}
	}
}

// Order is a pure function of the key set, so an unrelated command never
// churns the file.
func TestOverlayWritesAreStable(t *testing.T) {
	s := writeStore(t, `{"//user_note": "n", "user": "ada", "engine": {"bin": "codex"}}`)
	cfg := testConfig{User: "ada", Engine: engine{Bin: "codex"}}
	var passes []string
	for range 3 {
		if err := s.Save(cfg); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(s.Path)
		if err != nil {
			t.Fatal(err)
		}
		passes = append(passes, string(data))
	}
	if passes[0] != passes[1] || passes[1] != passes[2] {
		t.Errorf("repeated saves of the same value changed the file:\n%s\nvs\n%s", passes[0], passes[2])
	}
}

// A corrupt file must not also be an unwritable one.
func TestOverlayToleratesACorruptDocument(t *testing.T) {
	s := writeStore(t, "{not json")
	if err := s.Save(testConfig{User: "ada"}); err != nil {
		t.Fatal(err)
	}
	if doc := reload(t, s); doc["user"] != "ada" {
		t.Errorf("a corrupt document must not block the write: %+v", doc)
	}
}

// Update is the locked read-modify-write, and must honour Overlay too — it is
// the path a CLI's `config set` actually takes.
func TestOverlayAppliesThroughUpdate(t *testing.T) {
	s := writeStore(t, `{"//user_note": "n", "user": "ada", "stray": 1}`)
	var cfg testConfig
	if err := s.Update(&cfg, func() error { cfg.User = "grace"; return nil }); err != nil {
		t.Fatal(err)
	}
	doc := reload(t, s)
	if doc["user"] != "grace" || doc["//user_note"] == nil || doc["stray"] != float64(1) {
		t.Errorf("Update must preserve what Save does: %+v", doc)
	}
}

func TestPinnedToNeedsASibling(t *testing.T) {
	obj := map[string]any{"model": "opus", "//model_note": "n", "//why": "c", "//bin": "n", "bin": "codex"}
	for _, tc := range []struct {
		key, want string
		pinned    bool
	}{
		{"//model_note", "model", true},
		{"//bin", "bin", true}, // the suffix is optional
		{"//why", "", false},   // no sibling "why"
		{"model", "", false},
	} {
		got, ok := pinnedTo(tc.key, obj)
		if ok != tc.pinned || got != tc.want {
			t.Errorf("pinnedTo(%q) = %q, %v; want %q, %v", tc.key, got, ok, tc.want, tc.pinned)
		}
	}
}

func TestJSONFieldsFlattensEmbeddedStructs(t *testing.T) {
	type inner struct {
		A string `json:"a"`
	}
	type outer struct {
		inner
		B string `json:"b"`
		C string `json:"-"`
	}
	fields := jsonFields(reflect.TypeOf(outer{}))
	if _, ok := fields["a"]; !ok {
		t.Error("a promoted field must be spelled at the level the document holds it")
	}
	if _, ok := fields["c"]; ok {
		t.Error(`a json:"-" field is not part of the schema`)
	}
	if len(fields) != 2 {
		t.Errorf("fields = %v, want a and b", fields)
	}
}

// The ordered emit must survive encoding/json's own map sorting, including
// inside arrays.
func TestOrderedSurvivesMarshalling(t *testing.T) {
	doc := map[string]any{
		"z": 1,
		"a": []any{map[string]any{"q": 1, "//q_note": "n"}},
	}
	data, err := json.MarshalIndent(ordered(doc), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Index(text, `"a"`) > strings.Index(text, `"z"`) {
		t.Errorf("keys are not in sort order:\n%s", text)
	}
	if strings.Index(text, `"//q_note"`) > strings.Index(text, `"q"`) {
		t.Errorf("a note inside an array element must precede its key:\n%s", text)
	}
}
