# Deferred Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the deferred features from SPEC §15: southern-hemisphere seasons (`--southern`), commit limits (`--max-commits`), co-authors (`Co-authored-by` git trailers), interactive playback controls (pause, seek, speed adjustments), and streaming GIF generation.

**Architecture:**
- `garden`: Add `Southern` boolean flag to `Garden` model and support it in `SeasonOf(time.Time, bool)` so seasons invert appropriately.
- `gitlog`: Support `--max-commits` / `-n` flag in `Load(dir, since string, maxCommits int)` and parse `Co-authored-by` git trailers into `Commit.CoAuthors []Author`.
- `player`: Upgrade `Play` loop to support interactive controls: Space (pause/resume), Left/Right/h/l (seek 1 frame), +/-/[/] (adjust playback speed dynamically), keeping non-blocking terminal input.
- `gifenc`: Provide a streaming GIF encoder that encodes and flushes frames sequentially to disk rather than requiring all frames in RAM at once.
- `cli`: Wire up `--southern` and `--max-commits` flags, documentation, and help text.

**Tech Stack:** Go 1.26.0, `golang.org/x/term`, `golang.org/x/image`, `image/gif`.

**Spec:** `docs/SPEC.md` §15 (Deferred Features)

## Global Constraints

- Module path `github.com/dpologdvinity/gitgarden`.
- Dependencies: only `golang.org/x/term` and `golang.org/x/image` plus Go stdlib.
- Read-only: git is only called with read-only commands (`rev-parse`, `log`).
- No randomness anywhere. Byte-identical determinism maintained.
- Errors format as `gitgarden: <message>`. Exit codes: 0 ok, 1 no commits, 2 error.
- All tests use stdlib `testing`.

## Review Focus

1. `--southern` in December returns `Summer`, and in July returns `Winter`.
2. Commit with multiple `Co-authored-by` trailers attributes commits to all distinct authors and handles mailmap correctly.
3. `--max-commits 5` with `--reverse` selects the 5 most recent commits in chronological order.
4. Pausing playback (`Space`) stops frame timer without exiting and resumes cleanly on next `Space`.
5. Streaming GIF encoder writes byte-identical or spec-compliant GIF frames without accumulating all image grids in memory.

---

### Task 1: Southern-Hemisphere Seasons (`--southern`)

**Files:**
- Modify: `internal/garden/model.go`
- Modify: `internal/garden/draw.go`
- Modify: `internal/garden/model_test.go`
- Modify: `internal/garden/draw_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [x] **Step 1: Write failing tests for southern hemisphere seasons**
  In `internal/garden/model_test.go`:
  Test `TestSeasonOfSouthern` verifying Dec/Jan/Feb are Summer, Mar/Apr/May are Autumn, Jun/Jul/Aug are Winter, Sep/Oct/Nov are Spring.
  In `internal/garden/draw_test.go`:
  Test `TestDrawSouthernSeason` verifying title displays Summer in January when `g.Southern = true`.

- [x] **Step 2: Implement southern hemisphere support in `model.go` and `draw.go`**
  Add `Southern bool` to `Garden` struct.
  Update `SeasonOf(t time.Time, southern bool) Season`:
  ```go
  func SeasonOf(t time.Time, southern bool) Season {
      s := northernSeason(t.Month())
      if southern {
          return (s + 2) % 4
      }
      return s
  }
  ```
  Update `Draw` in `internal/garden/draw.go` to use `SeasonOf(cur.Time, g.Southern)`.

- [x] **Step 3: Wire `--southern` flag into `cli.go`**
  Add `--southern` boolean flag in `cli.go`.
  Pass `g.Southern = southern` after `garden.Build`.
  Update help text.

- [x] **Step 4: Verify tests pass**
  Run `go test ./internal/garden ./internal/cli`.

- [x] **Step 5: Commit**
  `git commit -m "feat(garden): add southern-hemisphere season support via --southern"`

---

### Task 2: Commit Limit Flag (`--max-commits`)

**Files:**
- Modify: `internal/gitlog/log.go`
- Modify: `internal/gitlog/log_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [x] **Step 1: Write failing tests for `--max-commits` in `gitlog` and `cli`**
  In `internal/gitlog/log_test.go`:
  Test `TestLoadMaxCommits` on a fixture repo with 5 commits, passing `maxCommits = 2`, expecting the 2 newest commits in chronological order.
  In `internal/cli/cli_test.go`:
  Test `--max-commits 0` and `--max-commits -1` exit 2 with usage error.

- [x] **Step 2: Update `gitlog.Load` to accept `maxCommits int`**
  In `internal/gitlog/log.go`:
  If `maxCommits > 0`, append `fmt.Sprintf("-n%d", maxCommits)` to `args`.
  Note: When combined with `--reverse`, git picks the N newest commits and lists them chronologically.

- [x] **Step 3: Wire `--max-commits` flag in `cli.go`**
  Define `--max-commits` (and shorthand `-n` if appropriate) with integer validation (`> 0`).
  Update help text.

- [x] **Step 4: Verify tests pass**
  Run `go test ./internal/gitlog ./internal/cli`.

- [x] **Step 5: Commit**
  `git commit -m "feat(cli): add --max-commits flag to bound history size"`

