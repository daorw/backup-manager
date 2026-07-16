// gen_tray_icon.go — generates a small-size optimized tray icon (template style).
//
// Design rationale:
//   - macOS menu bar renders tray icons at 16-22px; the previous icon had a
//     circular blue background that consumed 60% of canvas, leaving the
//     folder detail nearly invisible when scaled down.
//   - New design: solid black filled folder + a bold white upward arrow in
//     the center. High contrast, no thin strokes, recognizable at 16px.
//   - Uses a "template image" (black with alpha) so macOS tints it
//     appropriately for light/dark menu bar backgrounds.
//
// Run: go run scripts/gen_tray_icon.go
// Output: internal/tray/icon.png (1024x1024 PNG, RGBA)
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
)

const size = 1024

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	black := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	transparent := color.NRGBA{R: 0, G: 0, B: 0, A: 0}

	// === Step 1: Draw a solid filled folder (no holes, no inner cutout) ===
	// Body: large filled rounded rectangle
	drawFilledRoundedRect(img, 90, 320, 934, 920, 130, black)
	// Tab: smaller filled rounded rect on top-left (the folder's "tongue")
	drawFilledRoundedRect(img, 90, 170, 470, 400, 100, black)

	// === Step 2: Draw a bold white upward arrow in the center ===
	// Arrow shaft (vertical thick bar)
	drawFilledRoundedRect(img, 432, 540, 592, 820, 60, white)
	// Arrow head (triangle pointing up — wide base at top)
	drawTriangle(img,
		512, 380, // top point
		332, 580, // bottom-left
		692, 580, // bottom-right
		white,
	)

	// === Sanity: make sure no stray transparent pixels remain inside the
	// folder body (just a visual guarantee for the template image style) ===
	for y := 920; y < 924; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, transparent)
		}
	}

	// === Save ===
	f, err := os.Create("internal/tray/icon.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	log.Println("Wrote internal/tray/icon.png (1024x1024 template-style tray icon)")
}

func drawFilledRoundedRect(img *image.NRGBA, x0, y0, x1, y1, radius int, c color.NRGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if isInRoundedRect(x, y, x0, y0, x1, y1, radius) {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

func isInRoundedRect(x, y, x0, y0, x1, y1, r int) bool {
	if x >= x0+r && x < x1-r && y >= y0 && y < y1 {
		return true
	}
	if x >= x0 && x < x1 && y >= y0+r && y < y1-r {
		return true
	}
	corners := []struct{ cx, cy int }{
		{x0 + r, y0 + r},
		{x1 - r - 1, y0 + r},
		{x0 + r, y1 - r - 1},
		{x1 - r - 1, y1 - r - 1},
	}
	for _, c := range corners {
		dx := x - c.cx
		dy := y - c.cy
		if dx*dx+dy*dy <= r*r {
			return true
		}
	}
	return false
}

func drawTriangle(img *image.NRGBA, x0, y0, x1, y1, x2, y2 int, c color.NRGBA) {
	minX := min3(x0, x1, x2)
	maxX := max3(x0, x1, x2)
	minY := min3(y0, y1, y2)
	maxY := max3(y0, y1, y2)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if pointInTriangle(
				float64(x), float64(y),
				float64(x0), float64(y0),
				float64(x1), float64(y1),
				float64(x2), float64(y2)) {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

// pointInTriangle uses the sign-of-cross-product method (robust for any
// winding order, no division by zero edge case).
func pointInTriangle(px, py, x0, y0, x1, y1, x2, y2 float64) bool {
	d1 := sign(px, py, x0, y0, x1, y1)
	d2 := sign(px, py, x1, y1, x2, y2)
	d3 := sign(px, py, x2, y2, x0, y0)
	hasNeg := d1 < 0 || d2 < 0 || d3 < 0
	hasPos := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNeg && hasPos)
}

func sign(px, py, x0, y0, x1, y1 float64) float64 {
	return (px-x1)*(y0-y1) - (x0-x1)*(py-y1)
}

func min3(a, b, c int) int { return min(min(a, b), c) }
func max3(a, b, c int) int { return max(max(a, b), c) }
