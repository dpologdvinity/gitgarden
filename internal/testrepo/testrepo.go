// Package testrepo builds throwaway git repositories with fixed dates for tests.
package testrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Repo is a fixture repository in a temp dir.
type Repo struct {
	Dir string
	t   *testing.T
	n   int
}

// Date returns 2024-01-15T12:00:00Z plus n months.
func Date(n int) time.Time {
	return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).AddDate(0, n, 0)
}

// New runs git init in a fresh temp dir.
func New(t *testing.T) *Repo {
	t.Helper()
	r := &Repo{Dir: t.TempDir(), t: t}
	r.run("Fixture", "fixture@example.com", "init", "-q", "-b", "main")
	return r
}

func (r *Repo) run(name, email string, args ...string) string {
	r.t.Helper()
	d := Date(r.n).Format(time.RFC3339)
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL="+email,
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
		"GIT_AUTHOR_DATE="+d, "GIT_COMMITTER_DATE="+d)
	out, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// Write creates path (relative to Dir) with content, making parent dirs.
func (r *Repo) Write(path, content string) {
	r.t.Helper()
	p := filepath.Join(r.Dir, path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// Remove deletes path (relative to Dir).
func (r *Repo) Remove(path string) {
	r.t.Helper()
	if err := os.Remove(filepath.Join(r.Dir, path)); err != nil {
		r.t.Fatal(err)
	}
}

// Commit stages everything, commits, and returns the HEAD hash.
func (r *Repo) Commit(name, email, msg string) string {
	r.t.Helper()
	r.run(name, email, "add", "-A")
	r.run(name, email, "commit", "-q", "--allow-empty", "-m", msg)
	h := r.run(name, email, "rev-parse", "HEAD")
	r.n++
	return h[:len(h)-1]
}

// Git runs git with the fixture env and the current date, returning stdout.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	return r.run("Fixture", "fixture@example.com", args...)
}
