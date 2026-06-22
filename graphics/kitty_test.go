package graphics

import (
	"encoding/base64"
	"strings"
	"testing"
)

// payloads reassembles the base64 chunks from a transmit sequence and decodes
// them, so a test can assert the bytes survived chunking intact.
func payloads(t *testing.T, seq string) []byte {
	t.Helper()
	var b64 strings.Builder
	for _, esc := range strings.Split(seq, "\x1b\\") {
		if esc == "" {
			continue
		}
		_, payload, ok := strings.Cut(esc, ";")
		if !ok {
			continue
		}
		b64.WriteString(payload)
	}
	data, err := base64.StdEncoding.DecodeString(b64.String())
	if err != nil {
		t.Fatalf("decode reassembled payload: %v", err)
	}
	return data
}

func TestInlineTransmitThenPlace(t *testing.T) {
	enc := NewEncoder()
	img := Image{ID: 7, Data: []byte("PNGDATA")}

	first := enc.Inline(img, 1)
	if !strings.Contains(first, "a=T") || !strings.Contains(first, "i=7") {
		t.Fatalf("first call should transmit-and-display with id: %q", first)
	}
	if !strings.HasPrefix(first, "\x1b_G") || !strings.HasSuffix(first, "\x1b\\") {
		t.Fatalf("first call not a well-formed APC sequence: %q", first)
	}
	if got := string(payloads(t, first)); got != "PNGDATA" {
		t.Fatalf("transmitted payload = %q, want PNGDATA", got)
	}

	second := enc.Inline(img, 1)
	if !strings.Contains(second, "a=p") || !strings.Contains(second, "i=7") {
		t.Fatalf("repeat should re-place by reference: %q", second)
	}
	if strings.Contains(second, base64.StdEncoding.EncodeToString(img.Data)) {
		t.Fatalf("repeat must not re-transmit bytes: %q", second)
	}
}

func TestInlineEmpty(t *testing.T) {
	enc := NewEncoder()
	if got := enc.Inline(Image{ID: 0, Data: []byte("x")}, 1); got != "" {
		t.Errorf("zero ID should yield no output, got %q", got)
	}
	if got := enc.Inline(Image{ID: 1, Data: nil}, 1); got != "" {
		t.Errorf("empty data should yield no output, got %q", got)
	}
}

func TestInlineClampsHeight(t *testing.T) {
	enc := NewEncoder()
	out := enc.Inline(Image{ID: 1, Data: []byte("x")}, 0)
	if !strings.Contains(out, "r=1") {
		t.Errorf("cellHeight < 1 should clamp to r=1, got %q", out)
	}
}

func TestTransmitChunking(t *testing.T) {
	// Raw bytes whose base64 exceeds one chunk, forcing continuation escapes.
	raw := make([]byte, 4096)
	for i := range raw {
		raw[i] = byte(i)
	}
	seq := kittyTransmit(9, raw, 1)

	n := strings.Count(seq, "\x1b_G")
	if n < 2 {
		t.Fatalf("expected multiple chunks, got %d", n)
	}
	if c := strings.Count(seq, "m=1"); c != n-1 {
		t.Errorf("expected %d continuation markers (m=1), got %d", n-1, c)
	}
	if c := strings.Count(seq, "m=0"); c != 1 {
		t.Errorf("expected exactly one terminal marker (m=0), got %d", c)
	}
	if !strings.HasPrefix(seq, "\x1b_Ga=T") {
		t.Errorf("control keys must ride the first chunk: %.16q", seq)
	}
	got := payloads(t, seq)
	if string(got) != string(raw) {
		t.Errorf("reassembled payload differs from original (%d vs %d bytes)", len(got), len(raw))
	}
}
