package cli

import (
	"bytes"
	"os"
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

// TestIsTerminal_NonFileIsFalse — a non-*os.File writer (e.g. a test buffer or a
// pipe) is never a terminal, keeping captured/piped output uncolored in auto.
func TestIsTerminal_NonFileIsFalse(t *testing.T) {
	if isTerminal(&bytes.Buffer{}) {
		t.Error("a bytes.Buffer must not be reported as a terminal")
	}
	// A real pipe's read/write ends are *os.File but not TTYs.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if isTerminal(w) {
		t.Error("an os.Pipe writer must not be reported as a terminal")
	}
}
