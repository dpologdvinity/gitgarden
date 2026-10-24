package cli

import (
	"bytes"
	"errors"
	"fmt"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dpologdvinity/gitgarden/internal/golden"
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

func threeAuthors(t *testing.T) *testrepo.Repo {
	r := testrepo.New(t)
	for i := 0; i < 12; i++ {
		who := []string{"Ada", "Bob", "Cy"}[i%3]
		r.Write(fmt.Sprintf("f%d.txt", i%4), strings.Repeat(fmt.Sprintf("line %d\n", i), 5+i))
		r.Commit(who, strings.ToLower(who)+"@example.com", fmt.Sprintf("change %d", i))
	}
	return r
}

func TestStillGolden(t *testing.T) {
	r := threeAuthors(t)
	out, _, code := run(fakeEnv(), "-C", r.Dir, "--still", "--width", "60", "--height", "16")
	if code != 0 {
		t.Fatal(code)
	}
	golden.Assert(t, "still_three_authors", strings.ReplaceAll(out, filepath.Base(r.Dir), "REPO"))
}

func TestNonTTYFallsBackToStill(t *testing.T) {
	r := threeAuthors(t)
	a, _, _ := run(fakeEnv(), "-C", r.Dir)
	b, _, _ := run(fakeEnv(), "-C", r.Dir, "--still", "--width", "80", "--height", "24")
	if a != b || strings.Count(a, "\n") != 24 || strings.Contains(a, "\x1b") {
		t.Fatalf("fallback differs or has escapes")
	}
}

func TestGifExport(t *testing.T) {
	r := threeAuthors(t)
	path := filepath.Join(t.TempDir(), "out.gif")
	out, _, code := run(fakeEnv(), "-C", r.Dir, "--gif", path)
	if code != 0 || out != "wrote "+path+" (12 frames, 560x312)\n" {
		t.Fatalf("%d %q", code, out)
	}
	f, _ := os.Open(path)
	defer f.Close()
	a, err := gif.DecodeAll(f)
	if err != nil || len(a.Image) != 12 || a.Delay[0] != 10 || a.Delay[11] != 300 {
		t.Fatalf("decode %v", err)
	}
	if _, e, code := run(fakeEnv(), "-C", r.Dir, "--gif", filepath.Join(path, "nope", "x.gif")); code != 2 || !strings.HasPrefix(e, "gitgarden: cannot write ") {
		t.Fatalf("bad path %d %q", code, e)
	}
}

func TestPlayMode(t *testing.T) {
	r := threeAuthors(t)
	env := fakeEnv()
	env.StdoutTTY = true
	env.Getenv = func(k string) string { return map[string]string{"NO_COLOR": "1"}[k] }
	env.TermSize = func() (int, int, error) { return 60, 16, nil }
	out, _, code := run(env, "-C", r.Dir)
	if code != 0 || !strings.HasPrefix(out, "\x1b[?1049h") || strings.Count(out, "\x1b[H") != 12 {
		t.Fatalf("play output wrong: code %d", code)
	}
	end := strings.LastIndex(out, "\x1b[?1049l") + len("\x1b[?1049l")
	still, _, _ := run(fakeEnv(), "-C", r.Dir, "--still", "--width", "60", "--height", "16")
	if out[end:] != still {
		t.Fatalf("final frame after playback differs from --still:\n%s", out[end:])
	}
	if strings.Contains(out, "\x1b[38;2") {
		t.Fatal("NO_COLOR ignored")
	}
}

func TestTinyTerminalClamps(t *testing.T) {
	r := threeAuthors(t)
	env := fakeEnv()
	env.StdoutTTY = true
	env.Getenv = func(k string) string { return map[string]string{"NO_COLOR": "1"}[k] }
	env.TermSize = func() (int, int, error) { return 20, 5, nil }
	out, _, code := run(env, "-C", r.Dir, "--still")
	if code != 0 || strings.Count(out, "\n") != 12 || !strings.Contains(out, strings.Repeat("=", 40)+"\n") {
		t.Fatalf("not clamped to 40x12: %q", out)
	}
}

type orderWriter struct {
	events *[]string
	last   string
}

func (w *orderWriter) Write(p []byte) (int, error) {
	w.last = string(p)
	*w.events = append(*w.events, "write")
	return len(p), nil
}

func TestPlayRestoresTerminalBeforeFinalFrame(t *testing.T) {
	r := threeAuthors(t)
	var events []string
	env := fakeEnv()
	env.StdoutTTY = true
	env.Getenv = func(k string) string { return map[string]string{"NO_COLOR": "1"}[k] }
	env.TermSize = func() (int, int, error) { return 60, 16, nil }
	env.MakeRaw = func() (func(), error) { return func() { events = append(events, "restore") }, nil }
	w := &orderWriter{events: &events}
	if code := Run([]string{"-C", r.Dir}, w, &bytes.Buffer{}, env); code != 0 {
		t.Fatal(code)
	}
	if len(events) < 2 || events[len(events)-2] != "restore" || events[len(events)-1] != "write" {
		t.Fatalf("final frame must be written right after restore, got tail %v", events[max(0, len(events)-3):])
	}
}
