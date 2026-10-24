package garden

import (
	"slices"
	"testing"
	"time"
)

// x0s returns the left column of each slot.
func x0s(s []Slot) []int {
	var out []int
	for _, v := range s {
		out = append(out, v.X0)
	}
	return out
}

// TestSlots checks slot positions and the wildflower cut-over for 1, 8, 9, 12 and 200 authors at several widths.
func TestSlots(t *testing.T) {
	s, fw := Slots(80, 1)
	if !slices.Equal(x0s(s), []int{36}) || fw != 1 {
		t.Fatalf("1 author: %v %d", x0s(s), fw)
	}
	s, fw = Slots(80, 8)
	if len(s) != 8 || s[0].X0 != 5 || s[7].X0 != 68 || fw != 8 {
		t.Fatalf("8 authors: %v %d", x0s(s), fw)
	}
	s, fw = Slots(80, 9)
	if len(s) != 8 || fw != 7 || s[7].Author != -1 || s[6].Author != 6 {
		t.Fatalf("9 authors: %+v %d", s, fw)
	}
	s, fw = Slots(40, 12)
	if !slices.Equal(x0s(s), []int{3, 12, 21, 30}) || fw != 3 {
		t.Fatalf("w40: %v %d", x0s(s), fw)
	}
	s, fw = Slots(160, 200)
	if len(s) != 17 || s[0].X0 != 4 || fw != 16 {
		t.Fatalf("200 authors: %v %d", x0s(s), fw)
	}
}

// TestStemHeight checks the stem height formula and its cap by frame height.
func TestStemHeight(t *testing.T) {
	for _, c := range [][3]int{{0, 24, 0}, {1, 24, 2}, {3, 24, 4}, {4, 24, 6}, {1000, 24, 19}, {1000, 12, 7}} {
		if got := StemHeight(c[0], c[1]); got != c[2] {
			t.Errorf("StemHeight(%d,%d)=%d want %d", c[0], c[1], got, c[2])
		}
	}
}

// TestLeafCells checks the number and order of leaf candidate cells for a 6-row stem.
func TestLeafCells(t *testing.T) {
	c := LeafCells(10, 20, 6)
	if len(c) != 18 || c[0] != [2]int{9, 15} || c[1] != [2]int{11, 15} ||
		c[6] != [2]int{8, 17} || c[17] != [2]int{13, 19} {
		t.Fatalf("%v", c)
	}
}

// TestLeafCountAndFallen checks the leaf and fallen-leaf counts, including the autumn bonus and the caps.
func TestLeafCountAndFallen(t *testing.T) {
	if LeafCount(Stats{5, 300, 50}, 18) != 15 || LeafCount(Stats{5, 10, 50}, 18) != 0 || LeafCount(Stats{5, 10000, 0}, 18) != 18 {
		t.Fatal("LeafCount")
	}
	if Fallen(Stats{1, 0, 0}, Summer) != 0 || Fallen(Stats{1, 0, 64}, Summer) != 2 ||
		Fallen(Stats{1, 0, 64}, Autumn) != 4 || Fallen(Stats{1, 0, 10000}, Autumn) != 6 || Fallen(Stats{1, 0, 0}, Autumn) != 2 {
		t.Fatal("Fallen")
	}
}

// TestSeasonOf checks the season of every month.
func TestSeasonOf(t *testing.T) {
	want := []Season{Winter, Winter, Spring, Spring, Spring, Summer, Summer, Summer, Autumn, Autumn, Autumn, Winter}
	for m := 1; m <= 12; m++ {
		if got := SeasonOf(time.Date(2024, time.Month(m), 1, 0, 0, 0, 0, time.UTC)); got != want[m-1] {
			t.Errorf("month %d: %v", m, got)
		}
	}
	if Autumn.String() != "autumn" {
		t.Fatal("String")
	}
}
