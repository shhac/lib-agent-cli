package hyperlink

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
		{" ON ", ModeOn, false},
		{"always", ModeOff, true},
	}
	for _, tc := range cases {
		got, err := ParseMode(tc.in)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("ParseMode(%q) = (%v, err=%v), want (%v, err=%v)", tc.in, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestActive(t *testing.T) {
	var buf bytes.Buffer // never a terminal
	if Active(&buf, ModeOff) {
		t.Error("off must be inactive")
	}
	if Active(&buf, ModeAuto) {
		t.Error("auto on a non-terminal must be inactive")
	}
	if !Active(&buf, ModeOn) {
		t.Error("on must force-activate")
	}
}

func TestEncode(t *testing.T) {
	got := Encode("https://example.com/x", "click")
	want := "\x1b]8;;https://example.com/x\x1b\\click\x1b]8;;\x1b\\"
	if got != want {
		t.Errorf("Encode = %q, want %q", got, want)
	}
	if got := Encode("", "plain"); got != "plain" {
		t.Errorf("empty url should pass text through, got %q", got)
	}
	if got := Encode("bad\x1burl", "label"); got != "label" {
		t.Errorf("control bytes in url should pass text through, got %q", got)
	}
}
