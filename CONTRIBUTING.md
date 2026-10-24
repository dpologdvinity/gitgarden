# Contributing to gitgarden

Thanks for helping! This guide covers the local workflow and what a pull request needs.
For how the code is organized, read [ARCHITECTURE.md](ARCHITECTURE.md); the exact behavior is in [docs/SPEC.md](docs/SPEC.md).

## Setup

You need Go (the version in `go.mod`) and `git`. Optional but recommended: [golangci-lint](https://golangci-lint.run) v2.

```
git clone https://github.com/dpologdvinity/gitgarden
cd gitgarden
go build ./... && go test ./...
```

## Everyday commands

| Task | Command |
|---|---|
| Run the program on this repo | `go run . --still` |
| All tests (with race detector) | `go test -race ./...` |
| Coverage | `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` |
| Format | `gofmt -w .` (CI fails if `gofmt -l .` prints anything) |
| Vet | `go vet ./...` |
| Lint | `golangci-lint run ./...` |
| Check release config | `goreleaser check` |

## Tests

* Write the failing test first, then the code (see how existing tests are structured).
* Use only the standard library `testing` package.
* Build repositories with `internal/testrepo`; never depend on the repository you run the tests in.
* Output tests use golden files in `testdata/golden`. After an intended change, regenerate with
  `go test ./internal/<pkg> -update`, then read the diff before committing.
* Keep statement coverage at or above 80% overall (CI enforces it).

## Code style

* Every package has a package comment, and every function, type and constant has a doc comment that
  starts with its name. Explain *why* when the code is not obvious.
* gitgarden is read-only: only run `git` commands that read (`rev-parse`, `log`).
* No randomness and no wall-clock time outside `main.go`, so output stays deterministic.
* Every drawn character must be printable ASCII (0x20-0x7E).
* Do not add dependencies without a strong reason; today there are only `x/term` and `x/image`.

## Commits and pull requests

* Use [Conventional Commits](https://www.conventionalcommits.org): `feat(scope): ...`, `fix: ...`,
  `docs: ...`, `test: ...`, `refactor: ...`, `ci: ...`, `chore: ...`.
* Keep each commit one logical change.
* Open pull requests against `master`. CI must pass: dependencies, lint, tests on Linux, macOS and
  Windows, and a build of all release binaries.
* Describe what changed and why. If behavior changes, update `docs/SPEC.md`, the README and tests in the same PR.
* Never commit secrets, `.env` files or editor/agent files (`.claude/`, `CLAUDE.md`, `AGENTS.md`, ...).

## Releases

Maintainers tag `vX.Y.Z` on a commit that is green on `master`. The release workflow runs the tests
and GoReleaser publishes binaries for linux, macOS and Windows (amd64 and arm64).
