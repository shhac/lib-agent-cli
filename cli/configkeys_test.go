package cli

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shhac/lib-agent-cli/creds"
)

type conn struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type claudeEngine struct {
	Floor *int `json:"floor,omitempty"`
}

type keysConfig struct {
	Name    string `json:"name,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Workers int    `json:"workers,omitempty"`
	Engines struct {
		Claude claudeEngine `json:"claude"`
	} `json:"engines"`
	Enabled *bool  `json:"enabled,omitempty"`
	Conns   []conn `json:"conns,omitempty"`
	Home    string `json:"home,omitempty"`
	KeyEnv  string `json:"key_env,omitempty"`
}

func keysDefault() keysConfig {
	return keysConfig{Mode: "fast", Workers: 4, Home: "/default/home"}
}

// memoryBinding is a binding with no document, so "set" means "differs from
// the default".
func memoryBinding(cfg *keysConfig) ConfigBinding[keysConfig] {
	return ConfigBinding[keysConfig]{
		Read:    func() (keysConfig, error) { return *cfg, nil },
		Update:  func(fn func(*keysConfig) error) error { return fn(cfg) },
		Default: keysDefault,
	}
}

// storeBinding reads defaults-then-file, and writes only what the file already
// holds plus the change, the way a CLI keeps its config sparse.
func storeBinding(t *testing.T, doc string) (ConfigBinding[keysConfig], creds.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if doc != "" {
		if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store := creds.Store{Path: path, Overlay: true}
	return ConfigBinding[keysConfig]{
		Read: func() (keysConfig, error) {
			c := keysDefault()
			return c, store.Load(&c)
		},
		Update: func(fn func(*keysConfig) error) error {
			var c keysConfig
			return store.Update(&c, func() error { return fn(&c) })
		},
		Default: keysDefault,
		Doc:     &store,
	}, store
}

func mustSet(t *testing.T, k ConfigKey, v string) {
	t.Helper()
	if err := k.Set(v); err != nil {
		t.Fatalf("set %s=%q: %v", k.Name, v, err)
	}
}

func assertGet(t *testing.T, k ConfigKey, wantValue string, wantSet bool) {
	t.Helper()
	v, set := k.Get()
	if v != wantValue || set != wantSet {
		t.Errorf("get %s = (%q, %v), want (%q, %v)", k.Name, v, set, wantValue, wantSet)
	}
}

func assertRejects(t *testing.T, k ConfigKey, values ...string) {
	t.Helper()
	for _, v := range values {
		assertAgentErr(t, k.Set(v))
	}
}

func TestIntKeyBoundsAndUnsetToDefault(t *testing.T) {
	cfg := keysDefault()
	k := IntKey(memoryBinding(&cfg), "workers", "", func(c *keysConfig) *int { return &c.Workers }, 1, 10)

	assertRejects(t, k, "abc", "0", "11", "", "2.5")
	if cfg.Workers != 4 {
		t.Fatalf("a rejected value must not be written, got %d", cfg.Workers)
	}
	assertGet(t, k, "4", false)

	mustSet(t, k, "7")
	assertGet(t, k, "7", true)

	// The zero value is out of bounds here, so unset must restore the default.
	if err := k.Unset(); err != nil {
		t.Fatal(err)
	}
	if cfg.Workers != 4 {
		t.Errorf("unset should restore the default 4, got %d", cfg.Workers)
	}
	assertGet(t, k, "4", false)
}

func TestOptionalIntKeyKeepsAnExplicitZero(t *testing.T) {
	cfg := keysDefault()
	k := OptionalIntKey(memoryBinding(&cfg), "floor", "", func(c *keysConfig) **int { return &c.Engines.Claude.Floor }, 0, 100)

	assertRejects(t, k, "-1", "101", "x")
	assertGet(t, k, "", false)

	mustSet(t, k, "0")
	assertGet(t, k, "0", true)

	if err := k.Unset(); err != nil {
		t.Fatal(err)
	}
	if cfg.Engines.Claude.Floor != nil {
		t.Errorf("unset should restore nil, got %d", *cfg.Engines.Claude.Floor)
	}
	assertGet(t, k, "", false)
}

func TestOptionalBoolKey(t *testing.T) {
	cfg := keysDefault()
	k := OptionalBoolKey(memoryBinding(&cfg), "enabled", "", func(c *keysConfig) **bool { return &c.Enabled })

	assertRejects(t, k, "maybe", "")
	if !slices.Equal(k.Values, []string{"true", "false"}) {
		t.Errorf("Values = %v", k.Values)
	}
	mustSet(t, k, "false")
	assertGet(t, k, "false", true)
	if err := k.Unset(); err != nil {
		t.Fatal(err)
	}
	assertGet(t, k, "", false)
}

func TestOneOfKey(t *testing.T) {
	cfg := keysDefault()
	values := []string{"fast", "slow"}
	k := OneOfKey(memoryBinding(&cfg), "mode", "", func(c *keysConfig) *string { return &c.Mode }, values)

	err := k.Set("medium")
	assertAgentErr(t, err)
	if err == nil || !strings.Contains(err.Error(), "fast, slow") {
		t.Errorf("the error should list the valid values, got %v", err)
	}
	assertRejects(t, k, "")
	if !slices.Equal(k.Values, values) {
		t.Errorf("Values = %v, want %v", k.Values, values)
	}

	mustSet(t, k, "slow")
	assertGet(t, k, "slow", true)
	if err := k.Unset(); err != nil {
		t.Fatal(err)
	}
	assertGet(t, k, "fast", false)
}

func TestStringKey(t *testing.T) {
	cfg := keysDefault()
	noSpaces := func(v string) error {
		if strings.Contains(v, " ") {
			return errors.New("no spaces")
		}
		return nil
	}
	k := StringKey(memoryBinding(&cfg), "name", "", func(c *keysConfig) *string { return &c.Name }, noSpaces)

	if err := k.Set("a b"); err == nil {
		t.Error("validate should reject")
	}
	mustSet(t, k, "ab")
	assertGet(t, k, "ab", true)

	unchecked := StringKey(memoryBinding(&cfg), "name", "", func(c *keysConfig) *string { return &c.Name }, nil)
	mustSet(t, unchecked, "a b")
	assertGet(t, unchecked, "a b", true)
}

func TestJSONKeyValidatesByDecoding(t *testing.T) {
	cfg := keysDefault()
	nonEmptyIDs := func(cs []conn) error {
		for _, c := range cs {
			if c.ID == "" {
				return errors.New("every connection needs an id")
			}
		}
		return nil
	}
	k := JSONKey(memoryBinding(&cfg), "conns", "", func(c *keysConfig) *[]conn { return &c.Conns }, nonEmptyIDs)

	assertRejects(t, k,
		`not json`,
		`{"id":"a"}`,                 // an object where the type is an array
		`[{"id":"a","knd":"slack"}]`, // a typo in a nested key
		`[{"id":"a"}] []`,            // trailing data
	)
	if err := k.Set(`[{"id":"","kind":"slack"}]`); err == nil {
		t.Error("validate should see the decoded value and reject it")
	}
	if cfg.Conns != nil {
		t.Fatalf("a rejected value must not be written, got %v", cfg.Conns)
	}

	mustSet(t, k, ` [ {"id": "a", "kind": "slack"} ] `)
	assertGet(t, k, `[{"id":"a","kind":"slack"}]`, true)

	if err := k.Unset(); err != nil {
		t.Fatal(err)
	}
	assertGet(t, k, `null`, false)
}

func TestPathKey(t *testing.T) {
	cfg := keysDefault()
	k := PathKey(memoryBinding(&cfg), "home", "", func(c *keysConfig) *string { return &c.Home })

	assertRejects(t, k, "relative/dir", "./x", "~/x", "/bad\x00path")
	mustSet(t, k, "/abs/dir")
	assertGet(t, k, "/abs/dir", true)
	mustSet(t, k, "")
	assertGet(t, k, "", true) // differs from the default path
	if err := k.Unset(); err != nil {
		t.Fatal(err)
	}
	assertGet(t, k, "/default/home", false)
}

func TestEnvNameKey(t *testing.T) {
	cfg := keysDefault()
	k := EnvNameKey(memoryBinding(&cfg), "key_env", "", func(c *keysConfig) *string { return &c.KeyEnv })

	assertRejects(t, k, "1BAD", "A-B", "HAS SPACE", "$HOME")
	mustSet(t, k, "_API_KEY2")
	assertGet(t, k, "_API_KEY2", true)
	mustSet(t, k, "")
	assertGet(t, k, "", false)
}

func TestFieldKeyReadErrorReportsUnset(t *testing.T) {
	b := ConfigBinding[keysConfig]{
		Read:   func() (keysConfig, error) { return keysConfig{Name: "x"}, errors.New("unreadable") },
		Update: func(func(*keysConfig) error) error { return nil },
	}
	k := StringKey(b, "name", "", func(c *keysConfig) *string { return &c.Name }, nil)
	assertGet(t, k, "", false)
}

func TestNilDefaultIsTheZeroConfig(t *testing.T) {
	cfg := keysConfig{Workers: 3}
	b := memoryBinding(&cfg)
	b.Default = nil
	k := IntKey(b, "workers", "", func(c *keysConfig) *int { return &c.Workers }, 0, 10)
	assertGet(t, k, "3", true)
	if err := k.Unset(); err != nil {
		t.Fatal(err)
	}
	assertGet(t, k, "0", false)
}

// With a document, "set" is a statement about the file: a value equal to the
// default that somebody wrote down is set, and a default nobody wrote is not.
func TestDocumentDecidesSet(t *testing.T) {
	b, _ := storeBinding(t, `{"workers": 4}`)
	workers := IntKey(b, "workers", "", func(c *keysConfig) *int { return &c.Workers }, 1, 10)
	mode := OneOfKey(b, "mode", "", func(c *keysConfig) *string { return &c.Mode }, []string{"fast", "slow"})

	assertGet(t, workers, "4", true)
	assertGet(t, mode, "fast", false)

	mustSet(t, mode, "slow")
	assertGet(t, mode, "slow", true)
}

func TestDocumentUnsetRemovesThePath(t *testing.T) {
	b, store := storeBinding(t, `{
  "//": "kept through every write",
  "workers": 7,
  "engines": {"claude": {"floor": 20}},
  "name": "n"
}`)
	workers := IntKey(b, "workers", "", func(c *keysConfig) *int { return &c.Workers }, 1, 10)
	floor := OptionalIntKey(b, "engines.claude.floor", "", func(c *keysConfig) **int { return &c.Engines.Claude.Floor }, 0, 100)

	assertGet(t, floor, "20", true)
	if err := floor.Unset(); err != nil {
		t.Fatal(err)
	}
	assertGet(t, floor, "", false)

	// The default (4) is not the zero value, so the struct writes it; the
	// unset must still leave the file without it.
	if err := workers.Unset(); err != nil {
		t.Fatal(err)
	}
	assertGet(t, workers, "4", false)
	if _, found := store.RawValue("workers"); found {
		t.Error("unset should remove workers from the document")
	}

	data, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"//": "kept through every write"`, `"name": "n"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("document lost %s:\n%s", want, data)
		}
	}
}

func TestDocumentUnsetWithoutAFile(t *testing.T) {
	b, store := storeBinding(t, "")
	b.Update = func(func(*keysConfig) error) error { return nil } // a daemon that has not written yet
	k := IntKey(b, "workers", "", func(c *keysConfig) *int { return &c.Workers }, 1, 10)
	if err := k.Unset(); err != nil {
		t.Fatalf("unset with no file should succeed, got %v", err)
	}
	if _, err := os.Stat(store.Path); !os.IsNotExist(err) {
		t.Errorf("unset should not create the file, stat err=%v", err)
	}
}

func TestBuildersThroughConfigCommand(t *testing.T) {
	b, _ := storeBinding(t, "")
	cmd := ConfigCommand(nil, []ConfigKey{
		IntKey(b, "workers", "", func(c *keysConfig) *int { return &c.Workers }, 1, 10),
		JSONKey(b, "conns", "", func(c *keysConfig) *[]conn { return &c.Conns }, nil),
	})

	rec, err := runCmd(t, cmd, "set", "conns", `[{"id":"a","kind":"k"}]`)
	if err != nil || rec["set"] != true {
		t.Fatalf("set: rec=%v err=%v", rec, err)
	}
	rec, err = runCmd(t, cmd, "get", "conns")
	if err != nil || rec["value"] != `[{"id":"a","kind":"k"}]` || rec["set"] != true {
		t.Errorf("get: rec=%v err=%v", rec, err)
	}
	rec, err = runCmd(t, cmd, "unset", "workers")
	if err != nil || rec["value"] != "4" || rec["set"] != false {
		t.Errorf("unset: rec=%v err=%v", rec, err)
	}
	if _, err := runCmd(t, cmd, "set", "workers", "0"); err == nil {
		t.Error("an out-of-bounds value should fail through the command")
	}
}
