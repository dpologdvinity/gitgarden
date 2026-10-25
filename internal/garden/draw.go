package garden

import (
	"fmt"

	"github.com/dpologdvinity/gitgarden/internal/frame"
)

// wildColors is the repeating color cycle for wildflowers, indexed by wildflower number.
var wildColors = [...]frame.Color{frame.Pink, frame.Yellow, frame.Purple, frame.Blue, frame.Cyan, frame.Red}

// Sanitize replaces every rune outside printable ASCII with '?'.
func Sanitize(s string) string {
	r := []rune(s)
	for i, c := range r {
		if c < 0x20 || c > 0x7e {
			r[i] = '?'
		}
	}
	return string(r)
}

// leaf returns the glyph and color of leaf number j for the season (SPEC section 7.2).
// Spring mixes pink blossoms in, autumn alternates orange and red, winter is white snow.
func leaf(season Season, j int) (rune, frame.Color) {
	switch season {
	case Spring:
		if j%3 == 0 {
			return '*', frame.Pink
		}
		return '&', frame.LightGreen
	case Summer:
		return '&', frame.Green
	case Autumn:
		if j%2 == 0 {
			return '&', frame.Orange
		}
		return '&', frame.Red
	}
	return '*', frame.White
}

// truncate cuts s to at most n runes. It is used on already sanitized (ASCII) text.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// Draw renders one snapshot into a w x h grid.
func Draw(g *Garden, s Snapshot, w, h int) *frame.Grid {
	fr := frame.New(w, h)
	cur := g.Commits[s.Index]
	season := SeasonOf(cur.Time)
	b := h - 4

	// title
	right := fmt.Sprintf("%d/%d commits", s.Index+1, len(g.Commits))
	left := truncate(fmt.Sprintf("gitgarden - %s - %s %s", Sanitize(g.Repo), cur.Time.Format("2006-01"), season), w-len([]rune(right))-1)
	fr.Text(0, 0, left, frame.FG)
	fr.Text(w-len([]rune(right)), 0, right, frame.FG)

	// sky: the plant area gets a blue background; glyphs drawn later keep it
	for y := 1; y <= b; y++ {
		for x := 0; x < w; x++ {
			fr.SetBG(x, y, frame.Sky)
		}
	}

	// soil
	soil := frame.Brown
	if season == Winter {
		soil = frame.White
	}
	for x := 0; x < w; x++ {
		fr.Set(x, b+1, '=', soil)
	}

	slots, firstWild := Slots(w, len(g.Authors))
	curRank := g.RankOf(cur)
	for _, sl := range slots {
		cx := sl.X0 + 3
		var label string
		var labelYellow bool
		if sl.Author >= 0 {
			st := s.Stats[sl.Author]
			if st.Commits == 0 {
				continue
			}
			drawPlant(fr, st, cx, b, h, season)
			label = Sanitize(g.Authors[sl.Author].Name)
			labelYellow = sl.Author == curRank
		} else {
			if s.MaxRank < firstWild {
				continue
			}
			for j := 0; j < 7*b && firstWild+j < len(s.Stats); j++ {
				if s.Stats[firstWild+j].Commits > 0 {
					fr.Set(sl.X0+j%7, b-j/7, '*', wildColors[j%6])
				}
			}
			label = fmt.Sprintf("+%d", len(g.Authors)-firstWild)
			labelYellow = curRank >= firstWild
		}
		label = truncate(label, 7)
		col := frame.FG
		if labelYellow {
			col = frame.Yellow
		}
		fr.Text(sl.X0+(7-len([]rune(label)))/2, h-2, label, col)
	}

	// status
	huge := ""
	if cur.Huge {
		huge = " [huge]"
	}
	status := fmt.Sprintf("%s %s: +%d -%d%s %s", cur.Time.Format("2006-01-02"), Sanitize(cur.Name), cur.Added, cur.Deleted, huge, Sanitize(cur.Subject))
	fr.Text(0, h-1, truncate(status, w), frame.Dim)
	return fr
}

// drawPlant draws one author's plant into fr: the stem, then leaves in candidate order,
// then fallen leaves on the base row. cx is the center column, b the base row and h the frame height.
func drawPlant(fr *frame.Grid, st Stats, cx, b, h int, season Season) {
	stem := StemHeight(st.Commits, h)
	for y := b - stem + 1; y <= b; y++ {
		fr.Set(cx, y, '|', frame.Brown)
	}
	if stem >= 6 {
		fr.Set(cx, b-stem+1, 'Y', frame.Brown)
	}
	cells := LeafCells(cx, b, stem)
	for j := 0; j < LeafCount(st, len(cells)); j++ {
		ch, col := leaf(season, j)
		fr.Set(cells[j][0], cells[j][1], ch, col)
	}
	fcol := frame.Brown
	if season == Autumn {
		fcol = frame.Orange
	}
	for _, o := range []int{-3, 3, -2, 2, -1, 1}[:Fallen(st, season)] {
		fr.Set(cx+o, b, ',', fcol)
	}
}
