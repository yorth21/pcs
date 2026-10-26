# Fix open issues #14–#21

## Objective

Resolve open GitHub issues #14–#21: profile/global scope safety, uninstall data safety, launcher argument handling, test isolation and Windows compatibility, persisted install flags, PATH hints, plugin installation, and doctor checks.

## Problem / Why

Several defects can modify or delete data in the user's main Claude Code profile (#14, #15, #17), block normal launcher use (#16), produce real side effects during tests (#18), or give harmful advice (#19). Remaining items are correctness and UX gaps (#20, #21).

## Scope

- Authorized: `internal/**`, `cmd/pcs/**`, `Makefile`, `README.md`, `docs/**` (CLI docs only), `.github/workflows/**` (only if needed for #18), this document.
- Out of scope: `brain/**` content changes, new external Go dependencies.

## Constraints

- Go standard library only; tests use `t.TempDir()`; `go vet ./...` and `go test ./...` (with `-race` where cgo is available) must pass.
- English artifacts; Conventional Commits; no AI attribution lines in commits.
- Branch: `fix/open-issues-14-21`. RDD: off (global) — ordinary checks only.
- Delivery: forecast exceeds ~400 authored lines; one work-unit commit per task; PR slicing decided by the user at delivery time.

## Tasks

- [x] T1 (#14, #15) Scope safety: `sync` without `--scope` only targets existing installs (via `.pcs-state.json`), refuses when none; profile scope ignores `CLAUDE_CONFIG_DIR`; refuse profile dir equal to main config dir; uninstall without state removes nothing it cannot prove pcs added.
- [x] T2 (#17) Uninstall `--remove-mcp`: remove exactly recorded servers/plugins (empty list = nothing); no state file → refuse unless `--force`.
- [ ] T3 (#16, #21 launcher part) Launchers: user args before `--mcp-config <file>`; per-shell escaping of `--launcher-args` (POSIX single-quote); executable launcher tests with a fake `claude`.
- [ ] T4 (#18) Test isolation and Windows: every installing test sets `HOME`, `USERPROFILE`, `LOCALAPPDATA`, `CLAUDE_CONFIG_DIR`, fake-only `PATH`; platform-aware temp-dir guard; Makefile without `rm -rf`, conditional `-race`.
- [ ] T5 (#21 rest) Persist `NoStatusLine`/`WithPlugins` in state and honor in `sync`; uninstall removes `statusLine` only if unchanged; keep `*.pre-pcs.bak` on uninstall.
- [ ] T6 (#19, #20) Per-OS PATH/plugin hints (PowerShell user-PATH form); `--with-plugins` adds marketplace first and surfaces failures; doctor checks profile plugins, `shared-mcp.json` sync, macOS Keychain login → "unknown"; README fix.

## Acceptance criteria

The acceptance criteria listed in each issue body, expressed as tests where the issue specifies a test.

## Checks

`go vet ./...`, `go test -race ./...`, `gofmt -l .` (empty).

## Progress / Evidence

| Task | Route | Commit | Checks |
|------|-------|--------|--------|
| T1 | delegated (writer trigger: 2+ non-trivial files) | `31eceb2` | `go vet ./...` ok; `go test -race ./...` ok (all packages); `gofmt -l .` empty |
| T2 | delegated | `5045c57` | `go vet ./...` ok; `go test -race ./...` ok (all packages); `gofmt -l .` empty |
| T3 | delegated | — | — |
| T4 | delegated | — | — |
| T5 | delegated | — | — |
| T6 | delegated | — | — |

## Decisions

- T1: plain `pcs sync` (no `--scope`/`--profile`/`--profile-dir`/`--target-dir`, no `PCS_INSTALL_SCOPE`) discovers installs via `.pcs-state.json` in the global target and the default `~/.claude-pcs` profile; it syncs the single install found and, when several exist, lists them and requires `--scope` (explicit choice over implicit multi-target writes). `--profile-dir` without `--scope` implies profile scope.
- T1: the profile guard skips the `$CLAUDE_CONFIG_DIR` check when that directory already holds a pcs profile state, so `pcs sync` still works inside a `claude-pcs` session; `~/.claude` is never exempt.
- T1: uninstall without state removes launchers only when they carry the pcs launcher signature.
- T2: "no record" is a nil list (field missing or `null`); "empty record" is `[]`. Fresh installs write `[]`; legacy states stay unrecorded unless a run adds entries (partial record, under-removes only).
- `AGENTS.md` section 5 updated inline by the parent to reflect the profile-scope rule.

## Next step

T3.
