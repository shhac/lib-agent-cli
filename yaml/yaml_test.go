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
