package gifenc

import (
	"bytes"
	"image"
	"image/gif"
	"slices"
	"testing"

	"github.com/dpologdvinity/gitgarden/internal/frame"
)

// TestSample checks the sampled indices for empty, single, short and long inputs.
func TestSample(t *testing.T) {
	cases := []struct {
		n, max int
		want   []int
	}{
		{0, 5, []int{}}, {3, 1, []int{2}}, {3, 5, []int{0, 1, 2}},
		{10, 4, []int{0, 3, 6, 9}}, {7, 3, []int{0, 3, 6}},
	}
	for _, c := range cases {
		if got := Sample(c.n, c.max); !slices.Equal(got, c.want) {
			t.Errorf("Sample(%d,%d)=%v want %v", c.n, c.max, got, c.want)
		}
	}
}

// TestDelay checks the speed-to-delay conversion and its 2-centisecond floor.
func TestDelay(t *testing.T) {
	if Delay(1) != 10 || Delay(4) != 3 || Delay(5) != 2 || Delay(100) != 2 || Delay(0.5) != 20 {
		t.Fatal("delay")
	}
}

// TestImageColors checks the image size, a background pixel and that a glyph is drawn in its color.
func TestImageColors(t *testing.T) {
	g := frame.New(2, 1)
	g.SetBG(0, 0, frame.Bar)
	g.Set(1, 0, 'A', frame.White)
	img := Image(g)
	if img.Bounds() != image.Rect(0, 0, 14, 13) {
		t.Fatalf("bounds %v", img.Bounds())
	}
	if img.ColorIndexAt(0, 0) != uint8(frame.Bar) {
		t.Fatal("bg of cell 0 must be Bar")
	}
	white := 0
	for y := 0; y < 13; y++ {
		for x := 7; x < 14; x++ {
			if img.ColorIndexAt(x, y) == uint8(frame.White) {
				white++
			}
		}
	}
	if white == 0 {
		t.Fatal("glyph A not drawn")
	}
}

// TestWriteRoundTrip encodes frames and decodes them again to check the frame count, delays, loop count and size.
func TestWriteRoundTrip(t *testing.T) {
	gs := []*frame.Grid{frame.New(2, 1), frame.New(2, 1), frame.New(2, 1)}
	var buf bytes.Buffer
	if err := Write(&buf, gs, 5); err != nil {
		t.Fatal(err)
	}
	a, err := gif.DecodeAll(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Image) != 3 || !slices.Equal(a.Delay, []int{5, 5, 300}) || a.LoopCount != 0 || a.Config.Width != 14 {
		t.Fatalf("frames %d delays %v loop %d w %d", len(a.Image), a.Delay, a.LoopCount, a.Config.Width)
	}
}
