package creds

import (
	"strings"
	"testing"
)

func TestReadSecretFlagWins(t *testing.T) {
	// A non-empty flag value short-circuits: stdin is never consulted, so even a
	// pipe with different content does not override the explicit flag.
	got, err := ReadSecret(strings.NewReader("piped\n"), "fromflag")
	if err != nil {
		t.Fatal(err)
	}
	if got != "fromflag" {
		t.Errorf("ReadSecret = %q, want fromflag", got)
	}
}

func TestReadSecretFromStdinTrims(t *testing.T) {
	got, err := ReadSecret(strings.NewReader("  ntn_abc123\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ntn_abc123" {
		t.Errorf("ReadSecret = %q, want ntn_abc123", got)
	}
}

func TestReadSecretEmptyWhenNoFlagAndEmptyPipe(t *testing.T) {
	// A non-terminal reader with no content yields "": the caller enforces
	// required-ness. (A strings.Reader is not an *os.File, so it is treated as
	// piped, not interactive.)
	got, err := ReadSecret(strings.NewReader(""), "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("ReadSecret = %q, want empty", got)
	}
}

func TestReadSecretLines(t *testing.T) {
	got, err := ReadSecretLines(strings.NewReader("api-key-value\n\n  app-key-value  \n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "api-key-value" || got[1] != "app-key-value" {
		t.Errorf("ReadSecretLines = %#v, want [api-key-value app-key-value]", got)
	}
}

func TestReadSecretLinesEmpty(t *testing.T) {
	got, err := ReadSecretLines(strings.NewReader("   \n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("ReadSecretLines = %#v, want empty", got)
	}
}

// errReader forces the io.ReadAll error path.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errRead }

var errRead = &readErr{}

type readErr struct{}

func (*readErr) Error() string { return "boom" }

func TestReadSecretReadError(t *testing.T) {
	if _, err := ReadSecret(errReader{}, ""); err == nil {
		t.Error("ReadSecret should propagate a read error")
	}
	if _, err := ReadSecretLines(errReader{}); err == nil {
		t.Error("ReadSecretLines should propagate a read error")
	}
}
