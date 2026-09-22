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
	  "//core_note": "the people who review everything",
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
	if groups["//core_note"] == nil {
		t.Errorf("a note among the entries is not an entry the struct dropped: %+v", groups)
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

// RawDelete rewrites the whole document, so it must not also be laid over the
// file it came from: under a type with no schema fields every stored key is
// one nothing owns, which would restore the key just removed.
func TestRawDeleteRemovesTheKeyAndKeepsTheRest(t *testing.T) {
	s := writeStore(t, `{"//user_note": "n", "user": "ada", "retired": {"percent": 30}, "other": 1}`)

	removed, err := s.RawDelete("retired")
	if err != nil || !removed {
		t.Fatalf("RawDelete = %v, %v; want removed", removed, err)
	}
	if _, found := s.RawValue("retired"); found {
		t.Error("the key survived its own deletion")
	}
	for _, kept := range []string{"//user_note", "user", "other"} {
		if _, found := s.RawValue(kept); !found {
			t.Errorf("%q did not survive an unrelated delete", kept)
		}
	}
}

func TestRawDeleteReportsAMissingKey(t *testing.T) {
	s := writeStore(t, `{"user": "ada"}`)
	removed, err := s.RawDelete("nothing.here")
	if err != nil || removed {
		t.Errorf("RawDelete = %v, %v; want not-removed and no error", removed, err)
	}
}

func TestRawValueReachesNestedPaths(t *testing.T) {
	s := writeStore(t, `{"engine": {"bin": "codex", "limits": {"percent": 30}}}`)
	if v, ok := s.RawValue("engine.bin"); !ok || v != "codex" {
		t.Errorf("RawValue = %q, %v; want codex", v, ok)
	}
	if v, ok := s.RawValue("engine.limits"); !ok || !strings.Contains(v, "30") {
		t.Errorf("RawValue of an object = %q, %v; want its JSON", v, ok)
	}
	if _, ok := s.RawValue("engine.bin.deeper"); ok {
		t.Error("a path through a scalar must report missing, not panic")
	}
}

// The walk stops at the outermost key it cannot place, so a whole retired
// section reports once with its contents rather than once per leaf.
func TestUnknownKeysReportsWhatTheSchemaLacks(t *testing.T) {
	s := writeStore(t, `{
	  "//why": "an annotation, not a finding",
	  "user": "ada",
	  "engine": {"bin": "codex", "turbo": true},
	  "groups": {"core": {"review": "approve", "vibes": "good"}},
	  "retired": {"percent": 30, "other": 1}
	}`)
	var paths []string
	for _, k := range s.UnknownKeys(testConfig{}) {
		paths = append(paths, k.Path)
	}
	want := []string{"engine.turbo", "groups.core.vibes", "retired"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Errorf("UnknownKeys = %v, want %v", paths, want)
	}
	for _, k := range s.UnknownKeys(testConfig{}) {
		if k.Path == "retired" && !strings.Contains(k.Value, "30") {
			t.Errorf("a retired section must report what it held, got %q", k.Value)
		}
	}
}

// An annotated config must not warn about its own annotations, and a file
// nobody can parse is a different complaint.
func TestUnknownKeysToleratesCorruptAndIgnoresNotes(t *testing.T) {
	if got := writeStore(t, "{not json").UnknownKeys(testConfig{}); got != nil {
		t.Errorf("a corrupt document must report nothing, got %v", got)
	}
	s := writeStore(t, `{"//user_note": "n", "//why": "c", "user": "ada"}`)
	if got := s.UnknownKeys(&testConfig{}); len(got) != 0 {
		t.Errorf("annotations are not unknown keys: %v", got)
	}
}

type limits struct {
	Percent int `json:"percent,omitempty"`
}

type base struct {
	Limits *limits `json:"limits,omitempty"`
}

// ptrConfig reaches every level through a shape testConfig does not: a
// pointer field, a map of pointers, and a field promoted from an embedded
// struct.
type ptrConfig struct {
	base
	Engine *engine            `json:"engine,omitempty"`
	Groups map[string]*cohort `json:"groups,omitempty"`
}

// The schema is found through pointers and embedding exactly as it is through
// plain fields, so every rule holds one level down whatever the shape.
func TestOverlayDescendsPointersAndEmbeddedStructs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		doc   string
		value ptrConfig
		path  string
		want  any // nil means the key must be absent
	}{
		{
			name:  "unknown key under a pointer field survives",
			doc:   `{"engine": {"bin": "codex", "turbo": true}}`,
			value: ptrConfig{Engine: &engine{Bin: "codex"}},
			path:  "engine.turbo", want: true,
		},
		{
			name:  "unknown key inside a pointer map entry survives",
			doc:   `{"groups": {"core": {"review": "approve", "vibes": "good"}}}`,
			value: ptrConfig{Groups: map[string]*cohort{"core": {Review: "approve"}}},
			path:  "groups.core.vibes", want: "good",
		},
		{
			name:  "unknown key under a promoted field survives",
			doc:   `{"limits": {"percent": 30, "window": "5h"}}`,
			value: ptrConfig{base: base{Limits: &limits{Percent: 30}}},
			path:  "limits.window", want: "5h",
		},
		{
			name:  "cleared field under a pointer is removed",
			doc:   `{"engine": {"bin": "codex", "model": "opus"}}`,
			value: ptrConfig{Engine: &engine{Bin: "codex"}},
			path:  "engine.model",
		},
		{
			name:  "cleared field inside a pointer map entry is removed",
			doc:   `{"groups": {"core": {"review": "approve", "vibes": "good"}}}`,
			value: ptrConfig{Groups: map[string]*cohort{"core": {}}},
			path:  "groups.core.review",
		},
		{
			name:  "cleared field under a promoted field is removed",
			doc:   `{"limits": {"percent": 30, "window": "5h"}}`,
			value: ptrConfig{base: base{Limits: &limits{}}},
			path:  "limits.percent",
		},
		{
			name:  "stored scalar replaced by an object",
			doc:   `{"engine": "codex"}`,
			value: ptrConfig{Engine: &engine{Bin: "codex"}},
			path:  "engine.bin", want: "codex",
		},
		{
			name:  "stored scalar entry replaced by an object",
			doc:   `{"groups": {"core": "approve"}}`,
			value: ptrConfig{Groups: map[string]*cohort{"core": {Review: "approve"}}},
			path:  "groups.core.review", want: "approve",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := writeStore(t, tc.doc)
			if err := s.Save(tc.value); err != nil {
				t.Fatal(err)
			}
			doc := reload(t, s)
			got, found := lookupPath(doc, strings.Split(tc.path, "."))
			if tc.want == nil {
				if found {
					t.Errorf("%s = %v, want it removed: %+v", tc.path, got, doc)
				}
				return
			}
			if got != tc.want {
				t.Errorf("%s = %v, want %v: %+v", tc.path, got, tc.want, doc)
			}
		})
	}
}

