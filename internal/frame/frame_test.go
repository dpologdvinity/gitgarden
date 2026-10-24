package frame

import "testing"

// TestStringTrimsAndClips checks plain rendering: text is clipped at the grid edge, out-of-range writes
// are ignored, and trailing spaces are removed.
func TestStringTrimsAndClips(t *testing.T) {
	g := New(5, 3)
	g.Text(1, 0, "hello world", FG) // clipped at W
	g.Set(4, 2, '*', Red)
	g.Set(9, 9, 'x', Red) // out of bounds: no-op
	if got, want := g.String(), " hell\n\n    *\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if c := g.At(4, 2); c != (Cell{'*', Red, BG}) {
		t.Fatalf("cell %+v", c)
	}
}

// TestANSI checks the exact escape sequences, including that a color sequence is only emitted on change.
func TestANSI(t *testing.T) {
	g := New(2, 1)
	g.Set(0, 0, 'a', FG)
	g.Set(1, 0, 'b', Red)
	g.SetBG(1, 0, Bar)
	want := "\x1b[0m\x1b[38;2;208;208;208ma" +
		"\x1b[0m\x1b[38;2;224;108;117m\x1b[48;2;44;49;58mb\x1b[0m\n"
	if got := g.ANSI(); got != want {
		t.Fatalf("got %q", got)
	}
}

// TestSkyColor checks that Sky is the last palette entry, so existing color indexes do not move.
func TestSkyColor(t *testing.T) {
	if Sky != 16 || len(Palette) != 17 || Palette[Sky] != [3]uint8{0x3d, 0x85, 0xc6} {
		t.Fatalf("Sky %d, palette size %d", Sky, len(Palette))
	}
}
