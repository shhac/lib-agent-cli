// Package yaml registers a YAML encoder for lib-agent-output's FormatYAML so a
// CLI gets `--format yaml` by importing this package instead of copying the
// ~10-line encoder block into its own internal/output. It carries the
// gopkg.in/yaml.v3 dependency here in lib-agent-cli (which already has
// dependencies), keeping the core lib-agent-output module dependency-free.
//
// Usage — blank-import for the side effect (registration runs in init):
//
//	import _ "github.com/shhac/lib-agent-cli/yaml"
//
// or call Register() explicitly. Then `output.Print(w, v, output.FormatYAML, …)`
// and `output.WriteList(w, output.FormatYAML, …)` emit YAML. JSON and NDJSON are
// handled natively by lib-agent-output and need no registration.
package yaml

import (
	"bytes"

	output "github.com/shhac/lib-agent-output"
	yaml "gopkg.in/yaml.v3"
)

func init() { Register() }

// Register installs the YAML encoder (2-space indent) for output.FormatYAML.
// It is called automatically on import and is idempotent, so an explicit call
// is only needed by callers that avoid blank imports.
func Register() {
	output.RegisterEncoder(output.FormatYAML, encode)
}

func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}
