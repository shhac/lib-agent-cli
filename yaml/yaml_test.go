package yaml

import (
	"bytes"
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
