// Package gifenc samples frames and encodes frame grids as an animated GIF.
package gifenc

import (
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/dpologdvinity/gitgarden/internal/frame"
)

const CellW, CellH = 7, 13

// Sample picks at most max strictly increasing indices from 0..n-1, always
// including the first and last.
func Sample(n, max int) []int {
	out := []int{}
	switch {
	case n <= 0:
	case max == 1:
		out = append(out, n-1)
	case n <= max:
		for i := 0; i < n; i++ {
			out = append(out, i)
		}
	default:
		for k := 0; k < max; k++ {
			out = append(out, k*(n-1)/(max-1))
		}
	}
	return out
}

// Delay converts a speed multiplier to a GIF frame delay in 1/100 s.
func Delay(speed float64) int {
	return max(2, int(math.Round(10/speed)))
}

func palette() color.Palette {
	pal := make(color.Palette, len(frame.Palette))
	for i, c := range frame.Palette {
		pal[i] = color.RGBA{c[0], c[1], c[2], 255}
	}
	return pal
}

// Image renders a grid with the 7x13 bitmap font.
func Image(g *frame.Grid) *image.Paletted {
	pal := palette()
	img := image.NewPaletted(image.Rect(0, 0, g.W*CellW, g.H*CellH), pal)
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			c := g.At(x, y)
			r := image.Rect(x*CellW, y*CellH, (x+1)*CellW, (y+1)*CellH)
			draw.Draw(img, r, image.NewUniform(pal[c.BG]), image.Point{}, draw.Src)
			if c.Ch == ' ' {
				continue
			}
			ch := c.Ch
			if ch < 0x20 || ch > 0x7e {
				ch = '?'
			}
			d := font.Drawer{Dst: img, Src: image.NewUniform(pal[c.FG]), Face: basicfont.Face7x13,
				Dot: fixed.P(x*CellW, y*CellH+11)}
			d.DrawString(string(ch))
		}
	}
	return img
}

// Write encodes frames as a looping GIF; the last frame holds for 3 s.
func Write(w io.Writer, frames []*frame.Grid, delay int) error {
	a := gif.GIF{LoopCount: 0}
	for i, g := range frames {
		a.Image = append(a.Image, Image(g))
		d := delay
		if i == len(frames)-1 {
			d = 300
		}
		a.Delay = append(a.Delay, d)
	}
	return gif.EncodeAll(w, &a)
}