---

### Task 3: Support Co-Authors (`Co-authored-by` Git Trailers)

**Files:**
- Modify: `internal/gitlog/log.go`
- Modify: `internal/gitlog/log_test.go`
- Modify: `internal/garden/model.go`
- Modify: `internal/garden/model_test.go`

- [x] **Step 1: Write failing tests for co-authors parsing and ranking**
  In `internal/gitlog/log_test.go`:
  Commit message with `Co-authored-by: Bob <bob@example.com>` trailer; verify `c.CoAuthors` contains Bob.
  In `internal/garden/model_test.go`:
  Verify co-authors get assigned plant slots and contribute to snapshot totals.

- [x] **Step 2: Parse co-author trailers in `gitlog`**
  Add `%(trailers:key=Co-authored-by,valueonly=true)` to git log format separator.
  Define `Author` struct `{ Name, Email string }` in `gitlog`.
  Add `CoAuthors []Author` to `Commit`.
  Parse trailer string into names and emails.

- [x] **Step 3: Update `garden.Build` to credit co-authors**
  In `internal/garden/model.go`:
  Count commits and lines for co-authors in author stats.
  Include co-authors in author ranking and flower beds.

- [x] **Step 4: Verify tests pass**
  Run `go test ./internal/gitlog ./internal/garden`.

- [x] **Step 5: Commit**
  `git commit -m "feat(gitlog): parse Co-authored-by trailers and credit co-authors in garden"`

---

### Task 4: Interactive Terminal Playback Controls

**Files:**
- Modify: `internal/player/player.go`
- Modify: `internal/player/player_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `README.md`

- [x] **Step 1: Write failing tests for interactive controls in `player_test.go`**
  - Space key pauses playback; subsequent key unpauses.
  - Left arrow / 'h' steps frame backward while paused.
  - Right arrow / 'l' steps frame forward while paused.
  - '+' / '-' adjusts frame delay.

- [x] **Step 2: Implement interactive state machine in `player.Play`**
  Support state:
  - `paused bool`
  - `i int` (current frame index, clamped between `0` and `n-1`)
  - `interval time.Duration` (speed clamped between 5ms and 2s)
  Input key handlers:
  - Space (`32`): toggle paused.
  - `h` / Left arrow (`27, 91, 68`): if paused or playing, step `i = max(0, i-1)`.
  - `l` / Right arrow (`27, 91, 67`): if paused, step `i = min(n-1, i+1)`.
  - `+` / `]`: speed up (`interval = max(5*time.Millisecond, interval*3/4)`).
  - `-` / `[`: slow down (`interval = min(2*time.Second, interval*4/3)`).
  - `q` / `Q` / `Ctrl-C` / `Esc`: quit.

- [x] **Step 3: Document interactive controls in `README.md`**
  Document Space (pause), Arrows/h/l (seek), +/- (speed), q (quit).

- [x] **Step 4: Verify tests pass**
  Run `go test -race ./internal/player ./internal/cli`.

- [x] **Step 5: Commit**
  `git commit -m "feat(player): add interactive pause, seek, and speed controls"`

---

### Task 5: Streaming GIF Generation

**Files:**
- Modify: `internal/gifenc/gifenc.go`
- Modify: `internal/gifenc/gifenc_test.go`
- Modify: `internal/cli/cli.go`

- [x] **Step 1: Write failing test for streaming GIF encoder**
  In `internal/gifenc/gifenc_test.go`:
  Add `TestStreamWriter` writing frames one-by-one via `Stream(w, width, height, delay)` interface and verify decoded output matches `Write`.

- [x] **Step 2: Implement streaming GIF encoder in `gifenc.go`**
  Provide `StreamWriter`:
  - `NewStream(w io.Writer, delay int) (*StreamWriter, error)`
  - `(*StreamWriter) Append(g *frame.Grid) error`
  - `(*StreamWriter) Close() error`
  Encodes each frame as `*image.Paletted` and writes GIF headers and sequential data without retaining past frames in heap.

- [x] **Step 3: Update `cli.go` to use streaming encoder for `--gif`**
  Generate snapshots on demand and feed to `StreamWriter.Append` instead of building entire `[]*frame.Grid` array.

- [x] **Step 4: Verify tests pass**
  Run `go test -v ./internal/gifenc ./internal/cli`.

- [x] **Step 5: Commit**
  `git commit -m "feat(gifenc): add streaming GIF encoder to reduce memory overhead"`

---

### Task 6: Documentation, Linting, and Release

**Files:**
- Modify: `README.md`
- Modify: `docs/SPEC.md`
- Modify: `docs/PLAN.md` (now `docs/agents/plans/2026-10-23-v0.1-implementation.md`)

- [x] **Step 1: Update documentation**
  Update `README.md` with new flags: `--southern`, `--max-commits`, interactive keys, and streaming performance note.
  Update `docs/SPEC.md` moving implemented features from §15 into active spec sections.

- [x] **Step 2: Run complete test suite and linters**
  Run:
  `go test -race ./...`
  `go vet ./...`
  `golangci-lint run`
  `goreleaser check`

- [x] **Step 3: Commit and cut release tag**
  Tag `v0.2.0` and push to publish new major feature release.
