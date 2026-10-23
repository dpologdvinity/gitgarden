# gitgarden v0.1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `gitgarden`, a read-only Go CLI that plays a repo's git history as an animated ASCII garden, prints a still final frame, or exports a GIF.

**Architecture:** `gitlog` reads commits through `git log -z`. `garden` turns them into ranked authors, sampled snapshots, and pure `frame.Grid` drawings. `player` (terminal) and `gifenc` (GIF) only consume grids. `cli` wires flags, modes, and an injected `Env` (TTY, size, raw mode, sleep clock) so every path is testable without a terminal.

**Tech Stack:** Go (`go 1.26.0`), stdlib `flag`/`image/gif`/`os/exec`, `golang.org/x/term` v0.47.0, `golang.org/x/image` v0.47.0 (`font/basicfont`), GoReleaser v2, GitHub Actions.

**Spec:** `docs/SPEC.md`

## Global Constraints

- Module path `github.com/dpologdvinity/gitgarden`; go.mod says `go 1.26.0`.
- Dependencies: only `golang.org/x/term v0.47.0` and `golang.org/x/image v0.47.0` (plus what they pull in). Nothing else.
- Read-only: git is only called with `log`, `rev-parse`. Every call is `git -C <dir> ...` with `LC_ALL=C` added to the environment.
- Every drawn glyph is printable ASCII 0x20–0x7E. Names and subjects are sanitized (non-ASCII rune becomes `?`).
- No randomness anywhere. No `time.Now()` and no `time.Sleep` outside `main.go`; the CLI uses `Env.Sleep`.
- Errors go to stderr as `gitgarden: <message>`. Exit codes: 0 ok, 1 no commits, 2 everything else (SPEC §3.4).
- Tests use only stdlib `testing`. Fixture repos use `internal/testrepo` (fixed dates, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`).
- `gofmt -l .` prints nothing and `go vet ./...` is clean before every commit.
- Commits use Conventional Commits. Never commit `CLAUDE.md`, `.claude/`, `.superpowers/`, `.codex/`, `AGENTS.md`, `.mcp.json`, `docs/superpowers/`. Never push.
- LICENSE: MIT, `Copyright (c) 2026 Kaitlyn Bassford`.

## Review Focus

1. Author names with only non-ASCII runes (`李雷`): the label must become `??`, not break the grid. Test: Task 7 `TestDrawSanitizesNames`.
2. Two emails that differ only in case are the same person and must share one plant. Test: Task 5 `TestBuildMergesEmailCase`.
3. `--since` that excludes every commit must give exit 1 `gitgarden: no commits found`, not an empty frame. Test: Task 3 `TestLoadSinceExcludesAll`.
4. A terminal narrower than 40 columns must still render at 40x12 (clamped), not panic. Test: Task 10 `TestTinyTerminalClamps`.
5. A very long commit subject must be cut at W runes and never wrap into the next row. Test: Task 7 `TestDrawTruncatesStatus`.

---

### Task 1: Scaffold, fixture repo helper, golden helper

**Files:**
- Create: `go.mod`, `main.go`, `LICENSE`, `.gitignore`, `.goreleaser.yaml`, `.github/workflows/ci.yml`
- Create: `internal/testrepo/testrepo.go`, `internal/testrepo/testrepo_test.go`
- Create: `internal/golden/golden.go`

**Interfaces:**
- Produces:
  - `testrepo.New(t *testing.T) *testrepo.Repo` (fields: `Dir string`)
  - `(*Repo).Write(path, content string)` — creates parent dirs, path is relative to `Dir`
  - `(*Repo).Remove(path string)`
  - `(*Repo).Commit(name, email, msg string) string` — `git add -A`, `git commit -q --allow-empty -m msg`, returns full HEAD hash; date = `Date(n)` then n++
  - `(*Repo).Git(args ...string) string` — runs git in `Dir` with the fixture env and date `Date(n)` (does not increment); `t.Fatalf` on error; returns stdout
  - `testrepo.Date(n int) time.Time` — `2024-01-15T12:00:00Z` plus `n` months (`AddDate(0, n, 0)`)
  - `golden.Assert(t *testing.T, name, got string)` — compares with `testdata/golden/<name>.txt`; with `-update` writes it

- [ ] **Step 1: Create the scaffold**

`go mod init github.com/dpologdvinity/gitgarden`, then set the `go` line to `go 1.26.0`. `main.go` for now: `package main; var version = "dev"; func main() {}`. `.gitignore`: `.env`, `dist/`, `*.gif`. LICENSE: standard MIT text with `Copyright (c) 2026 Kaitlyn Bassford`.

`.goreleaser.yaml` (exact; validated with goreleaser 2.19):
```yaml
version: 2
project_name: gitgarden
builds:
  - main: .
    binary: gitgarden
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags: ["-s -w -X main.version={{.Version}}"]
archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
checksum:
  name_template: checksums.txt
```

`.github/workflows/ci.yml` (exact):
```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - run: test -z "$(gofmt -l .)"
      - run: go vet ./...
      - run: go test ./...
```

- [ ] **Step 2: Write the failing test** in `internal/testrepo/testrepo_test.go`

```go
func TestCommitUsesFixedDatesAndAuthor(t *testing.T) {
	r := New(t)
	r.Write("a/b.txt", "hi\n")
	h1 := r.Commit("Ada", "ada@example.com", "first")
	r.Commit("Bob", "bob@example.com", "second")
	if len(h1) != 40 {
		t.Fatalf("hash %q", h1)
	}
	got := r.Git("log", "--reverse", "--format=%an|%ae|%aI|%cI|%cn")
	want := "Ada|ada@example.com|2024-01-15T12:00:00Z|2024-01-15T12:00:00Z|Fixture\n" +
		"Bob|bob@example.com|2024-02-15T12:00:00Z|2024-02-15T12:00:00Z|Fixture\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if !Date(12).Equal(time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("Date(12)")
	}
}
```
(Verified with git 2.55: `%aI` prints `Z` for UTC dates passed as `2024-01-15T12:00:00Z`.)

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./internal/testrepo/`
Expected: FAIL, `undefined: New`.

