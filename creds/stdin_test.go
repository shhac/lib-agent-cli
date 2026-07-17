package creds

import (
	"os"
	"strings"
	"testing"
)

func TestReadSecretFlagWinsVerbatim(t *testing.T) {
	// A non-empty flag value short-circuits: stdin is never consulted, and the
	// flag is returned VERBATIM — unlike the stdin path, it is not trimmed, so a
	// flag value with surrounding whitespace survives intact. (Guards against a
	// stray TrimSpace(flagVal) "for consistency".)
	got, err := ReadSecret(strings.NewReader("piped\n"), "  from flag  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "  from flag  " {
		t.Errorf("ReadSecret = %q, want the flag returned verbatim (untrimmed)", got)
	}
}

func TestReadSecretFromOSPipe(t *testing.T) {
	// Exercise the real production reader type: cmd.InOrStdin() resolves to an
	// *os.File, not the strings.Reader the other tests use. A pipe's read end is
	// an *os.File that is not a tty, so ReadSecret reads and trims it.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := w.WriteString("piped-secret\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	got, err := ReadSecret(r, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "piped-secret" {
		t.Errorf("ReadSecret(os.Pipe) = %q, want piped-secret", got)
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

func TestReadSecretsFillsFieldsInOrder(t *testing.T) {
	var api, app string
	if err := ReadSecrets(strings.NewReader("api-key-value\n\n  app-key-value  \n"), &api, &app); err != nil {
		t.Fatal(err)
	}
	if api != "api-key-value" || app != "app-key-value" {
		t.Errorf("ReadSecrets filled (%q, %q), want (api-key-value, app-key-value)", api, app)
	}
}

func TestReadSecretsAnyFlagSetSkipsStdin(t *testing.T) {
	// A flag on any field is all-or-nothing: stdin must not be consulted, so the
	// other field stays empty rather than being filled from the pipe.
	api, app := "from-flag", ""
	if err := ReadSecrets(strings.NewReader("piped-api\npiped-app\n"), &api, &app); err != nil {
		t.Fatal(err)
	}
	if api != "from-flag" || app != "" {
		t.Errorf("ReadSecrets = (%q, %q), want (from-flag, \"\") — flags win all-or-nothing", api, app)
	}
}

func TestReadSecretsFewerLinesLeavesRest(t *testing.T) {
	// One piped line fills only the first field; the second stays empty for the
	// caller's required-ness check.
	var api, app string
	if err := ReadSecrets(strings.NewReader("only-api\n"), &api, &app); err != nil {
		t.Fatal(err)
	}
	if api != "only-api" || app != "" {
		t.Errorf("ReadSecrets = (%q, %q), want (only-api, \"\")", api, app)
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
	var f string
	if err := ReadSecrets(errReader{}, &f); err == nil {
		t.Error("ReadSecrets should propagate a read error")
	}
}
