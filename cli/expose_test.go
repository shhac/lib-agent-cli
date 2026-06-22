package cli

import "testing"

// TestExposeFlag_OptIn — --expose is registered only when the CLI declares it
// redacts (Options.Redacts), so non-redacting tools don't advertise a no-op flag.
func TestExposeFlag_OptIn(t *testing.T) {
	plain := NewRoot(Options{Use: "x", Globals: &Globals{}})
	if plain.PersistentFlags().Lookup("expose") != nil {
		t.Error("--expose must not be registered when Redacts is false")
	}

	redacting := NewRoot(Options{Use: "x", Globals: &Globals{}, Redacts: true})
	if redacting.PersistentFlags().Lookup("expose") == nil {
		t.Error("--expose must be registered when Redacts is true")
	}
}
