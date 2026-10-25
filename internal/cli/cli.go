// Package cli parses flags, picks a mode and maps errors to exit codes.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dpologdvinity/gitgarden/internal/frame"
	"github.com/dpologdvinity/gitgarden/internal/garden"
	"github.com/dpologdvinity/gitgarden/internal/gifenc"
	"github.com/dpologdvinity/gitgarden/internal/gitlog"
	"github.com/dpologdvinity/gitgarden/internal/player"
)

// usage is the help text printed by -h (stdout) and after usage errors (stderr).
// It must match SPEC section 3.3 exactly; a test compares against it.
const usage = `usage: gitgarden [flags]

Plays a repository's git history as an ASCII garden.

flags:
  -C dir          repository directory (default ".")
  --since date    only plant commits after date (any git date)
  --speed x       playback speed multiplier (default 1)
  --still         print only the final frame
  --gif file      write an animated GIF instead of playing
  --width n       frame width in cells, 40-160 (default: terminal)
  --height n      frame height in cells, 12-60 (default: terminal)
  --no-color      disable color
  --southern      use southern-hemisphere seasons
  --version       print version

keys while playing: q quit
`

// Env holds everything Run needs from the real OS, so tests can fake it.
type Env struct {
	// Version is printed by --version.
	Version string
	// Stdin is read one byte at a time for key presses while playing.
	Stdin io.Reader
	// StdoutTTY reports whether stdout is a terminal. It selects play mode and color.
	StdoutTTY bool
	// TermSize returns the terminal size in columns and rows.
	TermSize func() (w, h int, err error)
	// MakeRaw puts the terminal in raw mode so single key presses are readable, and returns a
	// function that undoes it. It is nil when stdin is not a terminal.
	MakeRaw func() (restore func(), err error)
	// Sleep pauses between frames. Tests replace it with a no-op so playback is instant.
	Sleep func(time.Duration)
	// Getenv reads an environment variable (only NO_COLOR is used).
	Getenv func(string) string
}

// Run executes gitgarden with the given arguments and returns the process exit code.
// It writes the garden (or GIF summary) to stdout and errors to stderr. All access to
// the terminal, the clock and the environment goes through env, so tests can fake them.
// Exit codes: 0 success, 1 no commits to plant, 2 usage or git error (SPEC section 3.4).
func Run(args []string, stdout, stderr io.Writer, env Env) int {
	// Parse flags. The flag package prints its own error line to stderr; Run adds the usage text.
	fs := flag.NewFlagSet("gitgarden", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {}
	dir := fs.String("C", ".", "")
	since := fs.String("since", "", "")
	speed := fs.Float64("speed", 1, "")
	still := fs.Bool("still", false, "")
	gifPath := fs.String("gif", "", "")
	width := fs.Int("width", 0, "")
	height := fs.Int("height", 0, "")
	noColor := fs.Bool("no-color", false, "")
	southern := fs.Bool("southern", false, "")
	version := fs.Bool("version", false, "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, usage)
			return 0
		}
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	if *version {
		_, _ = fmt.Fprintf(stdout, "gitgarden %s\n", env.Version)
		return 0
	}
	// Validate flags in a fixed order so each invalid input has exactly one error message.
	fail := func(msg string) int {
		_, _ = fmt.Fprintf(stderr, "gitgarden: %s\n", msg)
		return 2
	}
	switch {
	case fs.NArg() > 0:
		return fail("unexpected argument: " + fs.Arg(0))
	case !(*speed > 0 && *speed <= 100):
		return fail("--speed must be greater than 0 and at most 100")
	case *width != 0 && (*width < 40 || *width > 160):
		return fail("--width must be between 40 and 160")
	case *height != 0 && (*height < 12 || *height > 60):
		return fail("--height must be between 12 and 60")
	case *still && *gifPath != "":
		return fail("--still and --gif cannot be used together")
	case *gifPath != "" && (*width > 120 || *height > 40):
		return fail("--gif supports at most 120x40 cells")
	}

	// Read history, then turn git's errors into the messages and exit codes of SPEC section 3.4.
	repo, commits, err := gitlog.Load(*dir, *since)
	switch {
	case errors.Is(err, gitlog.ErrNotRepo):
		return fail("not a git repository: " + *dir)
	case errors.Is(err, gitlog.ErrNoCommits):
		_, _ = fmt.Fprintf(stderr, "gitgarden: %v\n", err)
		return 1
	case err != nil:
		return fail(err.Error())
	}

	// Build the garden model and choose how to render a frame (plain text or ANSI color).
	w, h := frameSize(env, *width, *height, *gifPath != "")
	g := garden.Build(repo, commits)
	g.Southern = *southern
	color := env.StdoutTTY && env.Getenv("NO_COLOR") == "" && !*noColor
	render := func(gr *frame.Grid) string {
		if color {
			return gr.ANSI()
		}
		return gr.String()
	}
	stillFrame := func() string {
		return render(garden.Draw(g, g.Snapshots(1)[0], w, h))
	}

	// Mode selection, first match wins (SPEC section 3.1): GIF, still, non-terminal stdout, play.
	switch {
	case *gifPath != "":
		ss := g.Snapshots(300)
		grids := make([]*frame.Grid, len(ss))
		for i, sn := range ss {
			grids[i] = garden.Draw(g, sn, w, h)
		}
		if err := writeGIF(*gifPath, grids, gifenc.Delay(*speed)); err != nil {
			return fail(fmt.Sprintf("cannot write %s: %v", *gifPath, err))
		}
		_, _ = fmt.Fprintf(stdout, "wrote %s (%d frames, %dx%d)\n", *gifPath, len(grids), w*gifenc.CellW, h*gifenc.CellH)
	case *still || !env.StdoutTTY:
		_, _ = io.WriteString(stdout, stillFrame())
	default:
		ss := g.Snapshots(600)
		restore := func() {}
		if env.MakeRaw != nil {
			if r, err := env.MakeRaw(); err == nil {
				restore = r
			}
		}
		// Forward key presses to the player. The send is non-blocking so a held key never stalls the reader.
		keys := make(chan byte, 16)
		go func() {
			buf := make([]byte, 1)
			for {
				if n, err := env.Stdin.Read(buf); n > 0 {
					select {
					case keys <- buf[0]:
					default:
					}
				} else if err != nil {
					return
				}
			}
		}()
		interval := time.Duration(float64(100*time.Millisecond) / *speed)
		quit := player.Play(stdout, keys, len(ss), func(i int) string {
			return render(garden.Draw(g, ss[i], w, h))
		}, interval, env.Sleep)
		restore() // before the final frame: raw mode disables \n -> \r\n
		if !quit {
			_, _ = io.WriteString(stdout, stillFrame())
		}
	}
	return 0
}

// writeGIF encodes grids as an animated GIF at path. If encoding or closing fails it removes the
// partly written file so a truncated GIF is never left behind.
func writeGIF(path string, grids []*frame.Grid, delay int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := gifenc.Write(f, grids, delay); err != nil {
		_ = f.Close()
		_ = os.Remove(path) // no truncated GIF left behind
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

// frameSize resolves the frame size: explicit flags win, then the terminal
// (clamped), then 80x24.
func frameSize(env Env, width, height int, gifMode bool) (w, h int) {
	w, h = 80, 24
	if !gifMode && env.StdoutTTY {
		if cols, rows, err := env.TermSize(); err == nil {
			w, h = min(max(cols, 40), 160), min(max(rows, 12), 60)
		}
	}
	if width != 0 {
		w = width
	}
	if height != 0 {
		h = height
	}
	return w, h
}
