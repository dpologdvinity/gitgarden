package main

import (
	"os"
	"time"

	"golang.org/x/term"

	"github.com/dpologdvinity/gitgarden/internal/cli"
)

var version = "dev"

func main() {
	stdout, stdin := int(os.Stdout.Fd()), int(os.Stdin.Fd())
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
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, env))
}
