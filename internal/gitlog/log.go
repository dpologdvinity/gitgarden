// Package gitlog reads commit history by running read-only git commands.
package gitlog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// HugeCap is the largest added or deleted line count kept for one commit. A commit above it is marked Huge.
const HugeCap = 1000

// Errors returned by Load. Callers compare them with errors.Is.
var (
	ErrNoCommits  = errors.New("no commits found")
	ErrNotRepo    = errors.New("not a git repository")
	ErrGitMissing = errors.New("git not found in PATH")
)

// Commit is one non-merge commit with its line counts. Added and Deleted are capped at HugeCap;
// Huge records whether either raw total was above the cap.
type Commit struct {
	Hash    string
	Name    string    // author name, after .mailmap
	Email   string    // author email, after .mailmap, case unchanged
	Time    time.Time // author time in UTC
	Subject string
	Added   int  // lines added, capped at HugeCap
	Deleted int  // lines deleted, capped at HugeCap
	Huge    bool // true if the raw added or deleted total was above HugeCap
}

// GitError is a git command that failed. It carries the arguments and git's stderr.
type GitError struct {
	Args   []string // git arguments, without the leading -C <dir>
	Stderr string   // everything git wrote to stderr
}

// Error formats the failure as "git <args>: <first line of stderr>".
func (e *GitError) Error() string {
	first, _, _ := strings.Cut(strings.TrimSpace(e.Stderr), "\n")
	if first == "" {
		return "git " + strings.Join(e.Args, " ") + ": command failed"
	}
	return "git " + strings.Join(e.Args, " ") + ": " + first
}

// git runs `git -C dir args...` with LC_ALL=C and returns its stdout. A non-zero exit becomes a *GitError.
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
// If maxCommits is provided and > 0, at most that many of the latest commits are loaded.
func Load(dir, since string, maxCommits ...int) (repo string, commits []Commit, err error) {
	// 1. git must exist; then find the repository root to get its name.
	if _, err := exec.LookPath("git"); err != nil {
		return "", nil, ErrGitMissing
	}
	top, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", nil, classifyRevParse(err)
	}
	repo = filepath.Base(strings.TrimSpace(top))
	// 2. A repository without HEAD has no commits yet.
	if _, err := git(dir, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return repo, nil, ErrNoCommits
	}
	// 3. One git log call, oldest first, merges skipped. -z and the 0x1e/0x1f separators keep
	// parsing safe for names and subjects that contain spaces, tabs or newlines.
	args := []string{"log", "--reverse", "--no-merges", "--no-renames", "--numstat", "-z",
		"--no-color", "--format=%x1e%H%x1f%aN%x1f%aE%x1f%at%x1f%s"}
	if since != "" {
		args = append(args, "--since="+since)
	}
	if len(maxCommits) > 0 && maxCommits[0] > 0 {
		args = append(args, fmt.Sprintf("-n%d", maxCommits[0]))
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

// parseChunk parses one log record: a header (hash, name, email, unix time, subject separated by
// 0x1f), a NUL, then NUL-separated numstat entries "added\tdeleted\tpath". Binary files count as 0 lines.
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

// hashFollows reports whether s starts with a full commit hash (40 or 64 hex digits) and then 0x1f.
func hashFollows(s string) bool {
	n := 0
	for n < len(s) && (s[n] >= '0' && s[n] <= '9' || s[n] >= 'a' && s[n] <= 'f') {
		n++
	}
	return (n == 40 || n == 64) && n < len(s) && s[n] == 0x1f
}
