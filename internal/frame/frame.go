// Package frame is a grid of colored character cells with plain and ANSI rendering.
package frame

import (
	"fmt"
	"strings"
)

// Color is an index into Palette. BG is the frame background; FG is the default text color.
type Color uint8

// The palette indexes. The order is fixed because the GIF palette uses the same indexes.
const (
	BG Color = iota
	FG
	Dim
	Green
	DarkGreen
	Brown
	Yellow
	Red
	Orange
	White
	Blue
	Pink
	Cyan
	Bar
	Purple
	LightGreen
	Sky // background of the plant area
)

// Palette holds the RGB value of each Color, in index order.
var Palette = [17][3]uint8{
	{0x10, 0x14, 0x18}, {0xd0, 0xd0, 0xd0}, {0x6c, 0x6c, 0x6c}, {0x5f, 0xaf, 0x5f},
	{0x2e, 0x7d, 0x32}, {0x8d, 0x6e, 0x63}, {0xe5, 0xc0, 0x7b}, {0xe0, 0x6c, 0x75},
	{0xd1, 0x9a, 0x66}, {0xff, 0xff, 0xff}, {0x61, 0xaf, 0xef}, {0xf4, 0x8f, 0xb1},
	{0x56, 0xb6, 0xc2}, {0x2c, 0x31, 0x3a}, {0xc6, 0x78, 0xdd}, {0x98, 0xc3, 0x79},
	{0x3d, 0x85, 0xc6},
}

// Cell is one character position: a rune with a foreground and a background color.
type Cell struct {
	Ch     rune
	FG, BG Color
}

// Grid is a W x H array of cells stored row by row: the cell at (x, y) is Cells[y*W+x].
type Grid struct {
	W, H  int
	Cells []Cell // row-major
}

// New returns a w x h grid filled with spaces in the default colors.
func New(w, h int) *Grid {
	g := &Grid{W: w, H: h, Cells: make([]Cell, w*h)}
	for i := range g.Cells {
		g.Cells[i] = Cell{' ', FG, BG}
	}
	return g
}

// in reports whether (x, y) lies inside the grid.
func (g *Grid) in(x, y int) bool { return x >= 0 && y >= 0 && x < g.W && y < g.H }

// At returns the cell at (x, y). The caller must pass in-range coordinates.
func (g *Grid) At(x, y int) Cell { return g.Cells[y*g.W+x] }

// Set writes ch with color fg at (x, y), keeping the background. Out-of-range coordinates are ignored.
func (g *Grid) Set(x, y int, ch rune, fg Color) {
	if g.in(x, y) {
		c := &g.Cells[y*g.W+x]
		c.Ch, c.FG = ch, fg
	}
}

// SetBG sets the background color at (x, y). Out-of-range coordinates are ignored.
func (g *Grid) SetBG(x, y int, bg Color) {
	if g.in(x, y) {
		g.Cells[y*g.W+x].BG = bg
	}
}

// Text writes one rune per cell starting at (x, y), clipped at the grid edge.
func (g *Grid) Text(x, y int, s string, fg Color) {
	for _, r := range s {
		g.Set(x, y, r, fg)
		x++
	}
}

// String renders the grid as plain text: one line per row, trailing spaces removed,
// and a newline after every row including the last. Colors are dropped.
func (g *Grid) String() string {
	var sb strings.Builder
	for y := 0; y < g.H; y++ {
		row := make([]rune, g.W)
		for x := range row {
			row[x] = g.At(x, y).Ch
		}
		sb.WriteString(strings.TrimRight(string(row), " "))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// ANSI renders the grid with 24-bit color escape codes. It emits a new color sequence only when
// the colors change, resets at the end of each row, and does not trim trailing spaces.
func (g *Grid) ANSI() string {
	var sb strings.Builder
	for y := 0; y < g.H; y++ {
		var prev Cell
		for x := 0; x < g.W; x++ {
			c := g.At(x, y)
			if x == 0 || c.FG != prev.FG || c.BG != prev.BG {
				p := Palette[c.FG]
				fmt.Fprintf(&sb, "\x1b[0m\x1b[38;2;%d;%d;%dm", p[0], p[1], p[2])
				if c.BG != BG {
					p = Palette[c.BG]
					fmt.Fprintf(&sb, "\x1b[48;2;%d;%d;%dm", p[0], p[1], p[2])
				}
			}
			sb.WriteRune(c.Ch)
			prev = c
		}
		sb.WriteString("\x1b[0m\n")
	}
	return sb.String()
}
