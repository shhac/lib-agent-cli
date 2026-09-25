package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	output "github.com/shhac/lib-agent-output"

	"github.com/shhac/lib-agent-cli/creds"
)

// Typed config keys.
//
// Every CLI with a `config` command wrote the same four closures per key:
// read the config, render one field, parse and bound-check a string, write it
// back, and put it back to its default on unset. The builders below are that
// scaffold once, parameterised by a field accessor. What the keys are called,
// what their bounds are and where the config lives stay with the CLI; the
// binding is how it says so.

// ConfigBinding connects typed keys to a CLI's config: how to read it, how to
// change it (the caller's own writer: a store, or a running daemon), its
// defaults, and optionally the store's document so "set" means "present in
// the file" rather than "non-zero".
type ConfigBinding[C any] struct {
	// Read returns the config as the CLI sees it. A key whose Read fails
	// reports itself unset with an empty value, since Get has no error path.
	Read func() (C, error)
	// Update applies a change under the writer's own concurrency rules. A
	// creds.Store binding would wrap Store.Update; a daemon-backed one sends
	// the mutation to the process that owns the file.
	Update func(func(*C) error) error
	// Default returns the config a fresh install runs with; unset restores a
	// field to its value here. Nil means the zero C.
	Default func() C
	// Doc, when given, is the store whose document the key NAMES address:
	// each key's name is read as the dotted path of its field in the file.
	// A key is then set exactly when that path is present, and unset also
	// removes the path so the file stays sparse.
	Doc *creds.Store
}

func (b ConfigBinding[C]) defaults() C {
	if b.Default == nil {
		var zero C
		return zero
	}
	return b.Default()
}

// FieldKey is the scaffold every builder here parameterises, exported for a
// key type they do not cover (a duration, a float). parse turns input into the
// stored value, carrying all validation; format renders the stored value for
// get. Unset restores the field from the binding's Default.
func FieldKey[C, T any](b ConfigBinding[C], name, description string, field func(*C) *T, parse func(string) (T, error), format func(T) string) ConfigKey {
	return ConfigKey{
		Name:        name,
		Description: description,
		Get: func() (string, bool) {
			cfg, err := b.Read()
			if err != nil {
				return "", false
			}
			v := *field(&cfg)
			return format(v), isSet(b, name, v, field)
		},
		Set: func(value string) error {
			v, err := parse(value)
			if err != nil {
				return err
			}
			return b.Update(func(cfg *C) error {
				*field(cfg) = v
				return nil
			})
		},
		Unset: func() error {
			def := b.defaults()
			err := b.Update(func(cfg *C) error {
				*field(cfg) = *field(&def)
				return nil
			})
			if err != nil {
				return err
			}
			return b.dropFromDoc(name)
		},
	}
}

// isSet answers from the document when there is one: a value equal to the
// default that somebody wrote down is still a decision, and a default the
// struct happens to hold is not.
func isSet[C, T any](b ConfigBinding[C], name string, v T, field func(*C) *T) bool {
	if b.Doc != nil {
		_, found := b.Doc.RawValue(name)
		return found
	}
	def := b.defaults()
	return !reflect.DeepEqual(v, *field(&def))
}

// dropFromDoc keeps the file sparse after an unset. It only helps while the
// struct's encoding also omits the default (omitempty, a nil pointer): a field
// that always marshals is written straight back by the next save.
func (b ConfigBinding[C]) dropFromDoc(name string) error {
	if b.Doc == nil {
		return nil
	}
	// A missing or unreadable file has nothing to drop, and RawDelete would
	// report the read failure as if the unset had failed.
	if _, found := b.Doc.RawValue(name); !found {
		return nil
	}
	_, err := b.Doc.RawDelete(name)
	return err
}

// StringKey is a free-form string. validate may be nil.
func StringKey[C any](b ConfigBinding[C], name, description string, field func(*C) *string, validate func(string) error) ConfigKey {
	return FieldKey(b, name, description, field, func(v string) (string, error) {
		if validate == nil {
			return v, nil
		}
		return v, validate(v)
	}, identity)
}

