# gitgarden — Specification (v0.1)

`gitgarden` renders a git repository's history as an animated ASCII garden.
Every contributor is a plant. Commits make plants grow taller and add leaves,
deleted lines drop leaves, and the month of each commit sets the season. It
plays in the terminal, prints a still final frame, or exports a GIF for a README.

This document is the source of truth for v0.1 behavior. `docs/PLAN.md` says how
to build it. If the plan and the spec disagree, the spec wins.

## 1. Goals and non-goals

### Goals (v0.1)
- `gitgarden`: play the history of the repo in the current directory as a terminal animation. `q` quits.
- `gitgarden --still`: print only the final frame.
- `gitgarden --gif out.gif`: export the animation as an animated GIF.
- Deterministic: the same repo state and the same flags give byte-identical output. gitgarden uses no randomness at all.
- Read-only: only `git` subprocesses that read. Never writes to the repo.
- Works for 1 author and for 200+ authors (long tail becomes "wildflowers").

### Non-goals (v0.1)
- No config file, no themes, no southern-hemisphere seasons (see §13 Later).
- No per-file or per-directory view. Only per-author plants.
- No Unicode art. Every drawn glyph is printable ASCII (0x20–0x7E), so the GIF font can draw it.
- No mouse, no pause/seek during playback.

## 2. Tech stack (fixed)

| Item | Choice | Why |
|---|---|---|
| Language | Go, `go 1.26.0` in go.mod (builds with 1.27) | x/image and x/term v0.47.0 require go 1.26.0 |
| Module path | `github.com/dpologdvinity/gitgarden` | |
| Flags | stdlib `flag` | No positional args, so GNU-style ordering is not needed |
| Terminal | `golang.org/x/term` v0.47.0 | Raw mode (read `q` without Enter) and terminal size. The stdlib has neither. |
| GIF font | `golang.org/x/image` v0.47.0 (`font/basicfont`, `font`, `math/fixed`) | `basicfont.Face7x13` is a self-contained public-domain 7x13 ASCII bitmap font. No font files to ship. |
| GIF | stdlib `image/gif`, `image`, `image/color` | |
| Tests | stdlib `testing` only, golden files | |
| Release | GoReleaser v2 + GitHub Actions | |

Add no other dependencies in v0.1. Versions verified against `proxy.golang.org` on 2026-10-23.

## 3. CLI

```
gitgarden [flags]
```

| Flag | Type | Default | Meaning |
|---|---|---|---|
| `-C` | string | `.` | Repository directory. |
| `--since` | string | "" | Passed to git as `--since=<value>`. Only commits after it are planted; plants start from zero. |
| `--speed` | float | 1 | Playback speed multiplier. Must satisfy `0 < speed <= 100`. |
| `--still` | bool | false | Print only the final frame and exit. |
| `--gif` | string | "" | Write an animated GIF to this path instead of playing. |
| `--width` | int | 0 (auto) | Frame width in cells. If non-zero, must be in [40, 160]. |
| `--height` | int | 0 (auto) | Frame height in cells. If non-zero, must be in [12, 60]. |
| `--no-color` | bool | false | Disable ANSI color. |
| `--version` | bool | false | Print `gitgarden <version>` and exit 0. |
| `-h`, `--help` | bool | false | Print usage (§3.3) and exit 0. |

stdlib `flag` accepts both `-flag` and `--flag`.

### 3.1 Mode selection (first match wins)
1. `--gif <path>` set: GIF mode (§9). `--still` with `--gif` is a usage error.
2. `--still` set: still mode. Print the final frame.
3. stdout is not a terminal: still mode (avoid escape codes in pipes).
4. Otherwise: play mode (§8).

### 3.2 Frame size
- Explicit `--width`/`--height` win in every mode.
- GIF mode, auto: 80 x 24. GIF mode also requires `width <= 120` and `height <= 40` (memory cap, §9).
- Other modes, auto: if stdout is a terminal and its size is readable, `W = clamp(cols, 40, 160)`, `H = clamp(rows, 12, 60)`. Otherwise 80 x 24.

