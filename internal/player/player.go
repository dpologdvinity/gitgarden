// Package player plays frames in the terminal's alternate screen.
package player

import (
	"io"
	"strings"
	"time"
)

// pollEvery bounds how long Play sleeps before it looks at the keys again.
const pollEvery = 100 * time.Millisecond

// minInterval and maxInterval bound the frame interval reachable with the + and - keys.
const (
	minInterval = 5 * time.Millisecond
	maxInterval = 2 * time.Second
)

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

	left := interval // time until the next frame
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
			left = interval
			continue // seeks are handled back to back, without a sleep in between
		case actStepForward:
			if i < n-1 {
				i++
				draw(i)
			}
			left = interval
			continue
		case actSpeedUp:
			// Clamp only in the direction of the change, so a key never reverses a --speed outside the range.
			if interval > minInterval {
				interval = max(minInterval, interval*3/4)
			}
			left = min(left, interval)
		case actSlowDown:
			if interval < maxInterval {
				interval = min(maxInterval, interval*4/3)
			}
		}
		if paused {
			sleep(pollEvery)
			continue
		}
		if left <= 0 {
			if i == n-1 {
				break
			}
			i++
			draw(i)
			left = interval
			continue
		}
		d := min(left, pollEvery)
		sleep(d)
		left -= d
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
