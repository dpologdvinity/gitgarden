package garden

import (
	"math"
	"math/bits"
	"time"
)

type Season int

const (
	Winter Season = iota
	Spring
	Summer
	Autumn
)

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
type Slot struct{ X0, Author int }

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

func LeafCount(s Stats, nCandidates int) int {
	return min(nCandidates, isqrt(s.Added-s.Deleted))
}

func Fallen(s Stats, season Season) int {
	f := min(6, isqrt(s.Deleted)/4)
	if season == Autumn {
		f = min(6, f+2)
	}
	return f
}