### 3.3 Usage text (exact, printed for `-h` to stdout, and to stderr after usage errors)
```
usage: gitgarden [flags]

Plays a repository's git history as an ASCII garden.

flags:
  -C dir          repository directory (default ".")
  --since date    only plant commits after date (any git date)
  --speed x       playback speed multiplier (default 1)
  --still         print only the final frame
  --gif file      write an animated GIF instead of playing
  --width n       frame width in cells, 40-160 (default: terminal)
  --height n      frame height in cells, 12-60 (default: terminal)
  --no-color      disable color
  --version       print version

keys while playing: q quit
```
Set `flag.FlagSet.Usage` to print this text. Do not use `PrintDefaults`.

### 3.4 Exit codes and messages

| Code | When |
|---|---|
| 0 | Success, including quitting with `q`. |
| 1 | No commits to plant. |
| 2 | Usage error, not a git repository, git missing, git failure, or GIF write failure. |

Errors go to stderr as `gitgarden: <message>`. Exact messages:

| Condition | Message | Code |
|---|---|---|
| unknown flag / bad value | the `flag` package's own error line, then the usage text | 2 |
| positional argument given | `gitgarden: unexpected argument: <arg>` | 2 |
| speed out of range | `gitgarden: --speed must be greater than 0 and at most 100` | 2 |
| width out of range | `gitgarden: --width must be between 40 and 160` | 2 |
| height out of range | `gitgarden: --height must be between 12 and 60` | 2 |
| `--still` with `--gif` | `gitgarden: --still and --gif cannot be used together` | 2 |
| GIF too big | `gitgarden: --gif supports at most 120x40 cells` | 2 |
| `git` not on PATH | `gitgarden: git not found in PATH` | 2 |
| not a repo | `gitgarden: not a git repository: <dir>` | 2 |
| other git failure | `gitgarden: git <args joined by space>: <first line of git stderr>` | 2 |
| no HEAD, or zero commits after filtering | `gitgarden: no commits found` | 1 |
| cannot create/write GIF | `gitgarden: cannot write <path>: <err>` | 2 |

On GIF success print to stdout: `wrote <path> (<frames> frames, <Wpx>x<Hpx>)`.

## 4. Reading git (package `internal/gitlog`)

All git calls run as `git -C <dir> <args>` through `os/exec`, with `LC_ALL=C` added to the environment. Never pass `-c` options that write. Never run commands that change refs, the index, or the worktree.

1. `git -C <dir> rev-parse --show-toplevel`. A failure whose stderr says "not a git repository" means "not a repository"; any other failure (dubious ownership, permissions) is an "other git failure" (§3.4). `Repo` name = `filepath.Base` of the trimmed output.
2. `git -C <dir> rev-parse --verify --quiet HEAD`. Exit status non-zero means no commits: return `ErrNoCommits`.
3. `git -C <dir> log --reverse --no-merges --no-renames --numstat -z --no-color --format=%x1e%H%x1f%aN%x1f%aE%x1f%at%x1f%s [--since=<v>] HEAD`

### 4.1 Parsing the log output
- Split the whole output on `\x1e`. Drop the first element (empty).
- Each chunk: header = text before the first `\x00`. Rest = text after that `\x00`, with one leading `\n` removed if present.
- Header: `strings.SplitN(header, "\x1f", 5)` gives hash, author name (mailmap applied by `%aN`), author email (mailmap applied by `%aE`), unix seconds, subject.
- Rest: split on `\x00`, ignore empty fields. Each field is `<added>\t<deleted>\t<path>`. `-` (binary) counts as 0.
- Zero chunks after filtering: `ErrNoCommits`.

### 4.2 Commit record

```go
type Commit struct {
    Hash    string
    Name    string    // %aN
    Email   string    // %aE, unchanged
    Time    time.Time // time.Unix(at, 0).UTC()
    Subject string
    Added   int       // sum over files, then capped at HugeCap
    Deleted int       // sum over files, then capped at HugeCap
    Huge    bool      // raw added sum > HugeCap or raw deleted sum > HugeCap
}
const HugeCap = 1000
```

Defined behaviors:
- **Merge commits** are excluded by `--no-merges`. Their changes are already counted in the commits they merge, so counting them again would double-grow plants.
- **Huge commits** (vendoring, generated code, mass reformat) count as one commit, but their added and deleted lines are capped at 1000 each, and the status line shows `[huge]`.
- **Binary files** add 0 lines.
- **Empty commits** count as a commit with 0 lines.

## 5. Garden model (package `internal/garden`)