// OneOfKey is a string restricted to values, which are also offered as
// completions. The error lists them from the same slice, so the two cannot
// drift. An empty string is accepted only if values contains it.
func OneOfKey[C any](b ConfigBinding[C], name, description string, field func(*C) *string, values []string) ConfigKey {
	k := StringKey(b, name, description, field, func(v string) error {
		if slices.Contains(values, v) {
			return nil
		}
		return output.Newf(output.FixableByAgent, "Invalid %s: %q. Valid: %s", name, v, strings.Join(values, ", "))
	})
	k.Values = values
	return k
}

// IntKey is an int within [min, max].
func IntKey[C any](b ConfigBinding[C], name, description string, field func(*C) *int, min, max int) ConfigKey {
	return FieldKey(b, name, description, field,
		func(v string) (int, error) { return parseBoundedInt(name, v, min, max) },
		strconv.Itoa)
}

// OptionalIntKey is IntKey over a nullable field, for a setting whose default
// is decided in code rather than stored: nil renders empty, and an explicit
// value (0 included) is a real setting.
func OptionalIntKey[C any](b ConfigBinding[C], name, description string, field func(*C) **int, min, max int) ConfigKey {
	return FieldKey(b, name, description, field,
		func(v string) (*int, error) {
			n, err := parseBoundedInt(name, v, min, max)
			if err != nil {
				return nil, err
			}
			return &n, nil
		},
		formatOptional(strconv.Itoa))
}

// OptionalBoolKey is a nullable bool, so "false" can be stated rather than
// merely defaulted into.
func OptionalBoolKey[C any](b ConfigBinding[C], name, description string, field func(*C) **bool) ConfigKey {
	k := FieldKey(b, name, description, field,
		func(v string) (*bool, error) {
			parsed, err := strconv.ParseBool(v)
			if err != nil {
				return nil, output.Newf(output.FixableByAgent, "%s must be true or false, got %q", name, v)
			}
			return &parsed, nil
		},
		formatOptional(strconv.FormatBool))
	k.Values = []string{"true", "false"}
	return k
}

// JSONKey takes any JSON value — the key for an array or object that has no
// sensible string form. The input must decode into the field's type with no
// unknown object fields and nothing trailing, so a typo in a nested key is an
// error rather than a silently dropped setting. validate may be nil; it sees
// the decoded value. Get renders compact JSON.
func JSONKey[C, T any](b ConfigBinding[C], name, description string, field func(*C) *T, validate func(T) error) ConfigKey {
	return FieldKey(b, name, description, field,
		func(v string) (T, error) {
			decoded, err := decodeStrict[T](v)
			if err != nil {
				return decoded, output.Newf(output.FixableByAgent, "%s must be JSON matching its type: %v", name, err)
			}
			if validate == nil {
				return decoded, nil
			}
			return decoded, validate(decoded)
		},
		func(v T) string {
			data, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			return string(data)
		})
}

// PathKey is an absolute filesystem path, or empty. A relative path is
// refused rather than resolved: a config outlives the directory it was set
// from, so it would mean something different to every later reader.
func PathKey[C any](b ConfigBinding[C], name, description string, field func(*C) *string) ConfigKey {
	return StringKey(b, name, description, field, func(v string) error {
		if v == "" || (filepath.IsAbs(v) && !strings.ContainsRune(v, 0)) {
			return nil
		}
		return output.Newf(output.FixableByAgent, "%s must be an absolute path, got %q", name, v)
	})
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// EnvNameKey is the NAME of an environment variable, or empty — how a config
// refers to a secret without holding it.
func EnvNameKey[C any](b ConfigBinding[C], name, description string, field func(*C) *string) ConfigKey {
	return StringKey(b, name, description, field, func(v string) error {
		if v == "" || envName.MatchString(v) {
			return nil
		}
		return output.Newf(output.FixableByAgent, "%s must be an environment variable name, got %q", name, v)
	})
}

func identity(v string) string { return v }

func formatOptional[T any](format func(T) string) func(*T) string {
	return func(p *T) string {
		if p == nil {
			return ""
		}
		return format(*p)
	}
}

func parseBoundedInt(name, value string, min, max int) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < min || n > max {
		return 0, output.Newf(output.FixableByAgent, "%s must be an integer in [%d, %d], got %q", name, min, max, value)
	}
	return n, nil
}

func decodeStrict[T any](v string) (T, error) {
	var decoded T
	dec := json.NewDecoder(bytes.NewReader([]byte(v)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&decoded); err != nil {
		return decoded, err
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return decoded, errors.New("unexpected data after the value")
	}
	return decoded, nil
}
