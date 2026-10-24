package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dpologdvinity/gitgarden/internal/testrepo"
)

func fakeEnv() Env {
	return Env{Version: "1.2.3", Stdin: strings.NewReader(""),
		TermSize: func() (int, int, error) { return 0, 0, errors.New("no tty") },
		Sleep:    func(time.Duration) {}, Getenv: func(string) string { return "" }}
}

func run(env Env, args ...string) (string, string, int) {
	var o, e bytes.Buffer
	code := Run(args, &o, &e, env)
	return o.String(), e.String(), code
}

func TestUsageErrors(t *testing.T) {
	cases := []struct {
		args []string
		msg  string
	}{
		{[]string{"extra"}, "gitgarden: unexpected argument: extra\n"},
		{[]string{"--speed", "0"}, "gitgarden: --speed must be greater than 0 and at most 100\n"},
		{[]string{"--speed", "101"}, "gitgarden: --speed must be greater than 0 and at most 100\n"},
		{[]string{"--width", "39"}, "gitgarden: --width must be between 40 and 160\n"},
		{[]string{"--height", "61"}, "gitgarden: --height must be between 12 and 60\n"},
		{[]string{"--still", "--gif", "x.gif"}, "gitgarden: --still and --gif cannot be used together\n"},
		{[]string{"--gif", "x.gif", "--width", "121"}, "gitgarden: --gif supports at most 120x40 cells\n"},
	}
	for _, c := range cases {
		_, stderr, code := run(fakeEnv(), c.args...)
		if code != 2 || !strings.HasPrefix(stderr, c.msg) {
			t.Errorf("%v: code %d stderr %q", c.args, code, stderr)
		}
	}
	if _, stderr, code := run(fakeEnv(), "--bogus"); code != 2 || !strings.Contains(stderr, "usage: gitgarden [flags]") {
		t.Errorf("bogus flag: %d %q", code, stderr)
	}
}

func TestHelpAndVersion(t *testing.T) {
	if out, _, code := run(fakeEnv(), "-h"); code != 0 || out != usage {
		t.Fatalf("help %d %q", code, out)
	}
	if out, _, code := run(fakeEnv(), "--version"); code != 0 || out != "gitgarden 1.2.3\n" {
		t.Fatalf("version %d %q", code, out)
	}
}

func TestRepoErrors(t *testing.T) {
	dir := t.TempDir()
	if _, e, code := run(fakeEnv(), "-C", dir, "--still"); code != 2 || e != "gitgarden: not a git repository: "+dir+"\n" {
		t.Fatalf("%d %q", code, e)
	}
	empty := testrepo.New(t)
	if _, e, code := run(fakeEnv(), "-C", empty.Dir, "--still"); code != 1 || e != "gitgarden: no commits found\n" {
		t.Fatalf("%d %q", code, e)
	}
}
