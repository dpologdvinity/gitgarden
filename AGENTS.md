# AGENTS.md

Instructions for AI coding agents (Claude Code, Codex, ...) working in this repo.
`CLAUDE.md` only imports this file, so this is the single source of truth.

## What this is

`gitgarden` is a read-only Go CLI that plays a repo's git history as an animated
ASCII garden, prints the final frame (`--still`), or exports a GIF (`--gif`).
Behavior is specified in [docs/SPEC.md](docs/SPEC.md); code layout is in
[ARCHITECTURE.md](ARCHITECTURE.md).

## Commands

Needs Go (version in `go.mod`) and `git`. All of these must pass before a commit:

```sh
go build ./...
test -z "$(gofmt -l .)"
go vet ./...
go test -race ./...
golangci-lint run ./...          # v2, config in .golangci.yml
go run . --still                 # smoke test on this repo
```

- Golden files: `go test ./internal/<pkg> -update`, then read the diff.
- Release config: `goreleaser check`.
- Git hooks: run `lefthook install` once (gitleaks, gofmt, golangci-lint, typos).

No environment variables are needed for local work. CI and release use the GitHub
secrets `CODECOV_TOKEN`, `HOMEBREW_TAP_TOKEN` and `APT_GPG_PRIVATE_KEY` (see ARCHITECTURE.md).

## Rules that are easy to break

- Read-only: only call `git` with `rev-parse`, `log` and `check-mailmap`, always as `git -C <dir>`.
- Deterministic: no randomness, no `time.Now()` or real sleeps outside `main.go`; use `cli.Env`.
- Every drawn glyph is printable ASCII (0x20-0x7E).
- Dependencies: only `golang.org/x/term` and `golang.org/x/image`. Don't add more.
- Errors go to stderr as `gitgarden: <message>`. Exit codes: 0 ok, 1 no commits, 2 anything else.
- Tests use stdlib `testing` only and build fixture repos with `internal/testrepo`,
  never the repo they run in. CI enforces 80% statement coverage.
- Every package, exported identifier and constant has a doc comment (revive enforces it).
- If behavior changes, update `docs/SPEC.md`, the README and tests in the same change.

## Commits

Conventional Commits (`feat(garden): ...`, `fix: ...`, `docs: ...`, `ci: ...`), one
logical change each. The default branch is `master`.

## Agent workflow

Features are built spec-first with the [superpowers](https://github.com/obra/superpowers) skills:

1. Brainstorm the design, then write or update the spec (`docs/SPEC.md`).
2. Write a step-by-step plan with checkboxes in `docs/agents/plans/YYYY-MM-DD-<topic>.md`.
3. Execute it with subagent-driven development (one subagent per task, review between tasks).

The plans in `docs/agents/plans/` are completed, historical records: their constraints
describe the repo at the time they were written, not now.

Project plugins are in `.claude/settings.json` (gopls LSP, commit commands,
[ponytail](https://github.com/DietrichGebert/ponytail)); Codex enables ponytail in
`.codex/config.toml`. Personal overrides go in `.claude/settings.local.json` or
`CLAUDE.local.md`, both gitignored.
