// Package garden turns commits into ranked authors, snapshots and drawings.
package garden

import (
	"cmp"
	"slices"
	"strings"

	"github.com/dpologdvinity/gitgarden/internal/gifenc"
	"github.com/dpologdvinity/gitgarden/internal/gitlog"
)

// Author holds the totals for one contributor over the loaded commits.
type Author struct {
	Key     string // identity: lower-case email, or "name:" + lower-case name when the email is empty
	Name    string // display name: the name on the author's latest commit
	Commits int    // number of commits
	Added   int    // total lines added (each commit capped at gitlog.HugeCap)
	Deleted int    // total lines deleted (each commit capped at gitlog.HugeCap)
	First   int    // index of the author's first commit, used to break ranking ties
}

// Stats is the running total for one author at some point in the history.
type Stats struct {
	Commits int // commits applied so far
	Added   int // lines added so far
	Deleted int // lines deleted so far
}

// Garden is the model of a repository: its commits and its ranked authors.
type Garden struct {
	Repo     string
	Commits  []gitlog.Commit // oldest first
	Authors  []Author        // ranked
	Southern bool            // true if seasons use southern hemisphere
	rank     map[string]int  // key -> rank
}

// MaxVisibleRanks bounds how many ranks a snapshot keeps: at most 16 plant
// slots (W=160) plus 7*(60-4) wildflower cells (H=60). Ranks beyond it are never drawn.
const MaxVisibleRanks = 16 + 7*56

// Snapshot is the state of the garden after one commit has been applied.
type Snapshot struct {
	Index   int     // commit index just applied
	Stats   []Stats // per author, indexed by rank, at most MaxVisibleRanks long
	MaxRank int     // highest rank with at least one commit so far
}

// authorKey returns the identity key for a name and email.
func authorKey(name, email string) string {
	if email == "" {
		return "name:" + strings.ToLower(name)
	}
	return strings.ToLower(email)
}

// Key is the identity of a commit's author.
func Key(c gitlog.Commit) string {
	return authorKey(c.Name, c.Email)
}

// Build groups commits by author identity, totals each author and ranks them
// (most commits first; ties by lines added, first appearance, then key).
func Build(repo string, commits []gitlog.Commit) *Garden {
	g := &Garden{Repo: repo, Commits: commits, rank: map[string]int{}}
	byKey := map[string]*Author{}
	for i, c := range commits {
		seen := map[string]bool{}
		add := func(name, email string, added, deleted int, isPrimary bool) {
			k := authorKey(name, email)
			if seen[k] {
				return
			}
			seen[k] = true
			a := byKey[k]
			if a == nil {
				a = &Author{Key: k, First: i}
				byKey[k] = a
			}
			if isPrimary || (name != "" && a.Name == "") {
				a.Name = name
			}
			a.Commits++
			a.Added += added
			a.Deleted += deleted
		}
		add(c.Name, c.Email, c.Added, c.Deleted, true)
		for _, co := range c.CoAuthors {
			add(co.Name, co.Email, c.Added, c.Deleted, false)
		}
	}
	for _, a := range byKey {
		g.Authors = append(g.Authors, *a)
	}
	slices.SortStableFunc(g.Authors, func(a, b Author) int {
		return cmp.Or(cmp.Compare(b.Commits, a.Commits), cmp.Compare(b.Added, a.Added),
			cmp.Compare(a.First, b.First), cmp.Compare(a.Key, b.Key))
	})
	for i, a := range g.Authors {
		g.rank[a.Key] = i
	}
	return g
}

// RankOf returns the rank of the commit's author.
func (g *Garden) RankOf(c gitlog.Commit) int { return g.rank[Key(c)] }

// Snapshots returns cumulative stats at up to limit sampled commits.
func (g *Garden) Snapshots(limit int) []Snapshot {
	want := gifenc.Sample(len(g.Commits), limit)
	cur := make([]Stats, len(g.Authors))
	var out []Snapshot
	maxRank := -1
	for i, c := range g.Commits {
		seen := map[string]bool{}
		apply := func(k string) {
			if seen[k] {
				return
			}
			seen[k] = true
			r := g.rank[k]
			s := &cur[r]
			s.Commits++
			s.Added += c.Added
			s.Deleted += c.Deleted
			maxRank = max(maxRank, r)
		}
		apply(Key(c))
		for _, co := range c.CoAuthors {
			apply(authorKey(co.Name, co.Email))
		}
		if len(out) < len(want) && want[len(out)] == i {
			out = append(out, Snapshot{Index: i, Stats: slices.Clone(cur[:min(len(cur), MaxVisibleRanks)]), MaxRank: maxRank})
		}
	}
	return out
}