func TestUnknownKeysDescendsPointersAndEmbeddedStructs(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want []string
	}{
		{
			name: "pointer field",
			doc:  `{"engine": {"bin": "codex", "turbo": true}}`,
			want: []string{"engine.turbo"},
		},
		{
			name: "pointer map entry",
			doc:  `{"groups": {"core": {"review": "approve", "vibes": "good"}}}`,
			want: []string{"groups.core.vibes"},
		},
		{
			name: "promoted field",
			doc:  `{"limits": {"percent": 30, "window": "5h"}, "retired": 1}`,
			want: []string{"limits.window", "retired"},
		},
		{
			name: "nothing unknown",
			doc:  `{"limits": {"percent": 30}, "engine": {"bin": "codex"}, "groups": {"core": {}}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var paths []string
			for _, k := range writeStore(t, tc.doc).UnknownKeys(&ptrConfig{}) {
				paths = append(paths, k.Path)
			}
			if strings.Join(paths, ",") != strings.Join(tc.want, ",") {
				t.Errorf("UnknownKeys = %v, want %v", paths, tc.want)
			}
		})
	}
}

// A map whose element is not a struct is free-form data: overlay owns each
// entry whole, so nothing inside one is a key the schema failed to model.
func TestUnknownKeysLeavesFreeFormMapsAlone(t *testing.T) {
	type freeForm struct {
		Labels map[string]any               `json:"labels,omitempty"`
		Env    map[string]map[string]string `json:"env,omitempty"`
	}
	s := writeStore(t, `{
	  "labels": {"team": {"name": "core", "size": 4}, "tier": "gold"},
	  "env": {"prod": {"REGION": "eu"}, "dev": {"REGION": "us", "DEBUG": "1"}},
	  "retired": 1
	}`)
	var paths []string
	for _, k := range s.UnknownKeys(freeForm{}) {
		paths = append(paths, k.Path)
	}
	if want := []string{"retired"}; strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Errorf("UnknownKeys = %v, want %v", paths, want)
	}
}
