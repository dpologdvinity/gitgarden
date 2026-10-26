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
		case 27: // Esc or escape sequence like \x1b[D
			select {
			case b2 := <-keys:
				if b2 == '[' {
					select {
					case b3 := <-keys:
						switch b3 {
						case 'D':
							return actStepBack
						case 'C':
							return actStepForward
						}
					case <-time.After(25 * time.Millisecond):
					}
				}
				return actNone
			case <-time.After(25 * time.Millisecond):
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
	draw(i)

	timerLeft := interval
	for i < n {
		act := nextKey(keys)
		switch act {
		case actQuit:
			return true
		case actPauseToggle:
			paused = !paused
		case actStepBack:
			if i > 0 {
				i--
				draw(i)
			}
			timerLeft = interval
			continue
		case actStepForward:
			if i < n-1 {
				i++
				draw(i)
			}
			timerLeft = interval
			continue
		case actSpeedUp:
			interval = max(5*time.Millisecond, interval*3/4)
			timerLeft = min(timerLeft, interval)
		case actSlowDown:
			interval = min(2*time.Second, interval*4/3)
		}

		if paused {
			sleep(pollEvery)
			continue
		}

		d := min(timerLeft, pollEvery)
		sleep(d)
		timerLeft -= d

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
			timerLeft = interval
			continue
		case actStepForward:
			if i < n-1 {
				i++
				draw(i)
			}
			timerLeft = interval
			continue
		case actSpeedUp:
			interval = max(5*time.Millisecond, interval*3/4)
			timerLeft = min(timerLeft, interval)
		case actSlowDown:
			interval = min(2*time.Second, interval*4/3)
		}

		if !paused && timerLeft <= 0 {
			if i == n-1 {
				break
			}
			i++
			draw(i)
			timerLeft = interval
		}
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