### 5.1 Authors
- Identity key: `strings.ToLower(Email)`. If Email is empty, the key is `"name:" + strings.ToLower(Name)`.
- Display name: the `Name` of that author's latest commit (highest index) in the loaded range.
- Totals per author: `Commits`, `Added`, `Deleted` (sums of the capped values), `First` (index of the first commit).
- Rank order (index 0 = first): `Commits` desc, then `Added` desc, then `First` asc, then key asc.

```go
type Author struct { Key, Name string; Commits, Added, Deleted, First int }
type Stats  struct { Commits, Added, Deleted int }
type Garden struct {
    Repo    string
    Commits []gitlog.Commit // oldest first
    Authors []Author        // ranked
    rank    map[string]int  // key -> rank
}
func Build(repo string, commits []gitlog.Commit) *Garden
```

### 5.2 Snapshots (which states become frames)
A snapshot is the cumulative per-author stats after commit `Index` was applied.

```go
type Snapshot struct {
    Index int     // commit index just applied (0-based)
    Stats []Stats // per author, indexed by rank
}
func (g *Garden) Snapshots(limit int) []Snapshot
```
- `n = len(Commits)`. The sampled indices are `Sample(n, limit)` (§5.3). Walk the commits once in order, add each commit's (1, Added, Deleted) to its author's Stats, and copy the Stats slice whenever the current index is in the sample set.
- Play mode uses `limit = 600`. GIF mode uses `limit = 300`. Still mode uses only the last snapshot (`Snapshots(1)` returns just index `n-1`).

### 5.3 Sampling (shared by gitgarden and replay)
`func Sample(n, limit int) []int` (package `internal/gifenc`, reused by garden):
- `n <= 0`: empty. `limit == 1`: `[n-1]`. `n <= limit`: `[0, 1, ..., n-1]`.
- Otherwise for `k = 0 .. limit-1`: `idx = k*(n-1)/(limit-1)` (integer division). The result is strictly increasing, starts at 0, and ends at `n-1`.

## 6. Layout (package `internal/garden`, file `layout.go`)

The frame is a `W x H` cell grid. Row 0 is the top.

| Rows | Content |
|---|---|
| 0 | Title row (§7.4) |
| 1 .. H-4 | Plant area. The base row is `b = H-4`. |
| H-3 | Soil row: `=` across the full width |
| H-2 | Label row |
| H-1 | Status row (§7.4) |

### 6.1 Slots
- `maxSlots = (W - 2) / 9` (integer division). W=40 gives 4, W=80 gives 8, W=160 gives 17.
- If `len(Authors) <= maxSlots`: every author gets a plant slot, and there is no wildflower slot.
- Else: authors with rank `0 .. maxSlots-2` get plant slots. All other authors are wildflowers, and one extra wildflower slot is added last.
- `nSlots` = plant slots + (1 if there is a wildflower slot).
- Slot `k` (0-based, rank order left to right): `x0 = (W - nSlots*9)/2 + 1 + k*9`. The slot covers columns `x0 .. x0+6`. Its center column is `cx = x0 + 3`.
- Layout depends only on the final author ranking, so plants never move during the animation.

```go
type Slot struct { X0 int; Author int /* rank, or -1 for the wildflower slot */ }
func Slots(w, nAuthors int) (slots []Slot, firstWild int) // firstWild = rank of first wildflower, or nAuthors if none
```

### 6.2 Plant shape (author with stats `s`, center column `cx`, base row `b`)
- If `s.Commits == 0`: draw nothing and no label.
- Stem height: `h = min(2 * bits.Len(uint(s.Commits)), H - 5)`. Commits 1 gives 2, 2-3 gives 4, 4-7 gives 6, 8-15 gives 8, and so on.
- Stem cells: rows `b-h+1 .. b` at column `cx`, char `|`, color Brown. If `h >= 6`, the top cell (row `b-h+1`) is `Y` instead.
- Leaf candidates form a teardrop crown at the top of the stem. Let `crown = max(1, min(h - 2, 8))` rows. For crown row `i = 0 .. crown-1` (row `b-h+1+i`, `i = 0` is the top), in the offset order `[-1, +1, -2, +2, -3, +3]`, keep offsets with `|o| <= min(3, 1 + i/2, crown - i)`. Each candidate is the cell `(cx+o, b-h+1+i)`. The crown widens by one cell every two rows, narrows on its last rows, and the lowest stem cells stay bare trunk (at most 30 candidates).
- Leaf count: `L = min(len(candidates), isqrt(max(0, s.Added - s.Deleted)))`, where `isqrt(n) = floor(sqrt(n))`. The first `L` candidates get leaves. Leaf `j` is the j-th leaf in candidate order.
- Fallen leaves: `F = min(6, isqrt(s.Deleted) / 4)`. In autumn, `F = min(6, F + 2)`. Fallen leaves are drawn on the base row `b` at offsets `[-3, +3, -2, +2, -1, +1]` (first `F`), char `,`, color Orange in autumn and Brown otherwise.

