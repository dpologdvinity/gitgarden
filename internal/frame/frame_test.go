package frame

import "testing"

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
