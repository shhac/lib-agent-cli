package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoEndToEnd builds the demo and drives it as a subprocess against a
// throwaway XDG config dir, proving the whole lib-agent-cli surface end to end:
// the config command, the resolution helpers, the --yes gate, and the
// unknown-command handler. It deliberately avoids `login` (which touches the
// real keychain on macOS).
func TestDemoEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping demo build in -short")
	}
	bin := filepath.Join(t.TempDir(), "demo")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build demo: %v\n%s", err, out)
	}

	cfg := t.TempDir()
	run := func(args ...string) (stdout, stderr string, code int) {
		c := exec.Command(bin, args...)
		c.Env = append(c.Environ(),
			"XDG_CONFIG_HOME="+cfg,
			"XDG_CACHE_HOME="+filepath.Join(cfg, "cache"),
			"DEMO_ACCOUNT=",
		)
		var so, se strings.Builder
		c.Stdout, c.Stderr = &so, &se
		err := c.Run()
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
		return so.String(), se.String(), code
	}

	if _, _, code := run("config", "set", "page_size", "5"); code != 0 {
		t.Fatal("config set page_size should succeed")
	}
	if out, _, _ := run("config", "get", "page_size"); !strings.Contains(out, `"value":"5"`) {
		t.Errorf("config get: %s", out)
	}
	if _, se, code := run("config", "set", "page_size", "999"); code != 1 || !strings.Contains(se, `"fixable_by":"agent"`) {
		t.Errorf("page_size validation: code=%d se=%s", code, se)
	}
	if _, se, code := run("whoami"); code != 1 || !strings.Contains(se, `"fixable_by":"human"`) {
		t.Errorf("whoami (unset): code=%d se=%s", code, se)
	}
	run("config", "set", "default_account", "alice")
	if out, _, code := run("whoami"); code != 0 || !strings.Contains(out, `"account":"alice"`) {
		t.Errorf("whoami: code=%d out=%s", code, out)
	}
	if out, _, _ := run("item-list"); strings.Count(out, `"id"`) != 5 {
		t.Errorf("item-list should honor page_size=5, got: %s", out)
	}
	if _, se, code := run("logout"); code != 1 || !strings.Contains(se, `"fixable_by":"human"`) {
		t.Errorf("logout gate: code=%d se=%s", code, se)
	}
	if out, _, code := run("logout", "--yes"); code != 0 || !strings.Contains(out, "logged_out") {
		t.Errorf("logout --yes: code=%d out=%s", code, out)
	}
	if _, se, code := run("bogus"); code != 1 || !strings.Contains(se, `"fixable_by":"agent"`) {
		t.Errorf("unknown command: code=%d se=%s", code, se)
	}
}
