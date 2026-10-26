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
- [x] T3 (#16, #21 launcher part) Launchers: user args before `--mcp-config <file>`; per-shell escaping of `--launcher-args` (POSIX single-quote); executable launcher tests with a fake `claude`.
- [x] T4 (#18) Test isolation and Windows: every installing test sets `HOME`, `USERPROFILE`, `LOCALAPPDATA`, `CLAUDE_CONFIG_DIR`, fake-only `PATH`; platform-aware temp-dir guard; Makefile without `rm -rf`, conditional `-race`.
- [x] T5 (#21 rest) Persist `NoStatusLine`/`WithPlugins` in state and honor in `sync`; uninstall removes `statusLine` only if unchanged; keep `*.pre-pcs.bak` on uninstall.
- [x] T6 (#19, #20) Per-OS PATH/plugin hints (PowerShell user-PATH form); `--with-plugins` adds marketplace first and surfaces failures; doctor checks profile plugins, `shared-mcp.json` sync, macOS Keychain login → "unknown"; README fix.

## Acceptance criteria

The acceptance criteria listed in each issue body, expressed as tests where the issue specifies a test.

## Checks

`go vet ./...`, `go test -race ./...`, `gofmt -l .` (empty).

## Progress / Evidence

| Task | Route | Commit | Checks |
|------|-------|--------|--------|
| T1 | delegated (writer trigger: 2+ non-trivial files) | `31eceb2` | `go vet ./...` ok; `go test -race ./...` ok (all packages); `gofmt -l .` empty |
| T2 | delegated | `5045c57` | `go vet ./...` ok; `go test -race ./...` ok (all packages); `gofmt -l .` empty |
| T3 | delegated (writer trigger: 2+ non-trivial files) | `324ec83` | RED observed (POSIX exec test: `$HOME` expanded, backtick executed, `--mcp-config` first); then `go vet ./...` ok; `go test -race ./...` ok; `GOOS=windows go vet ./...` ok; `gofmt -l .` empty. `.cmd`/`.ps1` exec tests skip on macOS (run on Windows CI only) |
| T4 | delegated | `b896f7c` | `go vet ./...` ok; `go test -race ./...` ok (all packages); `GOOS=windows go vet ./...` ok; `gofmt -l .` empty; `make test` ok (`-race` auto-enabled, cgo=1); test binaries run with HOME/USERPROFILE/LOCALAPPDATA at an empty dir outside the repo leave it empty. Test-first exception: isolation leaks and the Windows `/tmp` guard path are not observable on macOS |
| T5 | delegated | `d8f5020` | Parent-verified on HEAD `64de255`: `go vet ./...` ok; `go test -race -count=1 ./...` ok (7 packages); `GOOS=windows go vet ./...` ok; `gofmt -l .` empty. RED evidence not reported (writer stopped by user before its final report) |
| T6 | delegated | `64de255` | Same parent verification as T5. RED evidence not reported (writer stopped before its final report) |

## Decisions

- T1: plain `pcs sync` (no `--scope`/`--profile`/`--profile-dir`/`--target-dir`, no `PCS_INSTALL_SCOPE`) discovers installs via `.pcs-state.json` in the global target and the default `~/.claude-pcs` profile; it syncs the single install found and, when several exist, lists them and requires `--scope` (explicit choice over implicit multi-target writes). `--profile-dir` without `--scope` implies profile scope.
- T1: the profile guard skips the `$CLAUDE_CONFIG_DIR` check when that directory already holds a pcs profile state, so `pcs sync` still works inside a `claude-pcs` session; `~/.claude` is never exempt.
- T1: uninstall without state removes launchers only when they carry the pcs launcher signature.
- T2: "no record" is a nil list (field missing or `null`); "empty record" is `[]`. Fresh installs write `[]`; legacy states stay unrecorded unless a run adds entries (partial record, under-removes only).
- T3: `--launcher-args` stays a single string flag (backward compatible, same `.pcs-state.json` field) split by `SplitLauncherArgs`: unquoted whitespace separates, single or double quotes group, no escape character (Windows backslashes stay literal), unterminated quote is an error before any write.
- T3: launchers run `claude <launcher-args> <user-args> --mcp-config <shared-mcp.json>`. Quoting: POSIX single quotes (`'` closed, backslash-escaped, reopened); PowerShell verbatim strings with `'` and U+2018–U+201B doubled; cmd.exe double quotes with `%` doubled, embedded `"` as `\"` for the program parser, and `^`-escaping of `& | < > ( ) ^ "` once cmd.exe leaves its quoted region; `setlocal DisableDelayedExpansion` keeps `!`/`^` literal.
- T3: executable launcher tests copy the test binary as a fake `claude` that echoes argv when `PCS_TEST_FAKE_CLAUDE=1` (handled in `TestMain`).
- T4: one isolation helper per package (`isolateEnv`, `isolateCLIEnv`, `isolateDoctorEnv`) also sets `APPDATA` and `XDG_*`; PATH is an empty dir (no claude at all) except in launcher exec tests (fake claude + `sh`/`env`/shell dirs).
- T4: Makefile `RACE ?=` follows `go env CGO_ENABLED`; `make test-race` always passes `-race`; `clean` uses PowerShell `Remove-Item` on Windows and `rm -rf` elsewhere (no new Go helper package). CI unchanged: Windows already runs `go test -v ./...` without `-race`.
- `AGENTS.md` section 5 updated inline by the parent to reflect the profile-scope rule.
- T5: `--no-statusline` and `--with-plugins` persist in `.pcs-state.json` and `sync` honors them unless overridden; a SHA-256 digest of the copied statusLine gates its removal on uninstall (legacy states compare against the main profile value); every `*.pre-pcs.bak` is kept and listed in the uninstall report.
- T6: per-OS hints are pure functions taking GOOS (`internal/installer/hints.go`); Windows PATH hint appends only to the user PATH via PowerShell. `--with-plugins` adds the `Gentleman-Programming/engram` marketplace before `engram@engram` and reports each failed step with its manual command. Profile doctor checks requested plugins and `shared-mcp.json` drift; login reports unknown on macOS (Keychain).
- `.codegraph/` added to `.gitignore`.

## Next step

T5.
