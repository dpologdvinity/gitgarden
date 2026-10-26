package player

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestPlayAllFrames checks the exact bytes written and the sleeps for a full playback.
func TestPlayAllFrames(t *testing.T) {
	var out bytes.Buffer
	var slept []time.Duration
	frames := []string{"A\nB", "C\nD"}
	quit := Play(&out, make(chan byte, 1), 2, func(i int) string { return frames[i] },
		100*time.Millisecond, func(d time.Duration) { slept = append(slept, d) })
	want := "\x1b[?1049h\x1b[?25l" +
		"\x1b[?2026h\x1b[HA\x1b[K\r\nB\x1b[K\x1b[?2026l" +
		"\x1b[?2026h\x1b[HC\x1b[K\r\nD\x1b[K\x1b[?2026l" +
		"\x1b[?25h\x1b[?1049l"
	if quit || out.String() != want {
		t.Fatalf("quit %v out %q", quit, out.String())
	}
	wantSlept := append([]time.Duration{100 * time.Millisecond, 100 * time.Millisecond}, slices.Repeat([]time.Duration{100 * time.Millisecond}, 10)...)
	if !slices.Equal(slept, wantSlept) {
		t.Fatalf("slept %v", slept)
	}
}

// TestPlayQuitKeys checks that q, Q, Ctrl-C and Esc each stop playback after the first frame.
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

// TestPlayQuitDuringLongInterval checks that a key pressed during a long frame interval is noticed
// within one polling step instead of after the whole interval.
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

// TestPlayQuitDuringFinalHold checks that hitting quit during the final 1-second hold exits promptly.
func TestPlayQuitDuringFinalHold(t *testing.T) {
	keys := make(chan byte, 1)
	var slept []time.Duration
	quit := Play(&bytes.Buffer{}, keys, 1, func(int) string { return "x" }, 10*time.Millisecond,
		func(d time.Duration) {
			slept = append(slept, d)
			if len(slept) == 2 {
				keys <- 'q'
			}
		})
	if !quit || len(slept) > 3 {
		t.Fatalf("quit %v slept %v", quit, slept)
	}
}

// TestPlayPauseAndResume checks that Space pauses playback and subsequent Space resumes.
func TestPlayPauseAndResume(t *testing.T) {
	keys := make(chan byte, 5)
	var rendered []int
	var unpauseSent bool
	quit := Play(&bytes.Buffer{}, keys, 3, func(i int) string {
		rendered = append(rendered, i)
		return fmt.Sprintf("frame %d", i)
	}, 10*time.Millisecond, func(d time.Duration) {
		if len(rendered) == 1 && !unpauseSent {
			keys <- ' ' // pause
			unpauseSent = true
			go func() {
				time.Sleep(10 * time.Millisecond)
				keys <- ' ' // resume
			}()
		}
	})
	if quit || len(rendered) < 3 {
		t.Fatalf("quit %v rendered %v", quit, rendered)
	}
}

// TestPlayStepFrames checks that 'h' (step back) and 'l' (step forward) work while paused.
func TestPlayStepFrames(t *testing.T) {
	keys := make(chan byte, 10)
	var rendered []int
	quit := Play(&bytes.Buffer{}, keys, 5, func(i int) string {
		rendered = append(rendered, i)
		return fmt.Sprintf("frame %d", i)
	}, 10*time.Millisecond, func(d time.Duration) {
		if len(rendered) == 2 {
			keys <- ' ' // pause after frame 1
			keys <- 'h' // step back to 0
			keys <- 'l' // step forward to 1
			keys <- 'q' // quit
		}
	})
	if !quit {
		t.Fatal("expected quit")
	}
	// Sequence should have visited 0, 1, then step back to 0, then step forward to 1
	if !slices.Equal(rendered, []int{0, 1, 0, 1}) {
		t.Fatalf("expected [0 1 0 1], got %v", rendered)
	}
}
