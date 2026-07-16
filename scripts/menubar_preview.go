// menubar_preview.go — renders the tray icon at 22px on a simulated macOS
// menu bar background, so we can see how it'll look in production.
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	f, _ := os.Open("internal/tray/icon.png")
	defer f.Close()
	src, _ := png.Decode(f)

	const w, h = 440, 22
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	// Light menu bar background (macOS light mode)
	bg := color.NRGBA{230, 230, 230, 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.SetNRGBA(x, y, bg)
		}
	}

	// Place 5 copies of the icon at 22px, spaced evenly
	for i := 0; i < 5; i++ {
		baseX := i*88 + 33
		for y := 0; y < 22; y++ {
			for x := 0; x < 22; x++ {
				// Sample the source 1024x1024 image at the corresponding location
				srcX := (x * 1024) / 22
				srcY := (y * 1024) / 22
				r, g, b, a := src.At(srcX, srcY).RGBA()
				if a > 0 {
					dst.SetNRGBA(baseX+x, y, color.NRGBA{
						uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255,
					})
				}
			}
		}
	}

	out, _ := os.Create("/tmp/icon_menubar.png")
	defer out.Close()
	png.Encode(out, dst)
	println("ok")
}
