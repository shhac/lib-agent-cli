package cli

import (
	"bytes"
	"testing"
)

// TestColorFlag_Validation — NewRoot binds --color and PersistentPreRunE rejects
// an unknown value (agent-fixable) while accepting auto/always/never.
func TestColorFlag_Validation(t *testing.T) {
	for _, tc := range []struct {
		val     string
		wantErr bool
	}{
		{"auto", false}, {"always", false}, {"never", false}, {"rainbow", true},
	} {
		g := &Globals{}
		root := NewRoot(Options{Use: "x", Globals: g})
		root.SetArgs([]string{"--color", tc.val})
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		err := root.Execute()
		if tc.wantErr && err == nil {
			t.Errorf("--color %q: expected error", tc.val)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("--color %q: unexpected error: %v", tc.val, err)
		}
	}
}
