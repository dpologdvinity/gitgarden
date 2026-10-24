package gitlog

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dpologdvinity/gitgarden/internal/testrepo"
)

// fixture builds a three-commit repository with a mixed-case email, a binary file and a huge file.
func fixture(t *testing.T) *testrepo.Repo {
	r := testrepo.New(t)
	r.Write("a.txt", "1\n2\n3\n")
	r.Commit("Ada", "ada@example.com", "one") // 2024-01-15
	r.Write("a.txt", "1\n2\nthree\n")
	r.Commit("Ada L", "ADA@example.com", "two") // 2024-02-15
	r.Write("bin.dat", "\x00\x01\x02")
	r.Write("big.txt", strings.Repeat("x\n", 1500))
	r.Commit("Bob", "bob@example.com", "three\x1fwith sep") // 2024-03-15
	return r
}

// TestLoadParsesNumstat checks line counts, the huge cap, binary files, the unit separator in a subject, and UTC times.
func TestLoadParsesNumstat(t *testing.T) {
	r := fixture(t)
	repo, cs, err := Load(r.Dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if repo != filepath.Base(r.Dir) || len(cs) != 3 {
		t.Fatalf("repo %q, %d commits", repo, len(cs))
	}
	if cs[0].Added != 3 || cs[0].Deleted != 0 || cs[0].Huge {
		t.Fatalf("c0 %+v", cs[0])
	}
	if cs[1].Added != 1 || cs[1].Deleted != 1 || cs[1].Name != "Ada L" || cs[1].Email != "ADA@example.com" {
		t.Fatalf("c1 %+v", cs[1])
	}
	if cs[2].Added != 1000 || !cs[2].Huge || cs[2].Subject != "three\x1fwith sep" {
		t.Fatalf("c2 %+v", cs[2])
	}
	if !cs[2].Time.Equal(testrepo.Date(2)) || cs[2].Time.Location() != time.UTC {
		t.Fatalf("time %v", cs[2].Time)
	}
}

// TestLoadAppliesMailmapAndSkipsMerges checks that .mailmap is applied and merge commits are left out.
func TestLoadAppliesMailmapAndSkipsMerges(t *testing.T) {
	r := testrepo.New(t)
	r.Write(".mailmap", "Bob Builder <bob@example.com> <bobby@old.example>\n")
	r.Commit("Ada", "ada@example.com", "base")
	r.Git("checkout", "-q", "-b", "side")
	r.Write("s.txt", "s\n")
	r.Commit("bobby", "bobby@old.example", "side work")
	r.Git("checkout", "-q", "main")
	r.Write("m.txt", "m\n")
	r.Commit("Ada", "ada@example.com", "main work")
	r.Git("merge", "--no-ff", "-q", "-m", "merge side", "side")
	_, cs, err := Load(r.Dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 3 {
		t.Fatalf("want 3 non-merge commits, got %d", len(cs))
	}
	var bob *Commit
	for i := range cs {
		if cs[i].Subject == "side work" {
			bob = &cs[i]
		}
	}
	if bob == nil || bob.Name != "Bob Builder" || bob.Email != "bob@example.com" {
		t.Fatalf("mailmap not applied: %+v", bob)
	}
}

// TestLoadSince checks that --since drops the older commits.
func TestLoadSince(t *testing.T) {
	_, cs, err := Load(fixture(t).Dir, "2024-02-01")
	if err != nil || len(cs) != 2 || cs[0].Subject != "two" {
		t.Fatalf("err %v, %d commits", err, len(cs))
	}
}

// TestLoadSinceExcludesAll checks that a --since date after every commit gives ErrNoCommits.
func TestLoadSinceExcludesAll(t *testing.T) {
	_, _, err := Load(fixture(t).Dir, "2030-01-01")
	if !errors.Is(err, ErrNoCommits) {
		t.Fatalf("got %v", err)
	}
}

// TestLoadEmptyRepoAndNotRepo checks ErrNoCommits for a repository without commits and ErrNotRepo for a plain directory.
func TestLoadEmptyRepoAndNotRepo(t *testing.T) {
	if _, _, err := Load(testrepo.New(t).Dir, ""); !errors.Is(err, ErrNoCommits) {
		t.Fatalf("empty repo: %v", err)
	}
	if _, _, err := Load(t.TempDir(), ""); !errors.Is(err, ErrNotRepo) {
		t.Fatalf("not repo: %v", err)
	}
}

// TestLoadFromSubdirUsesTopLevelName checks that the repository name comes from the top-level directory, not the directory passed in.
func TestLoadFromSubdirUsesTopLevelName(t *testing.T) {
	r := fixture(t)
	r.Write("sub/x.txt", "x\n")
	r.Commit("Ada", "ada@example.com", "sub")
	repo, _, err := Load(filepath.Join(r.Dir, "sub"), "")
	if err != nil || repo != filepath.Base(r.Dir) {
		t.Fatalf("repo %q err %v", repo, err)
	}
}

// TestLoadSubjectWithRecordSeparator checks that a 0x1e byte inside a subject does not split a commit in two.
func TestLoadSubjectWithRecordSeparator(t *testing.T) {
	r := testrepo.New(t)
	r.Write("a.txt", "x\n")
	r.Commit("Ada", "ada@example.com", "before\x1eafter")
	r.Write("a.txt", "y\n")
	r.Commit("Ada", "ada@example.com", "second")
	_, cs, err := Load(r.Dir, "")
	if err != nil || len(cs) != 2 || cs[0].Subject != "before\x1eafter" || cs[1].Subject != "second" {
		t.Fatalf("err %v commits %+v", err, cs)
	}
}

// TestLoadDubiousOwnershipIsNotReportedAsNotRepo checks that only git's "not a git repository" error maps to ErrNotRepo.
func TestLoadDubiousOwnershipIsNotReportedAsNotRepo(t *testing.T) {
	err := classifyRevParse(&GitError{Args: []string{"rev-parse"}, Stderr: "fatal: detected dubious ownership in repository at '/x'\n"})
	var ge *GitError
	if errors.Is(err, ErrNotRepo) || !errors.As(err, &ge) {
		t.Fatalf("got %v", err)
	}
	if !errors.Is(classifyRevParse(&GitError{Stderr: "fatal: not a git repository (or any of the parent directories): .git\n"}), ErrNotRepo) {
		t.Fatal("real non-repo must map to ErrNotRepo")
	}
}