```go
func StemHeight(commits, h int) int
func LeafCells(cx, b, stem int) [][2]int // candidates in order, as (x, y)
func LeafCount(s Stats, nCandidates int) int
func Fallen(s Stats, season Season) int
```

### 6.3 Wildflowers
- Wildflower `j` = the j-th wildflower author in rank order (`j = 0` is rank `firstWild`).
- Positions, in fill order: row `b`, columns `x0 .. x0+6` left to right, then row `b-1`, and so on up to row 1. Capacity = `7 * (H - 4)`.
- Wildflower `j` is drawn at position `j` if `j < capacity` and that author has `Commits >= 1` in the snapshot. Char `*`. Color `WildColors[j % 6]` with `WildColors = [Pink, Yellow, Purple, Blue, Cyan, Red]`.
- Label: `+<N>`, where N = total wildflower authors. Drawn once any wildflower author has a commit.

### 6.4 Labels (row H-2)
- Plant label: the author's display name, sanitized (§7.3), first 7 runes, centered in the slot: start column `x0 + (7 - len)/2`.
- Color: Yellow if the author made the snapshot's current commit, otherwise FG. The wildflower label uses the same rule (Yellow if the current commit's author is a wildflower).

## 7. Drawing (package `internal/garden`, file `draw.go`)

```go
func Draw(g *Garden, s Snapshot, w, h int) *frame.Grid
```

### 7.1 Seasons
Month of the current commit's `Time` (UTC): Dec–Feb `winter`, Mar–May `spring`, Jun–Aug `summer`, Sep–Nov `autumn`. Northern hemisphere only in v0.1.

```go
type Season int
const (Winter Season = iota; Spring; Summer; Autumn)
func SeasonOf(t time.Time) Season
func (s Season) String() string // "winter", "spring", "summer", "autumn"
```

### 7.2 Season palette

| Season | Leaf glyph and color | Soil |
|---|---|---|
| spring | `&` LightGreen, but leaf `j` with `j % 3 == 0` is `*` Pink (blossom) | `=` Brown |
| summer | `&` Green | `=` Brown |
| autumn | `&`, Orange if `j` is even, Red if `j` is odd | `=` Brown |
| winter | `*` White (snow) | `=` White |

### 7.3 Text sanitizing
Every rune outside 0x20–0x7E in names and subjects becomes `?`. This keeps the grid one rune per cell and matches the GIF font.

### 7.4 Title and status rows
- Title (row 0), color FG: `left = "gitgarden - <repo> - <YYYY-MM> <season>"`, `right = "<i>/<n> commits"` with `i = Index+1`. If `len(left) + 1 + len(right) > W`, truncate `left` to `W - len(right) - 1` runes. Put `right` so its last rune is in column `W-1`.
- Status (row H-1), color Dim: `<YYYY-MM-DD> <name>: +<Added> -<Deleted>[ [huge]] <subject>` for the current commit, sanitized, truncated to W runes. `[huge]` appears only when `Huge` is true.

### 7.5 Draw order
1. Title row. 2. Sky: every cell of rows `1 .. H-4` gets background `Sky` (glyphs drawn later keep it; other rows keep the default background). 3. Soil row. 4. For each slot: plant (stem, leaves, fallen) or wildflowers. 5. Labels. 6. Status row.

## 8. Play mode (package `internal/player`)

```go
func Play(w io.Writer, keys <-chan byte, n int, frameAt func(i int) string,
    interval time.Duration, sleep func(time.Duration)) (quit bool)
```
1. Write `"\x1b[?1049h\x1b[?25l"` (alternate screen, hide cursor).
2. For `i = 0 .. n-1`: write `"\x1b[?2026h\x1b[H"` + `frameAt(i)` with every `"\n"` replaced by `"\x1b[K\r\n"`, then `"\x1b[K\x1b[?2026l"` (synchronized output, which stops flicker). Then drain `keys` without blocking: if any byte is `q`, `Q`, `3` (Ctrl-C), or `27` (Esc), go to step 4 with `quit = true`. Otherwise sleep `interval`, in steps of at most 100 ms, draining `keys` after each step so a quit key works at low speeds.
3. After the last frame: `sleep(time.Second)`.
4. Write `"\x1b[?25h\x1b[?1049l"`. Return `quit`.

