// Command gitgarden plays a git repository's history as an animated ASCII garden.
//
// This file only connects the real terminal, clock and environment to the cli
// package; see ARCHITECTURE.md for how the rest fits together.
package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/dpologdvinity/gitgarden/internal/cli"
)

// version is the program version. GoReleaser overwrites it at build time with -X main.version.
var version = "dev"

// main wires the real terminal, clock and environment into cli.Run and exits with its code.
// It contains no logic of its own so that everything else can be tested without a terminal.
func main() {
	stdout, stdin := int(os.Stdout.Fd()), int(os.Stdin.Fd())
	// MakeRaw stays nil unless stdin is a terminal, so piped input never changes terminal modes.
	env := cli.Env{
		Version:   version,
		Stdin:     os.Stdin,
		StdoutTTY: term.IsTerminal(stdout),
		TermSize:  func() (int, int, error) { return term.GetSize(stdout) },
		Sleep:     time.Sleep,
		Getenv:    os.Getenv,
	}
	if term.IsTerminal(stdin) {
		env.MakeRaw = func() (func(), error) {
			old, err := term.MakeRaw(stdin)
			if err != nil {
				return nil, err
			}
			return func() { term.Restore(stdin, old) }, nil
		}
	}
	exitCleanlyOnSignal(env.StdoutTTY, stdin)
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, env))
}

// exitCleanlyOnSignal leaves the alternate screen, shows the cursor and
// restores the terminal state when SIGINT or SIGTERM arrives mid-playback.
func exitCleanlyOnSignal(stdoutTTY bool, stdin int) {
	var saved *term.State
	if term.IsTerminal(stdin) {
		saved, _ = term.GetState(stdin)
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-ch
		if saved != nil {
			term.Restore(stdin, saved)
		}
		if stdoutTTY {
			os.Stdout.WriteString("\x1b[?25h\x1b[?1049l")
		}
		code := 130
		if sig == syscall.SIGTERM {
			code = 143
		}
		os.Exit(code)
	}()
}
