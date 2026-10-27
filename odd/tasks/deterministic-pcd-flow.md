# ODD Task Tracker — Deterministic PCD Flow

## Objective
Make the Product-Centered Development (PCD) workflow hard to bypass. The coordinator must not be able to finish a turn that skipped mandatory phases, and phase completion must be recorded by the CLI and hooks instead of being self-reported by the model.

## Problem
Every PCD gate (Authorize, spec gate, Severity 3–4 hard gate, Engram persistence) is prompt text only. In a real session ("review the UI of feature/simulador-credito"), the coordinator audited inline with about 6 tool calls. It did not delegate to `pcs-explorer` or `pcs-verifier` and persisted nothing. Root causes:
- The routing allows inline work below a ">3 calls" threshold.
- Read-only audits have no tier.
- The verifier requires a spec that existing UI does not have.
- No hook, CLI state, or permission rule checks phases.

## Why
gentle-ai follows a "model proposes, CLI disposes" design. Its Go CLI returns a typed `next_transition`, and a `Stop` hook blocks ending the turn while required work is missing. pcs adopts the same pattern, scoped to PCD phases.

## Scope
- Routing text: an explicit audit tier, no call-count loophole, verifier baseline mode for existing UI.
- `pcs flow` CLI state machine with a per-project state file.
- Claude Code hooks (`SubagentStop`, `Stop`, `UserPromptSubmit`) installed by default in profile scope.
- A read-only `pcs-auditor` agent for diagnostics.
- Iterative validation against `A:/repositories/credi-siian/src/frontend`.

## Constraints
- Go standard library only; tests use `t.TempDir()`; `go test -race ./...` and `go vet ./...` must pass.
- English artifacts; no external company names in `brain/` or the CLI.
- Never modify the main `~/.claude` profile.
- Test credentials for the sample app are passed only in test prompts. They are never written to the repository, Engram, or state files.

## Tasks
- [x] T1: Routing and agent text fixes (audit tier, no inline threshold, verifier baseline mode, `pcs-auditor`), plus the main-thread generic-subagent denial in `pcs hook pre-tool-use`. Route: delegated writer. Commits: `cef6ff3` (hook denial), `1d8390a` (`pcs-auditor` agent and wiring), `b0f5b39` (flow-first routing and mandatory `**Status**` line).
- [x] T2: `pcs flow` state machine (`start`, `status`, `next`, `record`, `close`) with `.pcs/flows/<feature>.json` and typed `next_transition` JSON. Route: delegated writer. Commit: `8faae03`.
- [x] T3: Hook commands (`pcs hook pre-tool-use|subagent-stop|stop|user-prompt-submit`, including the main-thread `Bash` allowlist) and their installation in `settings.json`, the default for profile scope (`--no-hooks` opts out, `--with-hooks` for other scopes; sync re-renders, uninstall removes, doctor reports `flow-hooks`). Route: delegated writer. Commits: `91375de` (hook package), `e735c12` (installer, mcp merge, doctor, docs), `b24e9e1` (stop reason punctuation).
- [ ] T4: Reinstall the profile from the branch build, then run the validation loop in credi-siian (audit and build prompts) and compare the results with gentle-ai patterns. Route: delegated verification.

## Acceptance Criteria
- A read-only UI audit request produces a `pcs-explorer` and a `pcs-auditor`/`pcs-verifier` delegation, recorded in the flow state.
- The `Stop` hook blocks a turn that has an open flow with pending mandatory phases, and returns the exact next command.
- The main `~/.claude` profile stays byte-identical.

## Progress
- Baseline run started: headless `claude-pcs -p` audit of `src/features/perfil`.

- T2/T3 verification (writer, Windows): `go vet ./...` clean; `go test ./...` all 9 test packages ok; `go test -race ./...` unavailable ("-race requires cgo"; no gcc on PATH); `go build -o <scratchpad>/bin/pcs.exe ./cmd/pcs` ok; smoke test in a temp project: `pcs flow start --feature demo --kind audit --json` returned a `delegate` transition to `pcs-explorer`, `pcs flow next --json` returned the same transition, a main-thread Grep on `src` piped into `pcs hook pre-tool-use` returned `permissionDecision: deny` with the delegation instruction, a subagent Grep printed nothing (allow), and `pcs hook stop` returned `decision: block` listing `explore, audit`.
- Design deviations: writes under `.pcs/` are denied (state is CLI-owned); `npx` is allowlisted with any arguments; Bash command substitution and file redirections are never allowlisted; `hook-errors.log` is appended only when `<root>/.pcs/` already exists; flow hooks are also identified by the binary recorded in `.pcs-state.json`.
- T1 verification (writer, Windows): `go vet ./...` clean; `go test ./...` all 9 test packages ok; `go build -o <scratchpad>/bin/pcs.exe ./cmd/pcs` ok; a main-thread PreToolUse payload with `tool_name: "Agent"` and `subagent_type: "Explore"` piped into `pcs hook pre-tool-use` returned `permissionDecision: deny` ("Use the pcs-* agent returned by pcs flow next"), and `claude-code-guide` returned no output (allow).
- T1 design notes: `pcs-auditor` is read-only and declares no `mem_save`; the Coordinator writes `pcs/audits/<feature>.md` and Engram `pcs/<feature>/audit`. Playwright tools are listed by explicit `mcp__playwright__browser_*` names instead of a wildcard. `pcs-verifier` returns `blocked` without a spec and recommends `pcs-auditor`. Always-loaded CLAUDE.md blocks shrank (orchestrator + routing from 9,906 to about 8,800 bytes).

## Next Step
T4: reinstall the profile from the branch build and run the credi-siian validation loop.
