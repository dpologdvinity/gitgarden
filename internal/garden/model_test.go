package garden

import (
	"fmt"
	"slices"
	"testing"

	"github.com/dpologdvinity/gitgarden/internal/gitlog"
)

// cm returns a commit by the given author with the given line counts.
func cm(name, email string, add, del int) gitlog.Commit {
	return gitlog.Commit{Name: name, Email: email, Added: add, Deleted: del}
}

// TestBuildRanking checks the order of authors, including every tie-break, and that the latest name wins.
func TestBuildRanking(t *testing.T) {
	g := Build("demo", []gitlog.Commit{
		cm("Cy", "cy@x", 5, 0), // 1 commit, 5 added, first=0
		cm("Al", "al@x", 9, 0), // 1 commit, 9 added
		cm("Bo", "bo@x", 1, 0),
		cm("Bo2", "bo@x", 1, 0), // Bo: 2 commits -> rank 0, latest name "Bo2"
		cm("Di", "di@x", 9, 0),  // ties Al on commits+added; Al first=1 < Di first=4
	})
	var keys []string
	for _, a := range g.Authors {
		keys = append(keys, a.Key)
	}
	if !slices.Equal(keys, []string{"bo@x", "al@x", "di@x", "cy@x"}) {
		t.Fatalf("rank %v", keys)
	}
	if g.Authors[0].Name != "Bo2" || g.Authors[0].Commits != 2 || g.Authors[0].Added != 2 {
		t.Fatalf("bo %+v", g.Authors[0])
	}
}

// TestBuildMergesEmailCase checks that emails differing only in case are one author, and the key used when the email is empty.
func TestBuildMergesEmailCase(t *testing.T) {
	g := Build("demo", []gitlog.Commit{cm("A", "Ada@X.org", 1, 0), cm("A", "ada@x.org", 1, 0)})
	if len(g.Authors) != 1 || g.Authors[0].Key != "ada@x.org" {
		t.Fatalf("%+v", g.Authors)
	}
	if Key(cm("Zed Q", "", 0, 0)) != "name:zed q" {
		t.Fatal("empty email key")
	}
}

// TestBuildCreditsCoAuthors checks that co-authors get author entries and snapshots credit them.
func TestBuildCreditsCoAuthors(t *testing.T) {
	c := cm("Ada", "ada@example.com", 10, 2)
	c.CoAuthors = []gitlog.CoAuthor{{Name: "Bob", Email: "bob@example.com"}}
	g := Build("demo", []gitlog.Commit{c})
	if len(g.Authors) != 2 {
		t.Fatalf("want 2 authors, got %d: %+v", len(g.Authors), g.Authors)
	}
	snaps := g.Snapshots(1)
	if len(snaps) != 1 {
		t.Fatalf("want 1 snapshot, got %d", len(snaps))
	}
	s := snaps[0]
	if len(s.Stats) != 2 || s.Stats[0].Commits != 1 || s.Stats[1].Commits != 1 {
		t.Fatalf("stats mismatch: %+v", s.Stats)
	}
}

// TestSnapshots checks which commits are sampled and that stats are cumulative and copied per snapshot.
func TestSnapshots(t *testing.T) {
	var cs []gitlog.Commit
	for i := 0; i < 10; i++ {
		cs = append(cs, cm("A", "a@x", 2, 1))
	}
	g := Build("demo", cs)
	ss := g.Snapshots(4) // indices 0,3,6,9
	if len(ss) != 4 || ss[1].Index != 3 || ss[1].Stats[0] != (Stats{4, 8, 4}) {
		t.Fatalf("%+v", ss)
	}
	if last := g.Snapshots(1); len(last) != 1 || last[0].Stats[0] != (Stats{10, 20, 10}) {
		t.Fatalf("%+v", last)
	}
}

// TestSnapshotStatsAreBounded checks that a snapshot never keeps more than MaxVisibleRanks ranks, however many authors there are.
func TestSnapshotStatsAreBounded(t *testing.T) {
	var cs []gitlog.Commit
	for i := 0; i < 1000; i++ { // 1000 authors; author 0 has the most commits
		for k := 0; k <= (1000-i)/500; k++ {
			cs = append(cs, cm(fmt.Sprintf("A%d", i), fmt.Sprintf("a%d@x", i), 1, 0))
		}
	}
	g := Build("demo", cs)
	for _, s := range g.Snapshots(5) {
		if len(s.Stats) > MaxVisibleRanks {
			t.Fatalf("snapshot keeps %d ranks, cap %d", len(s.Stats), MaxVisibleRanks)
		}
	}
}

// TestSnapshotTracksFarWildflower checks that MaxRank still records an author whose rank is beyond the kept stats.
func TestSnapshotTracksFarWildflower(t *testing.T) {
	cs := []gitlog.Commit{cm("Last", "last@x", 1, 0)} // rank 1000 after the others below
	for i := 0; i < 1000; i++ {
		cs = append(cs, cm(fmt.Sprintf("A%d", i), fmt.Sprintf("a%d@x", i), 5, 0), cm(fmt.Sprintf("A%d", i), fmt.Sprintf("a%d@x", i), 5, 0))
	}
	g := Build("demo", cs)
	s := g.Snapshots(0 + 1000)[0] // index 0: only the lowest-ranked author has a commit
	if s.Index != 0 || s.MaxRank != 1000 {
		t.Fatalf("index %d maxRank %d", s.Index, s.MaxRank)
	}
}
