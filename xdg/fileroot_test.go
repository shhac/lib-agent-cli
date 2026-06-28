package xdg

import "testing"

func TestRoot(t *testing.T) {
	r := Root("cache", "/some/dir")
	if r.Name != "cache" || r.Path != "/some/dir" {
		t.Errorf("Root = %+v, want name cache path /some/dir", r)
	}
}