- `interval = time.Duration(float64(100*time.Millisecond) / speed)`.
- The CLI puts stdin in raw mode before `Play` (if stdin is a terminal), starts a goroutine that copies stdin bytes into `keys`, and restores the terminal after `Play`. If `quit` is false, it then prints the final frame normally, so it stays on screen.
- Frames come from `Snapshots(600)`, drawn lazily in `frameAt`.

## 9. GIF export (package `internal/gifenc`, same design in replay)

```go
const CellW, CellH = 7, 13
func Image(g *frame.Grid) *image.Paletted
func Write(w io.Writer, frames []*frame.Grid, delay int) error
```
- Image size: `W*7 x H*13` pixels. Palette: the 17 colors of §10.2, in index order.
- Each cell: fill its 7x13 rectangle with the cell's BG color, then draw its rune with `font.Drawer{Dst: img, Src: image.NewUniform(Palette[fg]), Face: basicfont.Face7x13, Dot: fixed.P(x*7, y*13+11)}`. Runes outside 0x20–0x7E draw as `?`. Space draws nothing.
- `gif.GIF{Image: images, Delay: delays, LoopCount: 0}` then `gif.EncodeAll`. Every delay is `delay` except the last, which is 300 (3 s).
- `delay = max(2, int(math.Round(10 / speed)))` in hundredths of a second.
- Limits: at most 300 frames (`Snapshots(300)`) and at most 120x40 cells (840x520 px). Worst-case memory is about 300 x 437 KB = 131 MB, because `image/gif` encodes from a slice of all frames. A streaming encoder is in §13 Later.

## 10. Frames (package `internal/frame`, same code in replay)

### 10.1 Grid

```go
type Color uint8
const (BG Color = iota; FG; Dim; Green; DarkGreen; Brown; Yellow; Red;
       Orange; White; Blue; Pink; Cyan; Bar; Purple; LightGreen; Sky)
type Cell struct { Ch rune; FG, BG Color }
type Grid struct { W, H int; Cells []Cell } // row-major, len W*H
func New(w, h int) *Grid                      // every cell {' ', FG, BG}
func (g *Grid) At(x, y int) Cell
func (g *Grid) Set(x, y int, ch rune, fg Color) // out of bounds: no-op; BG unchanged
func (g *Grid) SetBG(x, y int, bg Color)        // out of bounds: no-op
func (g *Grid) Text(x, y int, s string, fg Color) // one rune per cell, clipped at W
func (g *Grid) String() string                  // plain text
func (g *Grid) ANSI() string                    // truecolor
```
- `String()`: each row's runes with trailing spaces removed, and `"\n"` after every row (including the last).
- `ANSI()`: for each row, walk cells. When `(FG, BG)` differs from the previous cell's (or at row start), emit `"\x1b[0m"`, then `"\x1b[38;2;R;G;Bm"` for FG, then `"\x1b[48;2;R;G;Bm"` for BG only if BG != 0. Then the rune. At row end emit `"\x1b[0m\n"`. No trimming.

### 10.2 Palette (exact RGB, index order)

| # | Name | RGB |
|---|---|---|
| 0 | BG | `#101418` |
| 1 | FG | `#d0d0d0` |
| 2 | Dim | `#6c6c6c` |
| 3 | Green | `#5faf5f` |
| 4 | DarkGreen | `#2e7d32` |
| 5 | Brown | `#8d6e63` |
| 6 | Yellow | `#e5c07b` |
| 7 | Red | `#e06c75` |
| 8 | Orange | `#d19a66` |
| 9 | White | `#ffffff` |
| 10 | Blue | `#61afef` |
| 11 | Pink | `#f48fb1` |
| 12 | Cyan | `#56b6c2` |
| 13 | Bar | `#2c313a` |
| 14 | Purple | `#c678dd` |
| 15 | LightGreen | `#98c379` |
| 16 | Sky | `#3d85c6` |