- [ ] **Step 4: Implement `internal/testrepo/testrepo.go` and `internal/golden/golden.go`**

testrepo env for every git call: `os.Environ()` plus `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL` (for `Git`, use `Fixture` / `fixture@example.com`), `GIT_COMMITTER_NAME=Fixture`, `GIT_COMMITTER_EMAIL=fixture@example.com`, `GIT_AUTHOR_DATE` and `GIT_COMMITTER_DATE` = `Date(n).Format(time.RFC3339)`. `New` runs `git init -q -b main`.

golden: `var update = flag.Bool("update", false, "rewrite golden files")`. Path `filepath.Join("testdata", "golden", name+".txt")`. On update: `MkdirAll`, write, return. Otherwise read; on mismatch `t.Errorf` showing both texts.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/testrepo/ && go vet ./... && test -z "$(gofmt -l .)" && goreleaser check`
Expected: PASS; `1 configuration file(s) validated`. (`goreleaser check` needs a git repo; it is one.)

- [ ] **Step 6: Commit**

```bash
git add go.mod main.go LICENSE .gitignore .goreleaser.yaml .github/workflows/ci.yml internal/testrepo internal/golden
git commit -m "chore: scaffold module, CI, release config and test helpers"
```

---

### Task 2: Frame grid and palette

**Files:**
- Create: `internal/frame/frame.go`, `internal/frame/frame_test.go`

**Interfaces:**
- Produces (exact, SPEC §10.1): `type Color uint8` with constants `BG, FG, Dim, Green, DarkGreen, Brown, Yellow, Red, Orange, White, Blue, Pink, Cyan, Bar, Purple, LightGreen`; `var Palette = [16][3]uint8{...}` (SPEC §10.2 RGB); `type Cell struct{ Ch rune; FG, BG Color }`; `type Grid struct{ W, H int; Cells []Cell }`; `New(w, h int) *Grid`; `(*Grid).At(x, y int) Cell`; `Set(x, y int, ch rune, fg Color)`; `SetBG(x, y int, bg Color)`; `Text(x, y int, s string, fg Color)`; `String() string`; `ANSI() string`.

- [ ] **Step 1: Write the failing tests**

```go
func TestStringTrimsAndClips(t *testing.T) {
	g := New(5, 3)
	g.Text(1, 0, "hello world", FG) // clipped at W
	g.Set(4, 2, '*', Red)
	g.Set(9, 9, 'x', Red) // out of bounds: no-op
	if got, want := g.String(), " hell\n\n    *\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if c := g.At(4, 2); c != (Cell{'*', Red, BG}) {
		t.Fatalf("cell %+v", c)
	}
}

