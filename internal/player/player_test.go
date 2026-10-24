package player

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPlayAllFrames(t *testing.T) {
	var out bytes.Buffer
	var slept []time.Duration
	frames := []string{"A\nB", "C\nD"}
	quit := Play(&out, make(chan byte, 1), 2, func(i int) string { return frames[i] },
		100*time.Millisecond, func(d time.Duration) { slept = append(slept, d) })
	want := "\x1b[?1049h\x1b[?25l" +
		"\x1b[HA\x1b[K\r\nB\x1b[K" +
		"\x1b[HC\x1b[K\r\nD\x1b[K" +
		"\x1b[?25h\x1b[?1049l"
	if quit || out.String() != want {
		t.Fatalf("quit %v out %q", quit, out.String())
	}
	if !slices.Equal(slept, []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, time.Second}) {
		t.Fatalf("slept %v", slept)
	}
}

func TestPlayQuitKeys(t *testing.T) {
	for _, k := range []byte{'q', 'Q', 3, 27} {
		keys := make(chan byte, 1)
		keys <- k
		var out bytes.Buffer
		slept := 0
		quit := Play(&out, keys, 5, func(int) string { return "x" }, time.Millisecond, func(time.Duration) { slept++ })
		if !quit || strings.Count(out.String(), "\x1b[H") != 1 || slept != 0 || !strings.HasSuffix(out.String(), "\x1b[?1049l") {
			t.Fatalf("key %d: quit %v out %q slept %d", k, quit, out.String(), slept)
		}
	}
}

func TestPlayQuitDuringLongInterval(t *testing.T) {
	keys := make(chan byte, 1)
	var slept []time.Duration
	quit := Play(&bytes.Buffer{}, keys, 1, func(int) string { return "x" }, 10*time.Second,
		func(d time.Duration) {
			slept = append(slept, d)
			if len(slept) == 1 {
				keys <- 'q'
			}
		})
	if !quit || len(slept) != 1 || slept[0] > 100*time.Millisecond {
		t.Fatalf("quit %v slept %v", quit, slept)
	}
}
