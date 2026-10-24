package garden

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dpologdvinity/gitgarden/internal/frame"
	"github.com/dpologdvinity/gitgarden/internal/gitlog"
	"github.com/dpologdvinity/gitgarden/internal/golden"
)

// dated returns c with its time set to the given UTC date and its subject set to subj.
func dated(c gitlog.Commit, y, m, d int, subj string) gitlog.Commit {
	c.Time = time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	c.Subject = subj
	return c
}

// TestDrawOneAuthorSummer checks every row of a small summer frame against a hand-written picture.
func TestDrawOneAuthorSummer(t *testing.T) {
	g := Build("demo", []gitlog.Commit{
		dated(cm("Ada", "a@x", 10, 0), 2024, 5, 1, "a"),
		dated(cm("Ada", "a@x", 10, 0), 2024, 6, 1, "b"),
		dated(cm("Ada", "a@x", 10, 0), 2024, 7, 15, "add stuff"),
	})
	got := Draw(g, g.Snapshots(1)[0], 60, 12).String()
	want := "gitgarden - demo - 2024-07 summer" + strings.Repeat(" ", 16) + "3/3 commits\n" +
		"\n\n\n\n" +
		strings.Repeat(" ", 28) + "&|&\n" +
		strings.Repeat(" ", 28) + "&|&\n" +
		strings.Repeat(" ", 28) + "&|\n" +
		strings.Repeat(" ", 29) + "|\n" +
		strings.Repeat("=", 60) + "\n" +
		strings.Repeat(" ", 28) + "Ada\n" +
		"2024-07-15 Ada: +10 -0 add stuff\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

// TestDrawSanitizesNames checks that non-ASCII names and subjects become '?' and nothing above 0x7e is drawn.
func TestDrawSanitizesNames(t *testing.T) {
	g := Build("demo", []gitlog.Commit{dated(cm("李雷", "l@x", 1, 0), 2024, 1, 1, "héllo")})
	out := Draw(g, g.Snapshots(1)[0], 40, 12).String()
	if !strings.Contains(out, "??") || !strings.Contains(out, "h?llo") || strings.ContainsFunc(out, func(r rune) bool { return r > 0x7e }) {
		t.Fatalf("not sanitized:\n%s", out)
	}
}

// TestDrawTruncatesStatus checks that a very long subject is cut at the frame width.
func TestDrawTruncatesStatus(t *testing.T) {
	g := Build("demo", []gitlog.Commit{dated(cm("A", "a@x", 1, 0), 2024, 1, 1, strings.Repeat("s", 500))})
	lines := strings.Split(Draw(g, g.Snapshots(1)[0], 40, 12).String(), "\n")
	if len(lines[11]) != 40 {
		t.Fatalf("status row len %d", len(lines[11]))
	}
}

// TestDrawGolden compares two full frames (with wildflowers) with golden files.
func TestDrawGolden(t *testing.T) {
	var cs []gitlog.Commit
	for i := 0; i < 40; i++ { // 12 authors, uneven counts, months cycle
		a := i % 12
		if i >= 24 {
			a = i % 3
		}
		c := cm(fmt.Sprintf("Author%02d", a), fmt.Sprintf("a%02d@x", a), 30+i, i%4*3)
		cs = append(cs, dated(c, 2024, 1+i%12, 10, fmt.Sprintf("commit %d", i)))
	}
	g := Build("demo", cs)
	ss := g.Snapshots(4)
	golden.Assert(t, "w40_wildflowers_winter", Draw(g, ss[1], 40, 16).String()) // index 13, February
	golden.Assert(t, "w80_final", Draw(g, ss[3], 80, 24).String())
}

// TestLeafPalette checks the leaf glyph and color of the first four leaves in every season.
func TestLeafPalette(t *testing.T) {
	type lc struct {
		ch  rune
		col frame.Color
	}
	cases := []struct {
		season Season
		want   [4]lc
	}{
		{Spring, [4]lc{{'*', frame.Pink}, {'&', frame.LightGreen}, {'&', frame.LightGreen}, {'*', frame.Pink}}},
		{Summer, [4]lc{{'&', frame.Green}, {'&', frame.Green}, {'&', frame.Green}, {'&', frame.Green}}},
		{Autumn, [4]lc{{'&', frame.Orange}, {'&', frame.Red}, {'&', frame.Orange}, {'&', frame.Red}}},
		{Winter, [4]lc{{'*', frame.White}, {'*', frame.White}, {'*', frame.White}, {'*', frame.White}}},
	}
	for _, c := range cases {
		for j, w := range c.want {
			if ch, col := leaf(c.season, j); ch != w.ch || col != w.col {
				t.Errorf("%v leaf %d: got %c/%v want %c/%v", c.season, j, ch, col, w.ch, w.col)
			}
		}
	}
}

// TestAutumnFallenLeavesAreOrange checks that fallen leaves are drawn orange in autumn.
func TestAutumnFallenLeavesAreOrange(t *testing.T) {
	g := Build("demo", []gitlog.Commit{dated(cm("Ada", "a@x", 10, 0), 2024, 10, 1, "x")})
	fr := Draw(g, g.Snapshots(1)[0], 60, 12)
	if c := fr.At(26+3-3, 8); c.Ch != ',' || c.FG != frame.Orange {
		t.Fatalf("fallen leaf cell %+v", c)
	}
}

// TestDrawFarWildflowerStillGetsLabel checks that the "+N" label appears when the only active wildflower
// has a rank beyond the part of the stats a snapshot keeps.
func TestDrawFarWildflowerStillGetsLabel(t *testing.T) {
	cs := []gitlog.Commit{dated(cm("Last", "last@x", 1, 0), 2024, 1, 1, "first")}
	for i := 0; i < 1000; i++ {
		c := dated(cm(fmt.Sprintf("A%d", i), fmt.Sprintf("a%d@x", i), 5, 0), 2024, 1, 2, "x")
		cs = append(cs, c, c)
	}
	g := Build("demo", cs)
	out := Draw(g, g.Snapshots(2001)[0], 80, 24).String()
	if !strings.Contains(out, "+994") {
		t.Fatalf("wildflower label missing:\n%s", out)
	}
}
