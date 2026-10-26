# ODD Task Tracker — `pcs-cli` (Product Composition System Go CLI)

## Objective
Build the standalone, zero-external-dependency Go CLI (`pcs`), embedding `brain/**` and `docs/skill-style-guide.md` via `//go:embed`, configuring `engram`, `context7`, and `codegraph` MCP infrastructure alongside the `pcs` Brain (`orchestrator`, `ux-routing`, 5 `pcs-*` native subagents, `pcs-core`, and `pcs-adapter-react-shadcn`), and providing full lifecycle commands (`install`, `sync`, `uninstall`, `status`, `doctor`, `skill-registry refresh`, `codegraph init`, `version`) with comprehensive table-driven tests (`t.TempDir()`) and work-unit Conventional Commits.

## Work Units

- [x] **WU-1: Commit Brain Baseline & Build Embedded Assets + Marker Engine**
  - Commit the audited `pcs-brain` baseline (`brain/`, `docs/`, `README.md`, `AGENTS.md`, `odd/tasks/pcs-brain.md`) with a clean Conventional Commit (`9013627`).
  - Create `go.mod`, `.gitignore`, and root `embed.go` (`//go:embed all:brain docs/skill-style-guide.md`).
  - Implement `internal/assets/` (loads embedded orchestrator blocks, base `codegraph-guidance` / `persona` / `engram-protocol` blocks, native subagents, and skill directories) + `internal/assets/assets_test.go`.
  - Implement `internal/marker/` (idempotent HTML-comment section injector, updater, remover, and inspector for `<!-- ... -->` blocks) + `internal/marker/marker_test.go` (`3ffa7b9`).
- [x] **WU-2: MCP Infrastructure Configurator (`engram`, `context7`, `codegraph`) & Installer Engine**
  - Implement `internal/mcp/` (idempotent merger/unmerger for `.claude.json` MCP servers [`engram`, `context7`, `codegraph`] and `settings.json` permissions/hooks/plugins, preserving user keys and creating `.bak` backups) + `internal/mcp/mcp_test.go`.
  - Implement `internal/installer/` (`Install`, `Sync`, `Uninstall`, `Status` supporting `--scope global|workspace|profile` with `global` as default, respecting `PCS_INSTALL_SCOPE` and `CLAUDE_CONFIG_DIR`, `--target-dir`, `--adapter`, `--persona gentleman|neutral`, `--with-mcp=true|false`, `--dry-run`, cross-platform profile launchers (`sh`, `.cmd`, `.ps1`), state persistence in `.pcs-state.json`, and agent ownership tracking in `.pcs-native-agent-ownership.json`) + `internal/installer/installer_test.go` (`1622057`).
- [x] **WU-3: Skill Registry Generator, Ecosystem Doctor & CLI Command Router**
  - Implement `internal/registry/` (`pcs skill-registry refresh [--cwd <dir>]` scanning installed & project `SKILL.md` frontmatter and writing `.atl/skill-registry.md`) + `internal/registry/registry_test.go`.
  - Implement `internal/doctor/` (`pcs doctor` checking `engram`, `codegraph`, `npx` [`context7`], embedded assets, `CLAUDE.md` markers, native agents, and skills) + `internal/doctor/doctor_test.go`.
  - Implement `internal/cli/` and `cmd/pcs/main.go` (`install`, `sync`, `uninstall`, `status`, `doctor`, `skill-registry refresh`, `version`, `help`) + `internal/cli/cli_test.go` (`55fb9e8`).
- [x] **WU-4: End-to-End CLI Verification, Docs Update & Work-Unit Commits**
  - Run `go test -v -race ./...` and `go vet ./...` (all 7 test packages 100% green).
  - Build `./bin/pcs` (ensuring `bin/` is gitignored) and verify `pcs version`, `pcs help`, `pcs doctor`, `pcs install --dry-run`, and end-to-end temp profile install/sync/status/uninstall.
  - Update `README.md`, `AGENTS.md`, and `docs/architecture.md` with CLI installation, profile setup (`CLAUDE_CONFIG_DIR="$HOME/.claude-pcs"` and `claude-pcs` wrapper), and command reference.
  - Commit each work unit cleanly using Conventional Commits (zero AI attribution).
- [x] **WU-5: Resolve Issues #1–#10 (Granular Flags, Non-Destructive Merging, Namespaced Registry, Cross-Platform Portability, Token Budget & Portable Docs)**
  - Decouple `--mcp-guidance` (default `true`), `--write-mcp-config` (default `false`), and `--write-hook` (default `false`) in `install` and `sync` (`#1`, `#8`).
  - Preserve pre-existing MCP server entries (such as `engram`) and track exact added deltas in `.pcs-state.json` for surgical `uninstall` (`#2`).
  - Namespace skill registry output to `.atl/pcs-skill-registry.md` and clean up on workspace `uninstall` (`#3`).
  - Enforce LF line endings via `.gitattributes`, add cross-platform CI (`.github/workflows/ci.yml`), fix PowerShell/Windows path handling and `os.TempDir()` guards (`#4`).
  - Preserve user content outside `pcs:*` markers byte-for-byte (including CRLF and triple newlines) and create one-time `.pre-pcs.bak` backups (`#5`).
  - Add `--profile-dir` and `$CLAUDE_CONFIG_DIR` resolution in profile scope and substitute installed skills paths (`#6`).
  - Align `pcs-explorer`, `pcs-strategist`, and `pcs-architect` CodeGraph fallback with read-only tool permissions (`#7`).
  - Compact `CLAUDE.md` orchestrator/routing blocks (`<= 12 KB`), `pcs-phase-common.md` (`<= 5 KB`), and native subagents (`<= 6.5 KB`) with automated size budget tests (`#9`).
  - Replace machine-local `file:///Users/...` links in `README.md` and `AGENTS.md` with portable relative links and enforce via unit tests (`#10`).