### 10.3 Color in the terminal
Color is on only when stdout is a terminal, `NO_COLOR` is empty, and `--no-color` is not set. Golden tests use `String()`. The GIF always uses colors.

## 11. Package layout

```
gitgarden/
├── main.go                    // builds cli.Env from the real OS, os.Exit(cli.Run(...))
├── internal/
│   ├── cli/cli.go             // flags, mode selection, errors, exit codes
│   ├── cli/cli_test.go
│   ├── gitlog/log.go          // Load, parse
│   ├── garden/model.go        // Build, Snapshots
│   ├── garden/layout.go       // Slots, plant geometry
│   ├── garden/draw.go         // Draw, seasons
│   ├── frame/frame.go         // Grid, palette
│   ├── gifenc/gifenc.go       // Sample, Image, Write
│   ├── player/player.go       // Play
│   ├── testrepo/testrepo.go   // fixture repo builder (tests only)
│   └── golden/golden.go       // golden-file helper with -update (tests only)
├── .goreleaser.yaml
├── .github/workflows/{ci.yml,release.yml}
├── README.md
└── LICENSE                    // MIT, Copyright (c) 2026 Kaitlyn Bassford
```
Imports: `frame` and `gitlog` import only the stdlib. `gifenc` imports `frame` and x/image. `garden` imports `frame`, `gitlog`, `gifenc` (for `Sample`). `player` imports only the stdlib. `cli` imports all of them. Only `main.go` imports `golang.org/x/term`.

### 11.1 CLI injection

```go
type Env struct {
    Version  string
    Stdin    io.Reader
    StdoutTTY bool
    TermSize func() (w, h int, err error)
    MakeRaw  func() (restore func(), err error) // nil when stdin is not a terminal
    Sleep    func(time.Duration)                // the injected clock
    Getenv   func(string) string
}
func Run(args []string, stdout, stderr io.Writer, env Env) int
```

## 12. Testing

- **Fixture repos** (`internal/testrepo`): `New(t)` runs `git init -q -b main` in `t.TempDir()`. Every git call sets `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_COMMITTER_NAME=Fixture`, `GIT_COMMITTER_EMAIL=fixture@example.com`, plus the author name and email for that commit, and `GIT_AUTHOR_DATE` = `GIT_COMMITTER_DATE` = `2024-01-15T12:00:00Z` plus N months, where N is the number of commits made so far through this helper.
- **Golden files**: `internal/golden.Assert(t, name, got)` compares with `testdata/golden/<name>.txt` in the test's package directory. `go test ./internal/<pkg> -update` rewrites them. Run `-update` per package, because the flag exists only in packages that import `golden`.
- Unit tests: parsing (binary `-`, caps, mailmap, empty repo, `--since`), ranking ties, `Sample`, slots for 1 / 8 / 9 / 200 authors at W=40 and W=80, stem heights, leaf candidates, seasons, `String()` and `ANSI()`, GIF decode (frame count, delays, size, a pixel color), and `Play` with a fake sleep and pre-filled keys.
- CLI tests run `cli.Run` with a fake `Env` against fixture repos and compare stdout+stderr+`exit=<code>` with golden files.
- CI: `gofmt -l .` prints nothing, `go vet ./...`, `go test ./...`.

## 13. Definition of done (v0.1)
- Every flag and message in §3 behaves as specified, and the golden tests pass.
- `go test ./...`, `go vet ./...`, and `gofmt -l .` are clean. `goreleaser check` passes.
- Manual run of `gitgarden`, `--still`, and `--gif` on one real repo with 50+ authors.
- README with install (`go install github.com/dpologdvinity/gitgarden@latest`), usage, and a `--still` example copied from a golden file.
- A `v0.1.0` tag builds linux/darwin/windows × amd64/arm64 through GoReleaser.

## 14. Name availability (checked 2026-10-23)
- GitHub `dpologdvinity/gitgarden`: free (404). About 43 unrelated repos use the name.
- Homebrew formula and cask `gitgarden`: free. npm `gitgarden`: free.

## 15. Later (not v0.1)
- Streaming GIF encoder (removes the 300-frame and 120x40 caps).
- `--southern` seasons, themes, per-directory "garden beds".
- Pause, seek, and speed keys during playback. Live resize.
- Plants for co-authors (`Co-authored-by` trailers).
- `--max-commits` for huge histories.
