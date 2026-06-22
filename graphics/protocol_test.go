package graphics

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Protocol
	}{
		{"ghostty", map[string]string{"TERM_PROGRAM": "ghostty", "TERM": "xterm-256color"}, ProtocolKitty},
		{"ghostty mixed case", map[string]string{"TERM_PROGRAM": "Ghostty"}, ProtocolKitty},
		{"wezterm", map[string]string{"TERM_PROGRAM": "WezTerm"}, ProtocolKitty},
		{"kitty via TERM", map[string]string{"TERM": "xterm-kitty"}, ProtocolKitty},
		{"kitty via window id", map[string]string{"KITTY_WINDOW_ID": "1"}, ProtocolKitty},
		{"plain xterm", map[string]string{"TERM_PROGRAM": "Apple_Terminal", "TERM": "xterm-256color"}, ProtocolNone},
		{"empty", map[string]string{}, ProtocolNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detect(func(k string) string { return tc.env[k] })
			if got != tc.want {
				t.Fatalf("detect = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProtocolString(t *testing.T) {
	if ProtocolKitty.String() != "kitty" {
		t.Errorf("ProtocolKitty = %q", ProtocolKitty.String())
	}
	if ProtocolNone.String() != "none" {
		t.Errorf("ProtocolNone = %q", ProtocolNone.String())
	}
}
