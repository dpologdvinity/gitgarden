// Package player plays frames in the terminal's alternate screen.
package player

import (
	"io"
	"strings"
	"time"
)

// Play shows n frames, sleeping interval between them. It returns true if the
// user quit (q, Q, Ctrl-C or Esc) before the last frame finished.
func Play(w io.Writer, keys <-chan byte, n int, frameAt func(i int) string,
	interval time.Duration, sleep func(time.Duration)) (quit bool) {
	io.WriteString(w, "\x1b[?1049h\x1b[?25l")
	defer io.WriteString(w, "\x1b[?25h\x1b[?1049l")
	for i := 0; i < n; i++ {
		io.WriteString(w, "\x1b[H"+strings.ReplaceAll(frameAt(i), "\n", "\x1b[K\r\n")+"\x1b[K")
	drain:
		for {
			select {
			case b := <-keys:
				if b == 'q' || b == 'Q' || b == 3 || b == 27 {
					return true
				}
			default:
				break drain
			}
		}
		sleep(interval)
	}
	sleep(time.Second)
	return false
}
