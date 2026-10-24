package garden

import (
	"math"
	"math/bits"
	"time"
)

// Season is one of the four seasons, derived from the month of a commit (northern hemisphere).
type Season int

// The seasons.
const (
	Winter Season = iota
	Spring
	Summer
	Autumn
)

// String returns the lower-case season name used in the title row.
func (s Season) String() string {
	return [...]string{"winter", "spring", "summer", "autumn"}[s]
}

// SeasonOf maps the UTC month to a northern-hemisphere season.
func SeasonOf(t time.Time) Season {
	switch t.UTC().Month() {
	case time.December, time.January, time.February:
		return Winter
	case time.March, time.April, time.May:
		return Spring
	case time.June, time.July, time.August:
		return Summer
	}
	return Autumn
}

// Slot is one 9-column plant cell; Author is a rank, or -1 for the wildflower slot.
type Slot struct {
	X0     int // left column of the 7-column plant area
	Author int // author rank, or -1 for the wildflower slot
}

// Slots lays out plant slots for width w. firstWild is the rank of the first
// wildflower author, or nAuthors if there are none.
func Slots(w, nAuthors int) (slots []Slot, firstWild int) {
	maxSlots := (w - 2) / 9
	plants, wild := nAuthors, false
	if nAuthors > maxSlots {
		plants, wild = maxSlots-1, true
	}
	n := plants
	if wild {
		n++
	}
	x0 := func(k int) int { return (w-n*9)/2 + 1 + k*9 }
	for k := 0; k < plants; k++ {
		slots = append(slots, Slot{x0(k), k})
	}
	if wild {
		slots = append(slots, Slot{x0(plants), -1})
	}
	return slots, plants
}

// isqrt returns floor(sqrt(n)), or 0 for n <= 0.
func isqrt(n int) int {
	if n <= 0 {
		return 0
	}
	return int(math.Sqrt(float64(n)))
}

// StemHeight is the stem height for a plant with the given commits in a frame h rows tall.
func StemHeight(commits, h int) int {
	return min(2*bits.Len(uint(commits)), h-5)
}

// LeafCells lists leaf candidate cells (x, y) in fill order for a stem of the given height.
func LeafCells(cx, b, stem int) [][2]int {
	var out [][2]int
	top := b - stem + 1
	for i := 0; top+i <= b-1; i++ {
		lim := min(3, 1+i/2)
		for _, o := range [...]int{-1, 1, -2, 2, -3, 3} {
			if max(o, -o) <= lim {
				out = append(out, [2]int{cx + o, top + i})
			}
		}
	}
	return out
}

// LeafCount returns how many leaves a plant shows: the square root of its net added lines,
// limited to the number of candidate cells.
func LeafCount(s Stats, nCandidates int) int {
	return min(nCandidates, isqrt(s.Added-s.Deleted))
}

// Fallen returns how many fallen leaves lie at the foot of a plant: grows with deleted lines,
// gets 2 extra in autumn, and never exceeds 6.
func Fallen(s Stats, season Season) int {
	f := min(6, isqrt(s.Deleted)/4)
	if season == Autumn {
		f = min(6, f+2)
	}
	return f
}
