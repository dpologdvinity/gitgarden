// Package gitlog reads commit history by running read-only git commands.
package gitlog

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const HugeCap = 1000

var (
	ErrNoCommits  = errors.New("no commits found")
	ErrNotRepo    = errors.New("not a git repository")
	ErrGitMissing = errors.New("git not found in PATH")
)

type Commit struct {
	Hash, Name, Email string
	Time              time.Time
	Subject           string
	Added, Deleted    int
	Huge              bool
}

type GitError struct {
	Args   []string
	Stderr string
}

func (e *GitError) Error() string {
	first, _, _ := strings.Cut(strings.TrimSpace(e.Stderr), "\n")
	return "git " + strings.Join(e.Args, " ") + ": " + first
}

func git(dir string, args ...string) (string, error) {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stdout strings.Builder // String() does not copy, unlike bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", &GitError{Args: args, Stderr: stderr.String()}
	}
	return stdout.String(), nil
}

// Load returns the repo name and its non-merge commits, oldest first.
func Load(dir, since string) (repo string, commits []Commit, err error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", nil, ErrGitMissing
	}
	top, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", nil, classifyRevParse(err)
	}
	repo = filepath.Base(strings.TrimSpace(top))
	if _, err := git(dir, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return repo, nil, ErrNoCommits
	}
	args := []string{"log", "--reverse", "--no-merges", "--no-renames", "--numstat", "-z",
		"--no-color", "--format=%x1e%H%x1f%aN%x1f%aE%x1f%at%x1f%s"}
	if since != "" {
		args = append(args, "--since="+since)
	}
	args = append(args, "HEAD")
	out, err := git(dir, args...)
	if err != nil {
		return repo, nil, err
	}
	for _, chunk := range splitRecords(out) {
		c, err := parseChunk(chunk)
		if err != nil {
			return repo, nil, err
		}
		commits = append(commits, c)
	}
	if len(commits) == 0 {
		return repo, nil, ErrNoCommits
	}
	return repo, commits, nil
}

func parseChunk(chunk string) (Commit, error) {
	header, rest, _ := strings.Cut(chunk, "\x00")
	rest = strings.TrimPrefix(rest, "\n")
	f := strings.SplitN(header, "\x1f", 5)
	if len(f) != 5 {
		return Commit{}, &GitError{Args: []string{"log"}, Stderr: "unparsable output"}
	}
	at, err := strconv.ParseInt(f[3], 10, 64)
	if err != nil {
		return Commit{}, &GitError{Args: []string{"log"}, Stderr: "bad timestamp " + f[3]}
	}
	c := Commit{Hash: f[0], Name: f[1], Email: f[2], Time: time.Unix(at, 0).UTC(), Subject: f[4]}
	for _, field := range strings.Split(rest, "\x00") {
		if field == "" {
			continue
		}
		p := strings.SplitN(field, "\t", 3)
		if len(p) != 3 {
			continue
		}
		a, _ := strconv.Atoi(p[0]) // "-" (binary) parses as 0
		d, _ := strconv.Atoi(p[1])
		c.Added += a
		c.Deleted += d
	}
	if c.Added > HugeCap || c.Deleted > HugeCap {
		c.Huge = true
	}
	c.Added = min(c.Added, HugeCap)
	c.Deleted = min(c.Deleted, HugeCap)
	return c, nil
}

// classifyRevParse maps git's "not a repository" failure to ErrNotRepo and
// keeps every other failure (dubious ownership, permissions) visible.
func classifyRevParse(err error) error {
	var ge *GitError
	if errors.As(err, &ge) && !strings.Contains(ge.Stderr, "not a git repository") {
		return err
	}
	return ErrNotRepo
}

// splitRecords splits log output on \x1e, but only where a commit hash and
// \x1f follow, so a \x1e inside a subject does not cut a commit in two.
func splitRecords(out string) []string {
	var recs []string
	start := -1
	for i := 0; i < len(out); i++ {
		if out[i] != 0x1e || !hashFollows(out[i+1:]) {
			continue
		}
		if start >= 0 {
			recs = append(recs, out[start:i])
		}
		start = i + 1
	}
	if start >= 0 {
		recs = append(recs, out[start:])
	}
	return recs
}

func hashFollows(s string) bool {
	n := 0
	for n < len(s) && (s[n] >= '0' && s[n] <= '9' || s[n] >= 'a' && s[n] <= 'f') {
		n++
	}
	return (n == 40 || n == 64) && n < len(s) && s[n] == 0x1f
}
