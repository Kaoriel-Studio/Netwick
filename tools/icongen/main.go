package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

type pt struct{ x, y float64 }

func distSeg(p, a, b pt) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	t := ((p.x-a.x)*dx + (p.y-a.y)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	cx, cy := a.x+t*dx, a.y+t*dy
	return math.Hypot(p.x-cx, p.y-cy)
}

func inRoundRect(x, y, size, r float64) bool {
	cx := math.Max(r, math.Min(size-r, x))
	cy := math.Max(r, math.Min(size-r, y))
	return math.Hypot(x-cx, y-cy) <= r
}

func render(size int, tray bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	s := float64(size)
	line := []pt{{0.18, 0.56}, {0.36, 0.56}, {0.45, 0.36}, {0.57, 0.72}, {0.66, 0.50}, {0.82, 0.50}}
	width := 0.065
	if tray {
		width = 0.11
	}
	const ss = 4
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var bgA, fgA float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := (float64(x) + (float64(sx)+0.5)/ss) / s
					py := (float64(y) + (float64(sy)+0.5)/ss) / s
					margin := 0.06
					if tray {
						margin = 0.02
					}
					if inRoundRect(px-margin, py-margin, 1-2*margin, 0.2) {
						bgA++
					}
					p := pt{px, py}
					d := math.Inf(1)
					for i := 0; i+1 < len(line); i++ {
						d = math.Min(d, distSeg(p, line[i], line[i+1]))
					}
					if d <= width/2 {
						fgA++
					}
				}
			}
			bgA /= ss * ss
			fgA /= ss * ss
			t := (float64(x) + float64(y)) / (2 * s)
			r := 18 + (64-18)*t
			g := 158 + (92-158)*t
			b := 140 + (222-140)*t
			cr := r*(1-fgA) + 255*fgA
			cg := g*(1-fgA) + 255*fgA
			cb := b*(1-fgA) + 255*fgA
			img.SetNRGBA(x, y, color.NRGBA{uint8(cr), uint8(cg), uint8(cb), uint8(255 * bgA)})
		}
	}
	return img
}

func main() {
	save := func(path string, size int, tray bool) {
		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := png.Encode(f, render(size, tray)); err != nil {
			panic(err)
		}
	}
	save(os.Args[1], 1024, false)
	save(os.Args[2], 64, true)
}
