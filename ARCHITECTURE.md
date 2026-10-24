# Architecture

This document explains how gitgarden is built and why. For what it must do, see
[docs/SPEC.md](docs/SPEC.md), which is the source of truth for behavior.

## Overview

gitgarden reads a repository's history with `git`, turns it into a model of
authors and cumulative statistics, draws that model as a grid of characters,
and shows the grids in one of three ways: a terminal animation, a single
printed frame, or an animated GIF.

```
git log ──▶ gitlog ──▶ garden.Build ──▶ Snapshots ──▶ garden.Draw ──▶ frame.Grid
 (-z numstat)  Commit[]    Garden         Snapshot[]                     │
                                                                         ├─▶ player  (terminal)
                                                                         ├─▶ String/ANSI (still)
                                                                         └─▶ gifenc  (GIF file)
```

Everything between `gitlog` and the output is a pure function of its input.
The program uses no randomness and never reads the clock, so the same
repository and flags always give byte-identical output.

## Packages

All code lives under `internal/` except `main.go`.

| Package | Responsibility | Imports |
|---|---|---|
| `main` | Connects the real terminal, clock and environment to `cli.Run`. Handles SIGINT/SIGTERM so the terminal is restored. | `cli`, `x/term` |
| `internal/cli` | Flag parsing, validation, mode selection, error messages and exit codes. | all below except test helpers |
| `internal/gitlog` | Runs read-only `git` commands and parses the log into `Commit` values. | standard library |
| `internal/garden` | The model (`Build`, `Snapshots`), the layout (slots, plant geometry, seasons) and the drawing (`Draw`). | `frame`, `gitlog`, `gifenc` |
| `internal/frame` | `Grid` of colored cells, the 16-color palette, plain-text and ANSI rendering. | standard library |
| `internal/gifenc` | Frame sampling, rendering a grid to a paletted image, GIF encoding. | `frame`, `x/image` |
| `internal/player` | Plays frames in the terminal's alternate screen and watches for quit keys. | standard library |
| `internal/testrepo` | Builds fixture git repositories with fixed dates (tests only). | standard library |
| `internal/golden` | Compares test output with files in `testdata/golden` (tests only). | standard library |

`frame`, `gitlog` and `player` depend on nothing but the standard library.
Only `main.go` imports `golang.org/x/term`.

## Data flow

### 1. Reading history (`gitlog`)

`Load` runs three `git -C <dir>` commands: `rev-parse --show-toplevel` (the
repository name), `rev-parse --verify HEAD` (are there commits?) and one
`git log --reverse --no-merges --no-renames --numstat -z`. Output uses the
separators `0x1e` (between records) and `0x1f` (between header fields) and NUL
inside the numstat part, so names and subjects containing spaces, tabs or
newlines parse correctly. `0x1e` only starts a record when a full commit hash
and `0x1f` follow, so a stray `0x1e` in a subject does not split a commit.

Each commit's added and deleted lines are summed over its files. Counts above
`HugeCap` (1000) are capped and the commit is marked `Huge`. Merge commits are
skipped because their changes already appear in the commits they merge.

gitgarden only ever runs `rev-parse` and `log`; it never writes to the repository.

### 2. The model (`garden/model.go`)

`Build` groups commits by author identity (lower-case email, or the name when
the email is empty), totals each author, and ranks them: most commits first,
then most lines added, then earliest first commit, then key. The rank is the
author's position everywhere else (slot, color, snapshot index).

`Snapshots(limit)` walks the commits once, keeps running per-author `Stats`,
and copies them at up to `limit` evenly spaced commits (`gifenc.Sample`). Play
mode uses 600 snapshots, GIF mode 300 and still mode 1. A snapshot keeps only
the first `MaxVisibleRanks` ranks, because ranks beyond that can never be
drawn; `MaxRank` records whether any far-away author has started, which is all
the wildflower label needs. This keeps memory flat for repositories with
thousands of authors.

### 3. Layout and drawing (`garden/layout.go`, `garden/draw.go`)

The frame is a `W x H` grid: a title row, the plant area, a soil row, a label
row and a status row. Each author gets a 9-column slot, left to right in rank
order. Layout depends only on the final ranking, so plants never move while
the animation plays. When there are more authors than slots, the last slot
holds the "wildflowers": one `*` per remaining author, labeled `+N`.

A plant's stem height grows with the logarithm of its commit count. The number
of leaves is the square root of the net lines added, and fallen leaves grow
with deleted lines. The season (from the month of the current commit) chooses
leaf glyphs and colors. All geometry is in small pure functions (`StemHeight`,
`LeafCells`, `LeafCount`, `Fallen`, `Slots`) with their own tests.

Every drawn character is printable ASCII, and names and subjects pass through
`Sanitize` (anything else becomes `?`), so one rune is always one cell and the
GIF font can draw every glyph.

### 4. Output

* **Still**: `Grid.String()` (plain) or `Grid.ANSI()` (24-bit color) of the last snapshot.
* **Play**: `player.Play` writes each frame after `ESC[H` inside the alternate
  screen, sleeps in steps of at most 100 ms so quit keys (`q`, Ctrl-C, Esc) are
  noticed quickly, and restores the screen at the end. The CLI then leaves
  raw mode and prints the final frame so it stays visible.
* **GIF**: `gifenc.Image` paints each cell's background and draws its glyph
  with the `basicfont.Face7x13` bitmap font; `gifenc.Write` encodes all frames
  with a 3 second pause on the last one.

## Testability

`cli.Run` takes an `Env` (terminal flag, terminal size, raw-mode function,
sleep function, environment lookup) instead of touching the OS. Tests pass a
fake `Env`, so play mode runs instantly with no terminal. Only `main.go` builds
the real `Env`.

Tests use only the standard library:

* **Unit tests** cover parsing, ranking, sampling, slot layout, plant geometry,
  seasons, rendering and the player (with a fake sleep and pre-filled keys).
* **Golden files** (`testdata/golden/*.txt`) pin full frames. Regenerate them
  per package with `go test ./internal/<pkg> -update` and review the diff.
* **Fixture repositories** (`internal/testrepo`) run real `git` with a fixed
  author, committer, dates and no user configuration, so results are the same
  on every machine.

## Design decisions

* **Per-author plants, not per-file.** The model stays small and the drawing
  fits a terminal; per-directory views are listed as future work in the spec.
* **Fixed layout.** Slots depend on the final ranking, not on the snapshot, so
  the animation is stable and frames can be drawn independently (and lazily).
* **One `git log` call.** Streaming-free parsing of a single `-z` output is
  simple and fast enough for the intended load; the result is held in memory once.
* **No dependencies beyond two `golang.org/x` modules.** `x/term` gives raw
  mode and terminal size, `x/image` gives a self-contained bitmap font.
* **GIF encoding is not streaming.** `image/gif` encodes from a slice, so GIF
  mode is capped at 300 frames and 120x40 cells (about 130 MB worst case).

## Build, CI and release

* CI (`.github/workflows/ci.yml`) runs `gofmt`, `go vet` and `go test -race` on
  Linux, macOS and Windows, plus `govulncheck` and `goreleaser check`.
* Pushing a tag `v*` runs `.github/workflows/release.yml`, which uses
  [GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`) to build
  linux/darwin/windows binaries for amd64 and arm64 and publish a GitHub release.
* Third-party actions are pinned by commit SHA and updated by Dependabot.
