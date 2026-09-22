package jsondoc

import (
	"encoding/json"
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

func decode(t *testing.T, doc string) map[string]any {
	t.Helper()
	out, err := Decode([]byte(doc))
	if err != nil {
		t.Fatalf("unparseable JSON: %v\n%s", err, doc)
	}
	return out
}

// encoded is the bytes a write of v over stored would produce.
func encoded(t *testing.T, stored string, v any) string {
	t.Helper()
	var doc map[string]any
	if stored != "" {
		doc = decode(t, stored)
	}
	out, err := Overlay(doc, v)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// overlaid is the document a write of v over stored would leave behind.
func overlaid(t *testing.T, stored string, v any) map[string]any {
	t.Helper()
	return decode(t, encoded(t, stored, v))
}

// The other half of the rule, and what keeps unset working: a key the schema
// OWNS but the value no longer states was cleared, and must go.
func TestOverlayRemovesWhatTheStructCleared(t *testing.T) {
	doc := overlaid(t, `{"user": "ada", "//user_note": "kept", "engine": {"bin": "codex", "model": "opus"}}`,
		testConfig{Engine: engine{Bin: "codex"}})

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
	doc := overlaid(t, `{"groups": {
	  "//core_note": "the people who review everything",
	  "core": {"review": "approve", "vibes": "good"},
	  "gone": {"review": "comment"}
	}}`, testConfig{Groups: map[string]cohort{"core": {Review: "approve"}}})

	groups := doc["groups"].(map[string]any)
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

func TestOverlayLaysNotesBesideTheirKeys(t *testing.T) {
	text := encoded(t, `{"//why": "c", "//user_note": "n", "user": "ada", "repos": ["o/r"]}`,
		testConfig{User: "ada", Repos: []string{"o/r"}})
	// Free-standing commentary leads; a pinned note sits immediately before
	// its key, with "repos" sorting between them by name and not between the
	// pair.
	for _, pair := range [][2]string{{`"//why"`, `"repos"`}, {`"repos"`, `"//user_note"`}, {`"//user_note"`, `"user"`}} {
		if strings.Index(text, pair[0]) > strings.Index(text, pair[1]) {
			t.Errorf("%s must precede %s:\n%s", pair[0], pair[1], text)
		}
	}
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
			doc := overlaid(t, tc.doc, tc.value)
			got, found := Lookup(doc, tc.path)
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
	data, err := json.MarshalIndent(Ordered(doc), "", "  ")
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

func TestLookupAndDeleteWalkDottedPaths(t *testing.T) {
	doc := decode(t, `{"engine": {"bin": "codex", "limits": {"percent": 30}}}`)
	if v, ok := Lookup(doc, "engine.limits.percent"); !ok || v != float64(30) {
		t.Errorf("Lookup = %v, %v; want 30", v, ok)
	}
	if _, ok := Lookup(doc, "engine.bin.deeper"); ok {
		t.Error("a path through a scalar must report missing, not panic")
	}
	if Delete(doc, "engine.nothing") {
		t.Error("deleting a missing key must report false")
	}
	if !Delete(doc, "engine.limits.percent") {
		t.Fatal("deleting a present key must report true")
	}
	if limits, ok := Lookup(doc, "engine.limits"); !ok || Render(limits) != "" {
		t.Errorf("a parent emptied by a delete must stay, got %v, %v", limits, ok)
	}
}
