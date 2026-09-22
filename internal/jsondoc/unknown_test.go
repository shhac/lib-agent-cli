package jsondoc

import (
	"strings"
	"testing"
)

func unknownPaths(t *testing.T, doc string, schema any) []string {
	t.Helper()
	var paths []string
	for _, k := range UnknownKeys(decode(t, doc), schema) {
		paths = append(paths, k.Path)
	}
	return paths
}

// The walk stops at the outermost key it cannot place, so a whole retired
// section reports once with its contents rather than once per leaf.
func TestUnknownKeysReportsWhatTheSchemaLacks(t *testing.T) {
	const doc = `{
	  "//why": "an annotation, not a finding",
	  "user": "ada",
	  "engine": {"bin": "codex", "turbo": true},
	  "groups": {"core": {"review": "approve", "vibes": "good"}},
	  "retired": {"percent": 30, "other": 1}
	}`
	want := []string{"engine.turbo", "groups.core.vibes", "retired"}
	if paths := unknownPaths(t, doc, testConfig{}); strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Errorf("UnknownKeys = %v, want %v", paths, want)
	}
	for _, k := range UnknownKeys(decode(t, doc), testConfig{}) {
		if k.Path == "retired" && !strings.Contains(k.Value, "30") {
			t.Errorf("a retired section must report what it held, got %q", k.Value)
		}
	}
}

// An annotated config must not warn about its own annotations.
func TestUnknownKeysIgnoresNotes(t *testing.T) {
	if got := unknownPaths(t, `{"//user_note": "n", "//why": "c", "user": "ada"}`, &testConfig{}); len(got) != 0 {
		t.Errorf("annotations are not unknown keys: %v", got)
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
			if paths := unknownPaths(t, tc.doc, &ptrConfig{}); strings.Join(paths, ",") != strings.Join(tc.want, ",") {
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
	paths := unknownPaths(t, `{
	  "labels": {"team": {"name": "core", "size": 4}, "tier": "gold"},
	  "env": {"prod": {"REGION": "eu"}, "dev": {"REGION": "us", "DEBUG": "1"}},
	  "retired": 1
	}`, freeForm{})
	if want := []string{"retired"}; strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Errorf("UnknownKeys = %v, want %v", paths, want)
	}
}
