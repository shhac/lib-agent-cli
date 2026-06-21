package env

import "testing"

func TestPrefixFromName(t *testing.T) {
	cases := map[string]string{
		"agent-slack":    "AGENT_SLACK",
		"lin":            "LIN",
		"agent-deepweb":  "AGENT_DEEPWEB",
		"app.paulie.lin": "APP_PAULIE_LIN", // dotted ids become dotted prefixes — pass bare names
		"two words":      "TWO_WORDS",
	}
	for in, want := range cases {
		if got := PrefixFromName(in); got != want {
			t.Errorf("PrefixFromName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLookup_Precedence(t *testing.T) {
	ns := New("agent-foo") // prefix AGENT_FOO

	t.Run("specific wins over family on presence", func(t *testing.T) {
		t.Setenv("AGENT_FOO_TOKEN", "specific")
		t.Setenv("LIB_AGENT_TOKEN", "family")
		if v, ok := ns.Lookup("TOKEN"); !ok || v != "specific" {
			t.Fatalf("Lookup = %q,%v; want specific,true", v, ok)
		}
	})

	t.Run("falls back to family when specific unset", func(t *testing.T) {
		t.Setenv("LIB_AGENT_TOKEN", "family")
		if v, ok := ns.Lookup("TOKEN"); !ok || v != "family" {
			t.Fatalf("Lookup = %q,%v; want family,true", v, ok)
		}
	})

	t.Run("empty-but-set specific still shadows family", func(t *testing.T) {
		t.Setenv("AGENT_FOO_TOKEN", "")
		t.Setenv("LIB_AGENT_TOKEN", "family")
		if v, ok := ns.Lookup("TOKEN"); !ok || v != "" {
			t.Fatalf("Lookup = %q,%v; want \"\",true (presence shadows)", v, ok)
		}
	})
}

func TestFlag(t *testing.T) {
	ns := New("agent-foo")
	truthy := []string{"1", "true", "TRUE", "yes", "on"}
	falsey := []string{"0", "false", "FALSE", ""}

	for _, v := range truthy {
		t.Setenv("LIB_AGENT_NO_KEYCHAIN", v)
		if !ns.Flag("NO_KEYCHAIN") {
			t.Errorf("Flag with %q = false, want true", v)
		}
	}
	for _, v := range falsey {
		t.Setenv("LIB_AGENT_NO_KEYCHAIN", v)
		if ns.Flag("NO_KEYCHAIN") {
			t.Errorf("Flag with %q = true, want false", v)
		}
	}
}

func TestFlag_AbsentIsFalse(t *testing.T) {
	if (Namespace{Prefix: "AGENT_NONEXISTENT_XYZ"}).Flag("NOPE_NOPE") {
		t.Error("Flag for an absent var should be false")
	}
}

// TestZeroNamespace_FamilyOnly — the zero Namespace consults only the family var.
func TestZeroNamespace_FamilyOnly(t *testing.T) {
	var ns Namespace // empty prefix
	t.Setenv("LIB_AGENT_NO_KEYCHAIN", "1")
	if !ns.Flag("NO_KEYCHAIN") {
		t.Error("zero Namespace should still read the family-wide var")
	}
}
