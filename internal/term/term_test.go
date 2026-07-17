package term

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"", Off, false},
		{"off", Off, false},
		{"auto", Auto, false},
		{" ON ", On, false},
		{"always", Off, true},
	}
	for _, tc := range cases {
		got, err := Parse("images", tc.in)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("Parse(%q) = (%v, err=%v), want (%v, err=%v)", tc.in, got, err, tc.want, tc.wantErr)
		}
	}
	if _, err := Parse("hyperlinks", "x"); err == nil || !strings.Contains(err.Error(), "hyperlinks") {
		t.Errorf("Parse error should carry the noun, got %v", err)
	}
}

func TestString(t *testing.T) {
	for m, want := range map[Mode]string{Off: "off", Auto: "auto", On: "on"} {
		if m.String() != want {
			t.Errorf("Mode(%d).String() = %q, want %q", m, m.String(), want)
		}
	}
}

func TestActive(t *testing.T) {
	var buf bytes.Buffer
	autoYes := func(io.Writer) bool { return true }
	autoNo := func(io.Writer) bool { return false }

	if Active(&buf, Off, autoYes) {
		t.Error("Off must never be active")
	}
	if !Active(&buf, On, autoNo) {
		t.Error("On must force-activate regardless of the predicate")
	}
	if !Active(&buf, Auto, autoYes) {
		t.Error("Auto must defer to the predicate (true)")
	}
	if Active(&buf, Auto, autoNo) {
		t.Error("Auto must defer to the predicate (false)")
	}
}

func TestIsTerminal(t *testing.T) {
	// A non-*os.File writer (test buffer) is never a terminal.
	if IsTerminal(&bytes.Buffer{}) {
		t.Error("a bytes.Buffer must not be reported as a terminal")
	}
	// A real pipe's ends are *os.File but not TTYs.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if IsTerminal(w) {
		t.Error("an os.Pipe writer must not be reported as a terminal")
	}
}

func TestIsTerminalReader(t *testing.T) {
	// A non-*os.File reader (test buffer) is never a terminal.
	if IsTerminalReader(&bytes.Buffer{}) {
		t.Error("a bytes.Buffer must not be reported as a terminal")
	}
	// A real pipe's read end is an *os.File but not a TTY — the production
	// stdin-is-piped path.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if IsTerminalReader(r) {
		t.Error("an os.Pipe reader must not be reported as a terminal")
	}
}
