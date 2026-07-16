//go:build ignore

package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	size := 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	center := size / 2
	radius := size/2 - 2

	bg := color.RGBA{22, 119, 255, 255}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := x - center
			dy := y - center
			d2 := dx*dx + dy*dy
			r2 := radius * radius

			if d2 <= r2 {
				// Anti-aliased edge
				if d2 >= r2-2*radius {
					alpha := uint8(255 * (r2 - d2) / (2 * radius))
					if alpha > 255 {
						alpha = 255
					}
					img.Set(x, y, color.RGBA{bg.R, bg.G, bg.B, alpha})
				} else {
					img.Set(x, y, bg)
				}
			}
		}
	}

	// Draw a simple "B" shape
	white := color.RGBA{255, 255, 255, 255}
	// Upper horizontal
	for x := center - 8; x <= center+8; x++ {
		img.Set(x, center-10, white)
	}
	// Left vertical
	for y := center - 10; y <= center+10; y++ {
		img.Set(center-8, y, white)
	}
	// Middle horizontal
	for x := center - 6; x <= center+6; x++ {
		img.Set(x, center, white)
	}
	// Bottom horizontal
	for x := center - 6; x <= center+8; x++ {
		img.Set(x, center+10, white)
	}
	// Right vertical top
	for y := center - 8; y <= center-2; y++ {
		img.Set(center+8, y, white)
	}
	// Right vertical bottom
	for y := center + 2; y <= center+8; y++ {
		img.Set(center+8, y, white)
	}

	f, _ := os.Create("assets/tray-icon.png")
	defer f.Close()
	png.Encode(f, img)
}
