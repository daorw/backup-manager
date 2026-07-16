// preview_icon.go — composes the tray icon over a white background so the
// viewer can see the design (PNG transparency is invisible on dark backgrounds).
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	f, err := os.Open("internal/tray/icon.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		panic(err)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	white := color.NRGBA{255, 255, 255, 255}
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1024; x++ {
			dst.SetNRGBA(x, y, white)
		}
	}
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1024; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			if a > 0 {
				dst.SetNRGBA(x, y, color.NRGBA{
					uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255,
				})
			}
		}
	}
	out, err := os.Create("/tmp/icon_preview.png")
	if err != nil {
		panic(err)
	}
	defer out.Close()
	png.Encode(out, dst)
	println("preview written to /tmp/icon_preview.png")
}
