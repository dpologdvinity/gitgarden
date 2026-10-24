package garden

import (
	"slices"
	"testing"

	"github.com/dpologdvinity/gitgarden/internal/gitlog"
)

func cm(name, email string, add, del int) gitlog.Commit {
	return gitlog.Commit{Name: name, Email: email, Added: add, Deleted: del}
}

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

func TestBuildMergesEmailCase(t *testing.T) {
	g := Build("demo", []gitlog.Commit{cm("A", "Ada@X.org", 1, 0), cm("A", "ada@x.org", 1, 0)})
	if len(g.Authors) != 1 || g.Authors[0].Key != "ada@x.org" {
		t.Fatalf("%+v", g.Authors)
	}
	if Key(cm("Zed Q", "", 0, 0)) != "name:zed q" {
		t.Fatal("empty email key")
	}
}

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