func TestANSI(t *testing.T) {
	g := New(2, 1)
	g.Set(0, 0, 'a', FG)
	g.Set(1, 0, 'b', Red)
	g.SetBG(1, 0, Bar)
	want := "\x1b[0m\x1b[38;2;208;208;208ma" +
		"\x1b[0m\x1b[38;2;224;108;117m\x1b[48;2;44;49;58mb\x1b[0m\n"
	if got := g.ANSI(); got != want {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/frame/`
Expected: FAIL, `undefined: New`.

- [ ] **Step 3: Implement `internal/frame/frame.go`** per SPEC §10. `String()` uses `strings.TrimRight(row, " ")`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/frame/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/frame
git commit -m "feat(frame): add cell grid with plain and truecolor rendering"
```

---

### Task 3: Read commits from git

**Files:**
- Create: `internal/gitlog/log.go`, `internal/gitlog/log_test.go`

**Interfaces:**
- Consumes: `testrepo` (Task 1), tests only.
- Produces:
  - `type Commit struct{ Hash, Name, Email string; Time time.Time; Subject string; Added, Deleted int; Huge bool }`
  - `const HugeCap = 1000`
  - `var ErrNoCommits = errors.New("no commits found")`, `var ErrNotRepo = errors.New("not a git repository")`, `var ErrGitMissing = errors.New("git not found in PATH")`
  - `type GitError struct{ Args []string; Stderr string }` with `Error() = "git " + strings.Join(Args, " ") + ": " + <first line of Stderr>`
  - `func Load(dir, since string) (repo string, commits []Commit, err error)`

- [ ] **Step 1: Write the failing tests**

```go
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

func TestLoadSince(t *testing.T) {
	_, cs, err := Load(fixture(t).Dir, "2024-02-01")
	if err != nil || len(cs) != 2 || cs[0].Subject != "two" {
		t.Fatalf("err %v, %d commits", err, len(cs))
	}
}

func TestLoadSinceExcludesAll(t *testing.T) {
	_, _, err := Load(fixture(t).Dir, "2030-01-01")
	if !errors.Is(err, ErrNoCommits) {
		t.Fatalf("got %v", err)
	}
}

func TestLoadEmptyRepoAndNotRepo(t *testing.T) {
	if _, _, err := Load(testrepo.New(t).Dir, ""); !errors.Is(err, ErrNoCommits) {
		t.Fatalf("empty repo: %v", err)
	}
	if _, _, err := Load(t.TempDir(), ""); !errors.Is(err, ErrNotRepo) {
		t.Fatalf("not repo: %v", err)
	}
}

func TestLoadFromSubdirUsesTopLevelName(t *testing.T) {
	r := fixture(t)
	r.Write("sub/x.txt", "x\n")
	r.Commit("Ada", "ada@example.com", "sub")
	repo, _, err := Load(filepath.Join(r.Dir, "sub"), "")
	if err != nil || repo != filepath.Base(r.Dir) {
		t.Fatalf("repo %q err %v", repo, err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/gitlog/`
Expected: FAIL, `undefined: Load`.

- [ ] **Step 3: Implement `Load` in `internal/gitlog/log.go`**

Follow SPEC §4 exactly: `exec.LookPath("git")` failure gives `ErrGitMissing`; `rev-parse --show-toplevel` failure gives `ErrNotRepo`; `rev-parse --verify --quiet HEAD` non-zero gives `ErrNoCommits`; then the `log` command with the exact args from SPEC §4 (append `--since=<v>` only when `since != ""`). Any other failure of the `log` call returns `*GitError`. Parse per SPEC §4.1: `SplitN(header, "\x1f", 5)`, numstat fields `added\tdeleted\tpath` with `-` as 0, sum, then set `Huge` and cap at `HugeCap`. Zero parsed commits gives `ErrNoCommits`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/gitlog/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gitlog
git commit -m "feat(gitlog): load non-merge commits with capped numstat"
```

---

### Task 4: Sampling and GIF encoding

**Files:**
- Create: `internal/gifenc/gifenc.go`, `internal/gifenc/gifenc_test.go`
- Modify: `go.mod`, `go.sum` (`go get golang.org/x/image@v0.47.0`)

**Interfaces:**
- Consumes: `frame.Grid`, `frame.Palette` (Task 2).
- Produces: `const CellW, CellH = 7, 13`; `func Sample(n, max int) []int`; `func Image(g *frame.Grid) *image.Paletted`; `func Write(w io.Writer, frames []*frame.Grid, delay int) error`; `func Delay(speed float64) int` (= `max(2, int(math.Round(10/speed)))`).

- [ ] **Step 1: Write the failing tests**

```go
func TestSample(t *testing.T) {
	cases := []struct {
		n, max int
		want   []int
	}{
		{0, 5, []int{}}, {3, 1, []int{2}}, {3, 5, []int{0, 1, 2}},
		{10, 4, []int{0, 3, 6, 9}}, {7, 3, []int{0, 3, 6}},
	}
	for _, c := range cases {
		if got := Sample(c.n, c.max); !slices.Equal(got, c.want) {
			t.Errorf("Sample(%d,%d)=%v want %v", c.n, c.max, got, c.want)
		}
	}
}

func TestDelay(t *testing.T) {
	if Delay(1) != 10 || Delay(4) != 3 || Delay(5) != 2 || Delay(100) != 2 || Delay(0.5) != 20 {
		t.Fatal("delay")
	}
}

func TestImageColors(t *testing.T) {
	g := frame.New(2, 1)
	g.SetBG(0, 0, frame.Bar)
	g.Set(1, 0, 'A', frame.White)
	img := Image(g)
	if img.Bounds() != image.Rect(0, 0, 14, 13) {
		t.Fatalf("bounds %v", img.Bounds())
	}
	if img.ColorIndexAt(0, 0) != uint8(frame.Bar) {
		t.Fatal("bg of cell 0 must be Bar")
	}
	white := 0
	for y := 0; y < 13; y++ {
		for x := 7; x < 14; x++ {
			if img.ColorIndexAt(x, y) == uint8(frame.White) {
				white++
			}
		}
	}
	if white == 0 {
		t.Fatal("glyph A not drawn")
	}
}

func TestWriteRoundTrip(t *testing.T) {
	gs := []*frame.Grid{frame.New(2, 1), frame.New(2, 1), frame.New(2, 1)}
	var buf bytes.Buffer
	if err := Write(&buf, gs, 5); err != nil {
		t.Fatal(err)
	}
	a, err := gif.DecodeAll(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Image) != 3 || !slices.Equal(a.Delay, []int{5, 5, 300}) || a.LoopCount != 0 || a.Config.Width != 14 {
		t.Fatalf("frames %d delays %v loop %d w %d", len(a.Image), a.Delay, a.LoopCount, a.Config.Width)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/gifenc/`
Expected: FAIL, `undefined: Sample`.

- [ ] **Step 3: Implement** per SPEC §5.3 and §9. `Sample` returns a non-nil empty slice for `n <= 0`. In `Image`, build `color.Palette` from `frame.Palette` (alpha 255), fill each cell rect with `draw.Draw(img, rect, image.NewUniform(pal[bg]), image.Point{}, draw.Src)`, then draw the rune with the `font.Drawer` from SPEC §9 (skip spaces; runes outside 0x20–0x7E become `?`).

- [ ] **Step 4: Run to verify pass**

Run: `go mod tidy && go test ./internal/gifenc/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/gifenc
git commit -m "feat(gifenc): add frame sampling and paletted GIF writer"
```

---

### Task 5: Garden model

**Files:**
- Create: `internal/garden/model.go`, `internal/garden/model_test.go`

**Interfaces:**
- Consumes: `gitlog.Commit` (Task 3), `gifenc.Sample` (Task 4).
- Produces: `type Author struct{ Key, Name string; Commits, Added, Deleted, First int }`; `type Stats struct{ Commits, Added, Deleted int }`; `type Garden struct{ Repo string; Commits []gitlog.Commit; Authors []Author; rank map[string]int }`; `func Build(repo string, commits []gitlog.Commit) *Garden`; `func (g *Garden) RankOf(c gitlog.Commit) int`; `type Snapshot struct{ Index int; Stats []Stats }`; `func (g *Garden) Snapshots(max int) []Snapshot`; `func Key(c gitlog.Commit) string`.

- [ ] **Step 1: Write the failing tests**

```go
func cm(name, email string, add, del int) gitlog.Commit {
	return gitlog.Commit{Name: name, Email: email, Added: add, Deleted: del}
}

func TestBuildRanking(t *testing.T) {
	g := Build("demo", []gitlog.Commit{
		cm("Cy", "cy@x", 5, 0),   // 1 commit, 5 added, first=0
		cm("Al", "al@x", 9, 0),   // 1 commit, 9 added
		cm("Bo", "bo@x", 1, 0),
		cm("Bo2", "bo@x", 1, 0),  // Bo: 2 commits -> rank 0, latest name "Bo2"
		cm("Di", "di@x", 9, 0),   // ties Al on commits+added; Al first=1 < Di first=4
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
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/garden/`
Expected: FAIL, `undefined: Build`.

- [ ] **Step 3: Implement** per SPEC §5.1–§5.2. Sort with `slices.SortStableFunc` using the four keys in SPEC order. Each snapshot holds its own copy of the stats slice (`slices.Clone`).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/garden/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/garden
git commit -m "feat(garden): rank authors and sample cumulative snapshots"
```

---

### Task 6: Layout and plant geometry

**Files:**
- Create: `internal/garden/layout.go`, `internal/garden/layout_test.go`

**Interfaces:**
- Produces: `type Slot struct{ X0, Author int }` (Author = rank, `-1` for wildflowers); `func Slots(w, nAuthors int) (slots []Slot, firstWild int)`; `func StemHeight(commits, h int) int`; `func LeafCells(cx, b, stem int) [][2]int`; `func LeafCount(s Stats, nCandidates int) int`; `func Fallen(s Stats, season Season) int`; `type Season int` with `Winter, Spring, Summer, Autumn`; `func SeasonOf(t time.Time) Season`; `func (s Season) String() string`; `func isqrt(n int) int`.

- [ ] **Step 1: Write the failing tests**

```go
func x0s(s []Slot) []int {
	var out []int
	for _, v := range s {
		out = append(out, v.X0)
	}
	return out
}

func TestSlots(t *testing.T) {
	s, fw := Slots(80, 1)
	if !slices.Equal(x0s(s), []int{36}) || fw != 1 {
		t.Fatalf("1 author: %v %d", x0s(s), fw)
	}
	s, fw = Slots(80, 8)
	if len(s) != 8 || s[0].X0 != 5 || s[7].X0 != 68 || fw != 8 {
		t.Fatalf("8 authors: %v %d", x0s(s), fw)
	}
	s, fw = Slots(80, 9)
	if len(s) != 8 || fw != 7 || s[7].Author != -1 || s[6].Author != 6 {
		t.Fatalf("9 authors: %+v %d", s, fw)
	}
	s, fw = Slots(40, 12)
	if !slices.Equal(x0s(s), []int{3, 12, 21, 30}) || fw != 3 {
		t.Fatalf("w40: %v %d", x0s(s), fw)
	}
	s, fw = Slots(160, 200)
	if len(s) != 17 || s[0].X0 != 4 || fw != 16 {
		t.Fatalf("200 authors: %v %d", x0s(s), fw)
	}
}

func TestStemHeight(t *testing.T) {
	for _, c := range [][3]int{{0, 24, 0}, {1, 24, 2}, {3, 24, 4}, {4, 24, 6}, {1000, 24, 19}, {1000, 12, 7}} {
		if got := StemHeight(c[0], c[1]); got != c[2] {
			t.Errorf("StemHeight(%d,%d)=%d want %d", c[0], c[1], got, c[2])
		}
	}
}

func TestLeafCells(t *testing.T) {
	c := LeafCells(10, 20, 6)
	if len(c) != 18 || c[0] != [2]int{9, 15} || c[1] != [2]int{11, 15} ||
		c[6] != [2]int{8, 17} || c[17] != [2]int{13, 19} {
		t.Fatalf("%v", c)
	}
}

func TestLeafCountAndFallen(t *testing.T) {
	if LeafCount(Stats{5, 300, 50}, 18) != 15 || LeafCount(Stats{5, 10, 50}, 18) != 0 || LeafCount(Stats{5, 10000, 0}, 18) != 18 {
		t.Fatal("LeafCount")
	}
	if Fallen(Stats{1, 0, 0}, Summer) != 0 || Fallen(Stats{1, 0, 64}, Summer) != 2 ||
		Fallen(Stats{1, 0, 64}, Autumn) != 4 || Fallen(Stats{1, 0, 10000}, Autumn) != 6 || Fallen(Stats{1, 0, 0}, Autumn) != 2 {
		t.Fatal("Fallen")
	}
}

func TestSeasonOf(t *testing.T) {
	want := []Season{Winter, Winter, Spring, Spring, Spring, Summer, Summer, Summer, Autumn, Autumn, Autumn, Winter}
	for m := 1; m <= 12; m++ {
		if got := SeasonOf(time.Date(2024, time.Month(m), 1, 0, 0, 0, 0, time.UTC)); got != want[m-1] {
			t.Errorf("month %d: %v", m, got)
		}
	}
	if Autumn.String() != "autumn" {
		t.Fatal("String")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/garden/ -run 'Slots|Stem|Leaf|Season'`
Expected: FAIL, `undefined: Slots`.

- [ ] **Step 3: Implement** per SPEC §6.1, §6.2, §7.1. `isqrt(n)` = `int(math.Sqrt(float64(n)))` for `n >= 0`, 0 for negative.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/garden/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/garden/layout.go internal/garden/layout_test.go
git commit -m "feat(garden): add slot layout, plant geometry and seasons"
```

---

### Task 7: Draw a snapshot

**Files:**
- Create: `internal/garden/draw.go`, `internal/garden/draw_test.go`, `internal/garden/testdata/golden/*.txt`

**Interfaces:**
- Consumes: Tasks 2, 5, 6.
- Produces: `func Draw(g *Garden, s Snapshot, w, h int) *frame.Grid`; `func Sanitize(s string) string`.

- [ ] **Step 1: Write the failing tests**

```go
func dated(c gitlog.Commit, y, m, d int, subj string) gitlog.Commit {
	c.Time = time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	c.Subject = subj
	return c
}

func TestDrawOneAuthorSummer(t *testing.T) {
	g := Build("demo", []gitlog.Commit{
		dated(cm("Ada", "a@x", 10, 0), 2024, 5, 1, "a"),
		dated(cm("Ada", "a@x", 10, 0), 2024, 6, 1, "b"),
		dated(cm("Ada", "a@x", 10, 0), 2024, 7, 15, "add stuff"),
	})
	got := Draw(g, g.Snapshots(1)[0], 60, 12).String()
	want := "gitgarden - demo - 2024-07 summer" + strings.Repeat(" ", 16) + "3/3 commits\n" +
		"\n\n\n\n" +
		strings.Repeat(" ", 28) + "&|&\n" +
		strings.Repeat(" ", 28) + "&|&\n" +
		strings.Repeat(" ", 28) + "&|\n" +
		strings.Repeat(" ", 29) + "|\n" +
		strings.Repeat("=", 60) + "\n" +
		strings.Repeat(" ", 28) + "Ada\n" +
		"2024-07-15 Ada: +10 -0 add stuff\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestDrawSanitizesNames(t *testing.T) {
	g := Build("demo", []gitlog.Commit{dated(cm("李雷", "l@x", 1, 0), 2024, 1, 1, "héllo")})
	out := Draw(g, g.Snapshots(1)[0], 40, 12).String()
	if !strings.Contains(out, "??") || !strings.Contains(out, "h?llo") || strings.ContainsFunc(out, func(r rune) bool { return r > 0x7e }) {
		t.Fatalf("not sanitized:\n%s", out)
	}
}

func TestDrawTruncatesStatus(t *testing.T) {
	g := Build("demo", []gitlog.Commit{dated(cm("A", "a@x", 1, 0), 2024, 1, 1, strings.Repeat("s", 500))})
	lines := strings.Split(Draw(g, g.Snapshots(1)[0], 40, 12).String(), "\n")
	if len(lines[11]) != 40 {
		t.Fatalf("status row len %d", len(lines[11]))
	}
}

func TestDrawGolden(t *testing.T) {
	var cs []gitlog.Commit
	for i := 0; i < 40; i++ { // 12 authors, uneven counts, months cycle
		a := i % 12
		if i >= 24 {
			a = i % 3
		}
		c := cm(fmt.Sprintf("Author%02d", a), fmt.Sprintf("a%02d@x", a), 30+i, i%4*3)
		cs = append(cs, dated(c, 2024, 1+i%12, 10, fmt.Sprintf("commit %d", i)))
	}
	g := Build("demo", cs)
	ss := g.Snapshots(4)
	golden.Assert(t, "w40_wildflowers_winter", Draw(g, ss[1], 40, 16).String()) // index 13, February
	golden.Assert(t, "w80_final", Draw(g, ss[3], 80, 24).String())
}
```
`Sample(40, 4)` is `[0, 13, 26, 39]`, so `ss[1]` is commit 13 (February, winter) and `ss[3]` is the last commit.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/garden/ -run Draw`
Expected: FAIL, `undefined: Draw`.

- [ ] **Step 3: Implement `Draw` and `Sanitize`** in the draw order of SPEC §7.5, with the glyphs and colors of SPEC §6.2–§6.4 and §7.2–§7.4. Current commit = `g.Commits[s.Index]`; its author rank is `g.RankOf(...)`.

- [ ] **Step 4: Generate goldens and inspect them**

Run: `go test ./internal/garden/ -run DrawGolden -update && cat internal/garden/testdata/golden/*.txt`
Expected: in `w40_*`: 3 plants plus a wildflower slot labeled `+9`, 16 lines, soil row all `=`, no line longer than 40. In `w80_final`: 8 slots (7 plants + `+5`), title ends with `40/40 commits`.

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/garden/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/garden
git commit -m "feat(garden): draw plants, wildflowers, seasons and status rows"
```

---

### Task 8: Terminal player

**Files:**
- Create: `internal/player/player.go`, `internal/player/player_test.go`

**Interfaces:**
- Produces: `func Play(w io.Writer, keys <-chan byte, n int, frameAt func(i int) string, interval time.Duration, sleep func(time.Duration)) (quit bool)`

- [ ] **Step 1: Write the failing tests**

```go
func TestPlayAllFrames(t *testing.T) {
	var out bytes.Buffer
	var slept []time.Duration
	frames := []string{"A\nB", "C\nD"}
	quit := Play(&out, make(chan byte, 1), 2, func(i int) string { return frames[i] },
		100*time.Millisecond, func(d time.Duration) { slept = append(slept, d) })
	want := "\x1b[?1049h\x1b[?25l" +
		"\x1b[HA\x1b[K\r\nB\x1b[K" +
		"\x1b[HC\x1b[K\r\nD\x1b[K" +
		"\x1b[?25h\x1b[?1049l"
	if quit || out.String() != want {
		t.Fatalf("quit %v out %q", quit, out.String())
	}
	if !slices.Equal(slept, []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, time.Second}) {
		t.Fatalf("slept %v", slept)
	}
}

func TestPlayQuitKeys(t *testing.T) {
	for _, k := range []byte{'q', 'Q', 3, 27} {
		keys := make(chan byte, 1)
		keys <- k
		var out bytes.Buffer
		slept := 0
		quit := Play(&out, keys, 5, func(int) string { return "x" }, time.Millisecond, func(time.Duration) { slept++ })
		if !quit || strings.Count(out.String(), "\x1b[H") != 1 || slept != 0 || !strings.HasSuffix(out.String(), "\x1b[?1049l") {
			t.Fatalf("key %d: quit %v out %q slept %d", k, quit, out.String(), slept)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/player/`
Expected: FAIL, `undefined: Play`.

- [ ] **Step 3: Implement `Play`** per SPEC §8. Drain keys with a `for { select { case b := <-keys: ...; default: break loop } }`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/player/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/player
git commit -m "feat(player): add alt-screen frame player with quit keys"
```

---

### Task 9: CLI flags, validation, and errors

**Files:**
- Create: `internal/cli/cli.go`, `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `gitlog.Load` and its errors (Task 3).
- Produces: `type Env struct{ Version string; Stdin io.Reader; StdoutTTY bool; TermSize func() (int, int, error); MakeRaw func() (func(), error); Sleep func(time.Duration); Getenv func(string) string }`; `func Run(args []string, stdout, stderr io.Writer, env Env) int`; `const usage = "..."` (SPEC §3.3 text, exact).

- [ ] **Step 1: Write the failing tests**

```go
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
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/`
Expected: FAIL, `undefined: Run`.

- [ ] **Step 3: Implement flag parsing and validation in `Run`**

`flag.NewFlagSet("gitgarden", flag.ContinueOnError)` with output `stderr` and `fs.Usage = func() {}` (so the flag package never prints usage itself). `-h`/`--help` make `Parse` return `flag.ErrHelp`: print `usage` to **stdout** and return 0. Any other parse error: the flag package has already printed its error line to stderr; print `usage` to stderr and return 2. Validation order: positional args, speed, width, height, still+gif, gif size. Then `gitlog.Load`; map errors per SPEC §3.4 (`ErrNotRepo` message uses the `-C` value as given). For this task, after a successful load, return 0 without output; Task 10 fills in the modes.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/cli/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): parse and validate flags with exact errors"
```

---

### Task 10: CLI modes and main wiring

**Files:**
- Modify: `internal/cli/cli.go`, `internal/cli/cli_test.go`, `main.go`, `go.mod`, `go.sum` (`go get golang.org/x/term@v0.47.0`)
- Create: `internal/cli/testdata/golden/*.txt`

**Interfaces:**
- Consumes: `garden.Build`, `(*Garden).Snapshots`, `garden.Draw` (Tasks 5–7), `gifenc.Write`, `gifenc.Delay` (Task 4), `player.Play` (Task 8).
- Produces: the finished `Run`; `main.go` with the real `Env`.

- [ ] **Step 1: Write the failing tests**

```go
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
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/`
Expected: FAIL (still mode prints nothing yet).

- [ ] **Step 3: Implement the modes in `Run`**

Mode selection SPEC §3.1, size SPEC §3.2, color SPEC §10.3 (`env.StdoutTTY && env.Getenv("NO_COLOR") == "" && !noColor`). Still: `Draw(last snapshot)` then `String()` or `ANSI()`. GIF: `Snapshots(300)`, draw each, `gifenc.Write(f, grids, gifenc.Delay(speed))`, create the file with `os.Create`, report write/close errors as `cannot write`. Play: `Snapshots(600)`; if `env.MakeRaw != nil` call it and defer the restore; start a goroutine that reads `env.Stdin` one byte at a time into a `chan byte` (buffer 16, non-blocking send); `player.Play(stdout, keys, len(ss), frameAt, interval, env.Sleep)` where `frameAt` draws lazily; restore the terminal; if not quit, print the final frame as in still mode.

`main.go`: `os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, env))` with `StdoutTTY: term.IsTerminal(int(os.Stdout.Fd()))`, `TermSize: func() (int, int, error) { return term.GetSize(int(os.Stdout.Fd())) }`, `MakeRaw` set only when `term.IsTerminal(int(os.Stdin.Fd()))` (calls `term.MakeRaw`, returns a `term.Restore` closure), `Sleep: time.Sleep`, `Getenv: os.Getenv`, `Version: version`.

- [ ] **Step 4: Generate the golden, inspect, run**

Run: `go test ./internal/cli/ -run StillGolden -update && cat internal/cli/testdata/golden/still_three_authors.txt && go test ./...`
Expected: golden has 16 lines, 3 plants labeled `Ada`, `Bob`, `Cy`, title starts `gitgarden - REPO - 2024-12 winter`. All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum main.go internal/cli
git commit -m "feat(cli): add play, still and gif modes with real terminal env"
```

---

### Task 11: README, release workflow, final verification

**Files:**
- Create: `README.md`, `.github/workflows/release.yml`

**Interfaces:**
- Consumes: everything.

- [ ] **Step 1: Write `.github/workflows/release.yml`** (exact)

```yaml
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@v7
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2: Write `README.md`**: one-paragraph pitch, install (`go install github.com/dpologdvinity/gitgarden@latest` and GitHub releases), usage for each flag from SPEC §3.3, the `--still` example copied from `internal/cli/testdata/golden/still_three_authors.txt`, how merges/huge commits/seasons work (SPEC §4.2, §7.1), MIT license line.

- [ ] **Step 3: Full verification**

Run: `test -z "$(gofmt -l .)" && go vet ./... && go test ./... && goreleaser check && go build -o /tmp/gitgarden . && /tmp/gitgarden -C . --still --no-color && /tmp/gitgarden -C . --gif /tmp/gg.gif`
Expected: all PASS; a garden prints; `wrote /tmp/gg.gif (...)`. Also run `/tmp/gitgarden` in a real terminal on a repo with 50+ authors, watch it play, press `q`, and confirm the terminal is restored. Note the result in the final report.

- [ ] **Step 4: Commit**

```bash
git add README.md .github/workflows/release.yml
git commit -m "docs: add README and release workflow"
```
