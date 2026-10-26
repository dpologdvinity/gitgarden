// Package gifenc samples frames and encodes frame grids as an animated GIF.
package gifenc

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io"
	"math"
	"slices"

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

// StreamWriter streams frames into an animated GIF without buffering all images in memory.
type StreamWriter struct {
	w           io.Writer
	delay       int
	headerDone  bool
	pendingGCE  []byte
	pendingData []byte
}

// NewStream creates a StreamWriter that encodes frames with delay (in 1/100 s).
func NewStream(w io.Writer, delay int) (*StreamWriter, error) {
	return &StreamWriter{w: w, delay: delay}, nil
}

// Append adds one frame grid to the stream.
func (s *StreamWriter) Append(g *frame.Grid) error {
	img := Image(g)
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		return err
	}
	raw := buf.Bytes()
	hdrLen := 13
	if raw[10]&0x80 != 0 {
		hdrLen += 3 * (1 << ((raw[10] & 0x07) + 1))
	}
	if len(raw) <= hdrLen {
		return io.ErrUnexpectedEOF
	}

	if !s.headerDone {
		// Write GIF header + screen descriptor + global palette
		if _, err := s.w.Write(raw[:hdrLen]); err != nil {
			return err
		}
		// Write Netscape 2.0 application extension for infinite loop (19 bytes)
		loopBlock := []byte{
			0x21, 0xFF, 0x0B,
			'N', 'E', 'T', 'S', 'C', 'A', 'P', 'E', '2', '.', '0',
			0x03, 0x01, 0x00, 0x00, 0x00,
		}
		if _, err := s.w.Write(loopBlock); err != nil {
			return err
		}
		s.headerDone = true
	}

	// Flush previously pending frame with standard delay
	if len(s.pendingData) > 0 {
		if _, err := s.w.Write(s.pendingGCE); err != nil {
			return err
		}
		if _, err := s.w.Write(s.pendingData); err != nil {
			return err
		}
	}

	s.pendingGCE = []byte{
		0x21, 0xF9, 0x04,
		0x00, // disposal, flags
		byte(s.delay & 0xFF), byte((s.delay >> 8) & 0xFF),
		0x00, // transparent index
		0x00, // block terminator
	}
	s.pendingData = slices.Clone(raw[hdrLen : len(raw)-1])
	return nil
}

// Close flushes the final frame (with 3-second hold) and writes the GIF trailer.
func (s *StreamWriter) Close() error {
	if len(s.pendingData) > 0 {
		s.pendingGCE[4] = byte(300 & 0xFF)
		s.pendingGCE[5] = byte((300 >> 8) & 0xFF)
		if _, err := s.w.Write(s.pendingGCE); err != nil {
			return err
		}
		if _, err := s.w.Write(s.pendingData); err != nil {
			return err
		}
		s.pendingData = nil
	}
	_, err := s.w.Write([]byte{0x3B})
	return err
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
