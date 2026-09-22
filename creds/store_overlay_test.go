package creds

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shhac/lib-agent-cli/internal/jsondoc"
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
	doc, err := jsondoc.Decode(data)
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

// Overlay tolerates a document it cannot PARSE, which has nothing left to
// preserve, but not one it cannot READ: writing the struct's view over that
// would delete every note and unknown key it holds.
func TestOverlayRefusesToReplaceAnUnreadableDocument(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits do not make a file unreadable on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads a file whatever its mode")
	}
	const doc = `{"//user_note": "who we act as", "user": "ada", "stray": 1}`
	s := writeStore(t, doc)
	if err := os.Chmod(s.Path, 0o200); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(testConfig{User: "grace"}); err == nil {
		t.Error("a save over a document it could not read must fail")
	}
	if err := os.Chmod(s.Path, 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != doc {
		t.Errorf("the unreadable document was replaced:\n%s", data)
	}
}
