// Package player plays frames in the terminal's alternate screen.
package player

import (
	"io"
	"strings"
	"time"
)

// pollEvery bounds how long Play sleeps before it looks at the keys again.
const pollEvery = 100 * time.Millisecond

// Play shows n frames, sleeping interval between them. It returns true if the
// user quit (q, Q, Ctrl-C or Esc) before the last frame finished.
func Play(w io.Writer, keys <-chan byte, n int, frameAt func(i int) string,
	interval time.Duration, sleep func(time.Duration)) (quit bool) {
	io.WriteString(w, "\x1b[?1049h\x1b[?25l")
	defer io.WriteString(w, "\x1b[?25h\x1b[?1049l")
	quitKey := func() bool {
		for {
			select {
			case b := <-keys:
				if b == 'q' || b == 'Q' || b == 3 || b == 27 {
					return true
				}
			default:
				return false
			}
		}
	}
	for i := 0; i < n; i++ {
		io.WriteString(w, "\x1b[H"+strings.ReplaceAll(frameAt(i), "\n", "\x1b[K\r\n")+"\x1b[K")
		if quitKey() {
			return true
		}
		// sleep in short steps so a long interval (low --speed) still reacts to keys
		for left := interval; left > 0; {
			d := min(left, pollEvery)
			sleep(d)
			left -= d
			if quitKey() {
				return true
			}
		}
	}
	sleep(time.Second)
	return false
}
