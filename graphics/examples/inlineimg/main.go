// Command inlineimg is a proof-of-concept for graphics.Encoder: it renders
// small colored squares inline among text, the way agent-slack would render
// Slack custom emoji. Run it in Ghostty (or kitty/WezTerm) to see the images;
// in any other terminal it prints a notice and falls back to bracketed labels.
//
//	go run ./graphics/examples/inlineimg
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	graphics "github.com/shhac/lib-agent-cli/graphics"
)

func main() {
	kitty := graphics.Detect() == graphics.ProtocolKitty
	if !kitty {
		fmt.Fprintf(os.Stderr,
			"note: %s terminal has no kitty graphics protocol; showing text fallback\n\n",
			graphics.Detect())
	}

	enc := graphics.NewEncoder()
	red := graphics.Image{ID: 1, Data: square(color.RGBA{0xff, 0x3b, 0x30, 0xff})}
	blue := graphics.Image{ID: 2, Data: square(color.RGBA{0x00, 0x7a, 0xff, 0xff})}

	// red appears twice: the second occurrence re-places by reference rather
	// than re-transmitting — the dedup the emoji case depends on.
	img := func(i graphics.Image, label string) string {
		if !kitty {
			return "[" + label + "]"
		}
		return enc.Inline(i, 1)
	}

	fmt.Printf("deploy %s shipped %s and again %s — done\n",
		img(red, "red"), img(blue, "blue"), img(red, "red"))
}

// square returns a 64x64 PNG filled with c — a stand-in for a custom emoji.
func square(c color.Color) []byte {
	const n = 64
	im := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			im.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, im); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
