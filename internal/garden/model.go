// Package garden turns commits into ranked authors, snapshots and drawings.
package garden

import (
	"cmp"
	"slices"
	"strings"

	"github.com/dpologdvinity/gitgarden/internal/gifenc"
	"github.com/dpologdvinity/gitgarden/internal/gitlog"
)

type Author struct {
	Key, Name                      string
	Commits, Added, Deleted, First int
}

type Stats struct{ Commits, Added, Deleted int }

type Garden struct {
	Repo    string
	Commits []gitlog.Commit // oldest first
	Authors []Author        // ranked
	rank    map[string]int  // key -> rank
}

type Snapshot struct {
	Index int     // commit index just applied
	Stats []Stats // per author, indexed by rank
}

// Key is the identity of a commit's author.
func Key(c gitlog.Commit) string {
	if c.Email == "" {
		return "name:" + strings.ToLower(c.Name)
	}
	return strings.ToLower(c.Email)
}

func Build(repo string, commits []gitlog.Commit) *Garden {
	g := &Garden{Repo: repo, Commits: commits, rank: map[string]int{}}
	byKey := map[string]*Author{}
	for i, c := range commits {
		k := Key(c)
		a := byKey[k]
		if a == nil {
			a = &Author{Key: k, First: i}
			byKey[k] = a
		}
		a.Name = c.Name
		a.Commits++
		a.Added += c.Added
		a.Deleted += c.Deleted
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

// Snapshots returns cumulative stats at up to max sampled commits.
func (g *Garden) Snapshots(max int) []Snapshot {
	want := gifenc.Sample(len(g.Commits), max)
	cur := make([]Stats, len(g.Authors))
	var out []Snapshot
	for i, c := range g.Commits {
		s := &cur[g.RankOf(c)]
		s.Commits++
		s.Added += c.Added
		s.Deleted += c.Deleted
		if len(out) < len(want) && want[len(out)] == i {
			out = append(out, Snapshot{Index: i, Stats: slices.Clone(cur)})
		}
	}
	return out
}
