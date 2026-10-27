# Fix open issues #23–#27

## Objective

Resolve open GitHub issues #23–#27: remaining main-profile write paths, Engram availability in profile installs, environment-leaking hints, profile lifecycle leftovers, and binary distribution.

## Problem / Why

- #25 and #23 are high priority: `install`/`uninstall` can still target the main profile, and a default profile install lacks Engram, which every pcs agent requires.
- #24 hints leave `CLAUDE_CONFIG_DIR` set in the caller shell (reintroduces the #11 leak).
- #26 leaves stale state, residual files, and a false doctor warning in the profile.
- #27 has no supported way to install the `pcs` binary.

## Scope

- Authorized: `internal/**`, `cmd/pcs/**`, `Makefile`, `README.md`, `docs/**`, `AGENTS.md` (section 5 only), `.github/workflows/**`, `.goreleaser.yaml`, `.gitignore`, this document.
- Out of scope: `brain/**` content; new Go module dependencies; publishing a release or pushing tags (user decision); `pcs` self-copy into the launcher dir (optional in #27, deferred).

## Constraints

- Go standard library only; tests use `t.TempDir()` and the per-package env isolation helpers; never invoke the real `claude` or touch the real home.
- `go vet ./...`, `go test -race ./...`, `GOOS=windows go vet ./...`, `gofmt -l .` (empty).
- English artifacts; Conventional Commits; no AI attribution lines in commits.
- Branch: `fix/open-issues-23-27`. RDD: off (global). Delivery: one work-unit commit per task; PR decided by the user.

## Tasks

- [x] T1 (#25) Refuse profile targets that are the home dir, an ancestor of `~/.claude`, or whose derived `.claude.json`/`settings.json` equals the main profile's; `uninstall` without `--scope` discovers like `sync`; `install` without `--scope` refuses when a profile install exists; `--force` without explicit `--scope` refused.
- [x] T2 (#23) Profile install provides Engram when the main profile has only the `engram@engram` plugin: add an `engram` server (`<resolved engram> mcp`) to `shared-mcp.json` (read-only on main); doctor `--scope profile` fails when no Engram source exists.
- [ ] T3 (#24) Hints never leave `CLAUDE_CONFIG_DIR` set: launch hint prints `claude-pcs` (absolute path when not on PATH); plugin hints scope the variable per OS.
- [ ] T4 (#26) Lifecycle leftovers: old-state `--no-statusline` unknown → keep profile's current statusLine state; no pre-pcs backup for files pcs created (record + delete on uninstall); record created files and remove if empty/unchanged; migrate old state with `schema_version`; profile doctor checks `shared-mcp.json`.
- [ ] T5 (#27) GoReleaser config + tag-triggered release workflow (windows/amd64, darwin/arm64+amd64, linux/amd64, checksums); `pcs version` prints version, commit, build date via ldflags with `debug.ReadBuildInfo` fallback; README install docs per OS (`go install`, release download).

## Acceptance criteria

The acceptance criteria listed in each issue body, expressed as tests where the issue specifies a test.

## Progress / Evidence

| Task | Route | Commit | Checks |
|------|-------|--------|--------|
| T1 | delegated (writer trigger: 2+ non-trivial files) | `d3f0d9a` fix(scope): refuse profile targets overlapping the main profile and implicit main writes | RED: 6 `TestProfileTargetOverlappingMainProfileIsRefused` subtests and 4 CLI tests (`TestUninstallWithoutScope*`, `TestUninstallForceRequiresExplicitScope`, `TestInstallWithoutScopeRefusesWhenOnlyAProfileInstallExists`) failed before the fix; GREEN: `go vet ./...` ok, `go test -race ./...` ok, `GOOS=windows go vet ./...` ok, `gofmt -l .` empty |
| T2 | delegated | `e35b30c` fix(profile): provide Engram in shared-mcp.json and fail doctor without an Engram source | RED: `TestProfileInstallProvidesEngramWhenMainHasOnlyThePlugin`, `TestProfileInstallWithoutEngramBinaryWarnsInsteadOfWritingABrokenEntry`, `TestDoctorProfileEngramSourceCheck` (3 subtests) failed and the mcp tests did not compile against the old `GenerateSharedMCPFile` signature; GREEN: `go vet ./...` ok, `go test -race ./...` ok, `GOOS=windows go vet ./...` ok, `gofmt -l .` empty |
| T3 | delegated | — | — |
| T4 | delegated | — | — |
| T5 | delegated | — | — |

## Decisions

- T2: prefer an `engram` MCP entry in `shared-mcp.json` over installing the plugin by default: deterministic, offline, no marketplace dependency. When no `engram` binary resolves, doctor fails with the install hint.

- T1: profile targets are refused when they are the home directory, the main config directory (`~/.claude`, or `$CLAUDE_CONFIG_DIR` unless it holds a pcs profile state) or any ancestor of it, or when their derived `.claude.json`/`settings.json` is a main profile file (`~/.claude.json`, `~/.claude/.claude.json`, `~/.claude/settings.json`, `$CLAUDE_CONFIG_DIR/{.claude.json,settings.json}`), using the existing `Abs`/`Clean`/`EvalSymlinks`/case-insensitive-on-Windows comparison.
- T1: an explicit target selector is `--scope`, `--profile`, `--profile-dir`, or `--target-dir`. `PCS_INSTALL_SCOPE` counts as explicit for `install` and `uninstall` target selection (consistent with `sync`), but not for `--force`: a persistent environment variable is not a per-command confirmation of a destructive target, so `--force` requires a command-line selector.
- T1: `uninstall` without a selector reuses the `sync` discovery (one install → that one; none or several → refuse non-zero); `install` without a selector refuses only when a profile install exists and no global install does. Recovery uninstall without any state now needs an explicit `--scope`.
- T2: `GenerateSharedMCPFile` adds `engram` (`ServerDefinition("engram")`, i.e. `<resolved engram> mcp --tools=agent`) whenever the main profile has no `engram` server, including the defaults case; an unresolvable binary yields no entry plus a report warning, suppressed when `--with-plugins` succeeded in the same run. `SharedMCPDrift` does not report `engram` as extra when the main profile defines none.
- T2: engram resolution is `LookupEngramBinary` (PATH via `mcp.LookPathFunc`, then `mcp.EngramFallbackPaths`); the per-package isolation helpers set `EngramFallbackPaths` to nil so a host `/opt/homebrew/bin/engram` never leaks into tests.
- T2: doctor `--scope profile` adds an `engram` check: ok naming the source (`shared-mcp.json` server or `engram@engram` in the profile `settings.json`), error otherwise with the install, sync, and `--with-plugins` hints.

## Next step

T3.
