package yaml

import (
	"bytes"
	"math"
	"strings"
	"testing"

	output "github.com/shhac/lib-agent-output"
)

func TestRegisteredEncoderEmitsYAML(t *testing.T) {
	// init() already called Register(); Print should now produce YAML.
	var buf bytes.Buffer
	if err := output.Print(&buf, map[string]any{"name": "widget", "count": 3}, output.FormatYAML, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "name: widget") || !strings.Contains(out, "count: 3") {
		t.Errorf("not YAML mapping output:\n%s", out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("looks like JSON, want YAML:\n%s", out)
	}
}

// JSON decoding produces float64 for every number, so the encoder's whole reason
// to exist is converting whole-valued float64 to int64 — otherwise a large ID
// renders in scientific notation. The original test only passed an int literal,
// which skips that path; these exercise it.
func TestEncodeNormalizesWholeFloats(t *testing.T) {
	cases := []struct {
		name     string
		in       any
		contains string
		absent   string
	}{
		{"large whole float renders as integer", float64(1500000), "1500000", "e+"},
		{"fractional float preserved", float64(1.5), "1.5", "e+"},
		{"nested slice whole float", map[string]any{"items": []any{float64(1e7)}}, "10000000", "e+"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := encode(tc.in)
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}
			out := string(b)
			if !strings.Contains(out, tc.contains) {
				t.Errorf("output missing %q:\n%s", tc.contains, out)
			}
			if strings.Contains(out, tc.absent) {
				t.Errorf("output has scientific notation %q (should be a plain integer):\n%s", tc.absent, out)
			}
		})
	}
}

func TestNormalizeNumbers(t *testing.T) {
	if got := normalizeNumbers(float64(42)); got != int64(42) {
		t.Errorf("whole float64 → %T(%v), want int64(42)", got, got)
	}
	if got := normalizeNumbers(float64(1.5)); got != float64(1.5) {
		t.Errorf("fractional float64 must stay float64, got %T(%v)", got, got)
	}
	if got := normalizeNumbers(math.Inf(1)); got != math.Inf(1) {
		t.Errorf("Inf must pass through untouched (not int64), got %T(%v)", got, got)
	}
	if got, ok := normalizeNumbers(math.NaN()).(float64); !ok || !math.IsNaN(got) {
		t.Errorf("NaN must pass through as float64, got %T(%v)", got, got)
	}
	// Recursion into maps and slices converts nested whole floats.
	nested := normalizeNumbers(map[string]any{"a": []any{float64(7)}}).(map[string]any)
	if nested["a"].([]any)[0] != int64(7) {
		t.Errorf("nested whole float not converted: %#v", nested)
	}
}

// Ordered documents must keep their field order through `--format yaml`, the
// same as through JSON and NDJSON. yaml.v3 has no ordered map type, so this is
// the half of the guarantee lib-agent-output cannot provide on its own.
func TestEncodePreservesOrderedFieldOrder(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{
			name: "reverse-alphabetical order survives",
			in:   output.Ordered{{Key: "status", Value: 1}, {Key: "expiryDate", Value: 1}},
			want: "status: 1\nexpiryDate: 1\n",
		},
		{
			name: "nested Ordered keeps its own order",
			in: output.Ordered{
				{Key: "name", Value: "status_1_expiryDate_1"},
				{Key: "key", Value: output.Ordered{{Key: "status", Value: 1}, {Key: "expiryDate", Value: 1}}},
			},
			want: "name: status_1_expiryDate_1\nkey:\n  status: 1\n  expiryDate: 1\n",
		},
		{
			name: "nulls survive",
			in:   output.Ordered{{Key: "deletedAt", Value: nil}, {Key: "name", Value: "x"}},
			want: "deletedAt: null\nname: x\n",
		},
		{
			name: "Ordered inside a slice",
			in:   []any{output.Ordered{{Key: "b", Value: 1}, {Key: "a", Value: 2}}},
			want: "- b: 1\n  a: 2\n",
		},
		{
			name: "Ordered inside a map",
			in:   map[string]any{"filter": output.Ordered{{Key: "z", Value: 1}, {Key: "a", Value: 2}}},
			want: "filter:\n  z: 1\n  a: 2\n",
		},
		{
			name: "empty Ordered",
			in:   output.Ordered{},
			want: "{}\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := encode(tc.in)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

// The whole-float fix must reach inside an Ordered too — that walk previously
// only descended maps and slices, so a large double in an ordered document
// rendered as 1.5e+06 while the same value in a map rendered as 1500000.
func TestEncodeNormalizesNumbersInsideOrdered(t *testing.T) {
	got, err := encode(output.Ordered{
		{Key: "big", Value: float64(1500000)},
		{Key: "ratio", Value: 1.5},
		{Key: "nested", Value: output.Ordered{{Key: "also", Value: float64(2000000)}}},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := "big: 1500000\nratio: 1.5\nnested:\n  also: 2000000\n"
	if string(got) != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// End to end through the registered encoder, which is how a CLI reaches it.
func TestRegisteredEncoderEmitsOrderedYAML(t *testing.T) {
	var buf bytes.Buffer
	spec := output.Ordered{
		{Key: "name", Value: "status_1_expiryDate_1"},
		{Key: "key", Value: output.Ordered{{Key: "status", Value: 1}, {Key: "expiryDate", Value: 1}}},
	}
	if err := output.Print(&buf, spec, output.FormatYAML, output.PruneEmpty); err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "name: status_1_expiryDate_1\nkey:\n  status: 1\n  expiryDate: 1\n"
	if buf.String() != want {
		t.Errorf("got %q want %q", buf.String(), want)
	}
}
