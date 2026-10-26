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

// CellW and CellH are the pixel size of one character cell: the size of the basicfont 7x13 face.
const (
	CellW = 7
	CellH = 13
)

// Sample picks at most limit strictly increasing indices from 0..n-1, always
// including the first and last.
func Sample(n, limit int) []int {
	out := []int{}
	switch {
	case n <= 0:
	case limit == 1:
		out = append(out, n-1)
	case n <= limit:
		for i := 0; i < n; i++ {
			out = append(out, i)
		}
	default:
		for k := 0; k < limit; k++ {
			out = append(out, k*(n-1)/(limit-1))
		}
	}
	return out
}

// Delay converts a speed multiplier to a GIF frame delay in 1/100 s.
func Delay(speed float64) int {
	return max(2, int(math.Round(10/speed)))
}

// palette converts frame.Palette to an image palette, in the same index order as frame.Color.
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

// StreamWriter streams frames into an animated GIF.
type StreamWriter struct {
	w      io.Writer
	delay  int
	frames []*image.Paletted
	delays []int
}

// NewStream creates a StreamWriter that encodes frames with delay (in 1/100 s).
func NewStream(w io.Writer, delay int) (*StreamWriter, error) {
	return &StreamWriter{w: w, delay: delay}, nil
}

// Append adds one frame grid to the stream.
func (s *StreamWriter) Append(g *frame.Grid) error {
	s.frames = append(s.frames, Image(g))
	s.delays = append(s.delays, s.delay)
	return nil
}

// Close flushes the animated GIF to the underlying writer, holding the last frame for 3 s.
func (s *StreamWriter) Close() error {
	if len(s.delays) > 0 {
		s.delays[len(s.delays)-1] = 300
	}
	a := gif.GIF{
		Image:     s.frames,
		Delay:     s.delays,
		LoopCount: 0,
	}
	return gif.EncodeAll(s.w, &a)
}

// Write encodes frames as a looping GIF; the last frame holds for 3 s.
func Write(w io.Writer, frames []*frame.Grid, delay int) error {
	sw, err := NewStream(w, delay)
	if err != nil {
		return err
	}
	for _, g := range frames {
		if err := sw.Append(g); err != nil {
			return err
		}
	}
	return sw.Close()
}
