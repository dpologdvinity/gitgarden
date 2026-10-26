// Package player plays frames in the terminal's alternate screen.
package player

import (
	"io"
	"strings"
	"time"
)

// pollEvery bounds how long Play sleeps before it looks at the keys again.
const pollEvery = 100 * time.Millisecond

type keyAction int

const (
	actNone keyAction = iota
	actQuit
	actPauseToggle
	actStepBack
	actStepForward
	actSpeedUp
	actSlowDown
)

func nextKey(keys <-chan byte) keyAction {
	select {
	case b := <-keys:
		switch b {
		case 'q', 'Q', 3:
			return actQuit
		case 27: // Esc or escape sequence
			select {
			case b2 := <-keys:
				if b2 == '[' {
					select {
					case b3 := <-keys:
						if b3 == 'D' {
							return actStepBack
						}
						if b3 == 'C' {
							return actStepForward
						}
					default:
					}
				}
				return actNone
			default:
				return actQuit
			}
		case ' ':
			return actPauseToggle
		case 'h', '<':
			return actStepBack
		case 'l', '>':
			return actStepForward
		case '+', ']':
			return actSpeedUp
		case '-', '[':
			return actSlowDown
		}
	default:
	}
	return actNone
}

// Play shows n frames, sleeping interval between them. It returns true if the
// user quit (q, Q, Ctrl-C or Esc) before the last frame finished.
func Play(w io.Writer, keys <-chan byte, n int, frameAt func(i int) string,
	interval time.Duration, sleep func(time.Duration)) (quit bool) {
	_, _ = io.WriteString(w, "\x1b[?1049h\x1b[?25l")
	defer func() { _, _ = io.WriteString(w, "\x1b[?25h\x1b[?1049l") }()

	draw := func(i int) {
		_, _ = io.WriteString(w, "\x1b[?2026h\x1b[H"+strings.ReplaceAll(frameAt(i), "\n", "\x1b[K\r\n")+"\x1b[K\x1b[?2026l")
	}

	paused := false
	i := 0
	for i < n {
		draw(i)

		for {
			switch nextKey(keys) {
			case actQuit:
				return true
			case actPauseToggle:
				paused = !paused
			case actStepBack:
				if i > 0 {
					i--
					draw(i)
				}
				continue
			case actStepForward:
				if i < n-1 {
					i++
					draw(i)
				}
				continue
			case actSpeedUp:
				interval = max(5*time.Millisecond, interval*3/4)
			case actSlowDown:
				interval = min(2*time.Second, interval*4/3)
			}

			if paused {
				sleep(pollEvery)
				continue
			}

			for left := interval; left > 0; {
				d := min(left, pollEvery)
				sleep(d)
				left -= d
				switch nextKey(keys) {
				case actQuit:
					return true
				case actPauseToggle:
					paused = true
				case actStepBack:
					if i > 0 {
						i--
						draw(i)
					}
				case actStepForward:
					if i < n-1 {
						i++
						draw(i)
					}
				case actSpeedUp:
					interval = max(5*time.Millisecond, interval*3/4)
				case actSlowDown:
					interval = min(2*time.Second, interval*4/3)
				}
				if paused {
					break
				}
			}

			if !paused {
				break
			}
		}

		i++
	}

	for left := time.Second; left > 0; {
		d := min(left, pollEvery)
		sleep(d)
		left -= d
		if nextKey(keys) == actQuit {
			return true
		}
	}
	return false
}
