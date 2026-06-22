package graphics

import (
	"bytes"
	"testing"
)

func TestParseMode(t *testing.T) {
	cases := []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"", ModeOff, false},
		{"off", ModeOff, false},
		{"auto", ModeAuto, false},
		{"AUTO", ModeAuto, false},
		{" on ", ModeOn, false},
		{"always", ModeOff, true},
		{"yes", ModeOff, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseMode(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseMode(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("ParseMode(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestActive(t *testing.T) {
	// A bytes.Buffer is never a terminal, so off and auto are both false; on
	// forces regardless. (The auto=true path needs a real TTY and is covered by
	// isatty itself.)
	var buf bytes.Buffer
	if Active(&buf, ModeOff) {
		t.Error("ModeOff must never be active")
	}
	if Active(&buf, ModeAuto) {
		t.Error("ModeAuto on a non-terminal writer must be inactive")
	}
	if !Active(&buf, ModeOn) {
		t.Error("ModeOn must force-activate regardless of the writer")
	}
}
