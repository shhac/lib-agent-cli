// Package graphics renders bitmap images inline in a terminal that supports a
// pixel-graphics protocol (today: the Kitty graphics protocol, spoken by
// Ghostty, kitty, and WezTerm).
//
// It is the human-facing counterpart to lib-agent-output's color: where color
// wraps cosmetic ANSI around the canonical machine bytes — under the invariant
// that stripping the escapes yields the original — an image escape has no
// plaintext original to strip back to. Images therefore do NOT belong in the
// zero-dependency wire contract; they live here, beside dialog, in the runtime
// layer that already owns "interact with the human's terminal".
//
// Two concerns compose, each its own seam so a caller can change one without the
// other:
//
//   - the decision — WHETHER to draw images on a given stream: a Mode (off/auto/
//     on, the --images flag, mirroring --color) combined with the stream and the
//     terminal's capability. Active(w, mode) is the one call a renderer makes —
//     the images counterpart to output.Enabled for color — and Detect reports the
//     raw capability it consults for auto. A caller chooses the image branch over
//     a plain-text fallback when Active is true.
//   - the mechanism — HOW image bytes become an inline escape sequence: an
//     Encoder turns an Image into bytes you splice between two runs of text. It
//     is Kitty-only today; a second protocol is a new method, not a change to
//     any caller. There is deliberately no protocol registry — add the seam when
//     a second mechanism actually exists.
//
// Inline means within the flow of a line: the encoder clamps an image to a
// whole number of text rows (one, for an emoji) so following text continues on
// the same line and line height is undisturbed. Repeated images are cheap — the
// Encoder transmits an image's bytes once and re-places later occurrences by
// reference, so the same emoji appearing ten times costs one upload.
package graphics
