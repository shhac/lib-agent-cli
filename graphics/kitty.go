package graphics

import (
	"encoding/base64"
	"strconv"
	"strings"
)

// kittyChunk is the maximum base64 payload per escape. The Kitty protocol caps
// a single graphics command's payload at 4096 base64 bytes; larger images are
// split across continuation chunks (m=1 on all but the last, m=0 on the last).
const kittyChunk = 4096

// Image is a transmittable bitmap. ID must be stable and unique per distinct
// image within an Encoder's lifetime: it is how repeats are deduplicated and
// how later placements reference the already-uploaded bytes. Data is the raw
// encoded image (PNG); the encoder hands it to the terminal verbatim — Kitty
// decodes PNG directly, so no pixel decoding happens here.
type Image struct {
	ID   uint32
	Data []byte
}

// An Encoder builds inline-image escape sequences for one output stream,
// deduplicating transmits across calls. The zero value is not usable; call
// NewEncoder. An Encoder is not safe for concurrent use.
type Encoder struct {
	sent map[uint32]bool
}

// NewEncoder returns an Encoder with an empty transmit set.
func NewEncoder() *Encoder {
	return &Encoder{sent: map[uint32]bool{}}
}

// Inline returns the escape sequence that draws img at the current cursor
// position, clamped to cellHeight text rows (minimum 1) with aspect ratio
// preserved, leaving the cursor immediately to the right so following text
// continues on the same line. Splice the result between two runs of text.
//
// The first call for a given img.ID transmits its bytes; later calls for the
// same ID emit only a compact re-placement, so a repeated emoji costs one
// upload. Returns "" for a zero ID or empty Data, so a caller can splice
// unconditionally and get nothing when there is nothing to draw.
func (e *Encoder) Inline(img Image, cellHeight int) string {
	if img.ID == 0 || len(img.Data) == 0 {
		return ""
	}
	if cellHeight < 1 {
		cellHeight = 1
	}
	if e.sent[img.ID] {
		return kittyPlace(img.ID, cellHeight)
	}
	e.sent[img.ID] = true
	return kittyTransmit(img.ID, img.Data, cellHeight)
}

// kittyTransmit builds the transmit-and-display command (a=T) for a fresh
// image: PNG payload (f=100), direct transmission (t=d), height clamped to rows
// with width computed to preserve aspect, responses suppressed (q=2). The
// base64 payload is chunked; control keys ride only on the first chunk.
func kittyTransmit(id uint32, data []byte, rows int) string {
	payload := base64.StdEncoding.EncodeToString(data)
	control := "a=T,f=100,t=d,r=" + strconv.Itoa(rows) +
		",i=" + strconv.FormatUint(uint64(id), 10) + ",q=2"

	var b strings.Builder
	for first, rest := true, payload; len(rest) > 0; first = false {
		chunk := rest
		if len(chunk) > kittyChunk {
			chunk = chunk[:kittyChunk]
		}
		rest = rest[len(chunk):]
		more := "0"
		if len(rest) > 0 {
			more = "1"
		}

		b.WriteString("\x1b_G")
		if first {
			b.WriteString(control)
			b.WriteString(",m=")
		} else {
			b.WriteString("m=")
		}
		b.WriteString(more)
		b.WriteByte(';')
		b.WriteString(chunk)
		b.WriteString("\x1b\\")
	}
	return b.String()
}

// kittyPlace re-displays an already-transmitted image by reference (a=p), at the
// cursor, clamped to rows. No payload, so a repeat is a handful of bytes.
func kittyPlace(id uint32, rows int) string {
	return "\x1b_Ga=p,i=" + strconv.FormatUint(uint64(id), 10) +
		",r=" + strconv.Itoa(rows) + ",q=2\x1b\\"
}
