// Генератор PWA-иконок Альфа.Пульс: красный квадрат со скруглениями и белой
// «кардиограммой». Только stdlib (image/png), чтобы не тянуть зависимости.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

var (
	brand = color.RGBA{R: 0xEF, G: 0x31, B: 0x24, A: 0xFF}
	white = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
)

func main() {
	outDir := "web/public/icons"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}

	jobs := []struct {
		name     string
		size     int
		rounded  bool
		contentK float64 // масштаб пульса (маскируемым — меньше, безопасная зона)
	}{
		{"icon-192.png", 192, true, 1.0},
		{"icon-512.png", 512, true, 1.0},
		{"icon-maskable-512.png", 512, false, 0.72},
	}
	for _, j := range jobs {
		img := render(j.size, j.rounded, j.contentK)
		if err := save(filepath.Join(outDir, j.name), img); err != nil {
			fatal(err)
		}
		fmt.Println("создано:", filepath.Join(outDir, j.name))
	}
}

func render(size int, rounded bool, contentK float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	s := float64(size)
	radius := s * 0.22

	// фон: скруглённый квадрат (или полный — для maskable)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !rounded || insideRounded(float64(x)+0.5, float64(y)+0.5, s, radius) {
				img.SetRGBA(x, y, brand)
			}
		}
	}

	// «кардиограмма»: ломаная в центре
	base := []struct{ x, y float64 }{
		{0.14, 0.50}, {0.34, 0.50}, {0.42, 0.34},
		{0.52, 0.68}, {0.60, 0.42}, {0.65, 0.50}, {0.86, 0.50},
	}
	pts := make([][2]float64, len(base))
	for i, p := range base {
		pts[i][0] = s/2 + (p.x-0.5)*s*contentK
		pts[i][1] = s/2 + (p.y-0.5)*s*contentK
	}
	thick := s * 0.045 * contentK
	for i := 0; i+1 < len(pts); i++ {
		drawSegment(img, pts[i][0], pts[i][1], pts[i+1][0], pts[i+1][1], thick)
	}
	return img
}

func insideRounded(x, y, size, r float64) bool {
	if x >= r && x <= size-r {
		return y >= 0 && y <= size
	}
	if y >= r && y <= size-r {
		return x >= 0 && x <= size
	}
	// угловые четверти — по окружности
	cx := r
	if x > size-r {
		cx = size - r
	}
	cy := r
	if y > size-r {
		cy = size - r
	}
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

// drawSegment рисует отрезок «кистью»-кругом (грубо, но для иконки достаточно).
func drawSegment(img *image.RGBA, x1, y1, x2, y2, thick float64) {
	dist := math.Hypot(x2-x1, y2-y1)
	steps := int(dist*2) + 1
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		drawDot(img, x1+(x2-x1)*t, y1+(y2-y1)*t, thick/2)
	}
}

func drawDot(img *image.RGBA, cx, cy, r float64) {
	for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			if dx*dx+dy*dy <= r*r && image.Pt(x, y).In(img.Bounds()) {
				img.SetRGBA(x, y, white)
			}
		}
	}
}

func save(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "genicons:", err)
	os.Exit(1)
}
