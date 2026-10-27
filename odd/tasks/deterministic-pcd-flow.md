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
- [x] T5: Iteration 1 fixes from headless `claude-pcs -p` runs (Claude Code 2.1.310 hook payloads). Route: delegated writer. Commits: `4cfed80` (envelope parsing, monotonic phases, verify acceptance, manual-record denial, duplicate-run guard, abandon reason length, `.pcs/.gitignore`), `893e382` (pcs-owned Playwright MCP server and docs).
  - Evidence: the `SubagentStop` payload carries `agent_type`, `agent_id`, `agent_transcript_path`, and `last_assistant_message` (the full final subagent text). A `pcs-explorer` run ended with `**Status**: success` / `**Skill Resolution**: paths-injected`, yet the hook recorded `unverified`. The old regex matches that text in isolation, so the failure came from reading the transcript; the hook now reads `last_assistant_message` first and falls back to the transcript.
  - Evidence: because `pcs flow next` kept saying "delegate explore", the coordinator relaunched `pcs-explorer` (duplicate run). It then ran `pcs flow record --phase explore --status done` from main-thread Bash, which made the gate self-reported again. Main-thread `pcs flow record` is now denied. A done phase is never downgraded by a later run without a status, every run is listed under `runs`, and `pcs flow next` names an earlier unverified run.
  - Evidence: Playwright screenshots (`*-1440.png`) and `.playwright-mcp/` were written into the project root. The profile `shared-mcp.json` now adds `@playwright/mcp@0.0.83 --output-dir .pcs/playwright` when the main profile has no `playwright` server, and `pcs flow start` writes `.pcs/.gitignore` (`*`).
  - Evidence: `partial` counted as done for every phase. `verify` now requires `success`; a `partial` result is `blocked` with a Severity 3-4 note.
  - Abandon guard: the simpler option was chosen. `--abandon` stays allowed but needs a `--reason` of at least 15 characters; the transcript cancel-intent check was not implemented.
- [x] T6: Session-bound flows. Route: delegated writer. Commit: `c8a39bc`.
  - Evidence: a headless `claude-pcs` session ran `pcs flow start` and was killed (OS low memory) mid-flow. The flow stayed in `.pcs/active`, so every later, unrelated session in the repo had its reads denied by `PreToolUse` and its turn blocked by `Stop` until someone ran `pcs flow close --abandon`.
  - Fix: every hook event (`user-prompt-submit`, `pre-tool-use`, `stop`, `subagent-stop`) claims an unowned active flow for its stdin `session_id` (`owner_session_id`, `claimed_at`). A flow owned by another session is foreign: `stop` never blocks, `pre-tool-use` gates as if no flow were active with the adopt/abandon reason, `user-prompt-submit` appends the same notice, and `subagent-stop` records nothing into it. `pcs flow adopt` clears the owner so the next event of the running session claims it. `updated_at` is refreshed on every state write, and `pcs flow start` auto-abandons an active flow idle for more than 12 hours (reason `stale: no activity for >12h`) and reports it. State files without these fields load and are claimed by the first event.
  - Deviations: the CLI has no session id, so the 12-hour staleness rule applies to any active flow, owned or not, and falls back to `created_at` for state files without `updated_at`. A payload without `session_id` neither claims nor counts as foreign, so it keeps the previous behavior.
- [x] T7: Interactive blocking question. Route: delegated writer. Commit: `05a15cc`.
  - Rationale: PCD lets the coordinator ask at most one blocking question and then stop, but the `Stop` hook blocked ending the turn while phases were pending. In an interactive session the coordinator looped on stop-hook continuations up to the 8-block cap or fabricated the answer, which made pcs unusable in real interactive work.
  - Fix: `pcs flow await --question "..."` sets `awaiting_user` (question, `asked_at`) on the active flow owned by the caller's session (`$CLAUDE_CODE_SESSION_ID`, or `--session`); it requires a non-empty question, claims an unowned flow, and refuses a foreign one. `stop` does not block while `awaiting_user` is set, and its block reason names the escape. The owning session's next `user-prompt-submit` clears the mark and prefixes "The user answered your blocking question: continue the flow with `pcs flow next`." `pre-tool-use` is unchanged. Routing step 3 and the Lossless Blocking Prompts section run `await` before the question; non-interactive runs record a Declared Assumption instead.
  - Deviations: the CLI reads the session from `CLAUDE_CODE_SESSION_ID` (Claude Code sets it for Bash commands and it equals the hook `session_id`); without it, `await` behaves like a payload without `session_id` and is never foreign. A foreign `user-prompt-submit` keeps the owner's mark.
- [x] T8: Root drift and screenshot location. Route: delegated writer. Commit: `93c5c12`.
  - Rationale: the Playwright MCP runs with `--output-dir .pcs/playwright` relative to the session cwd (`<repo>/src/frontend`), so it created `<repo>/src/frontend/.pcs/`. Root resolution walked up to the nearest `.pcs/`, so the flow root silently moved from the git toplevel to `src/frontend`, and the coordinator wrote `pcs/audits/...` there while earlier flows lived at the repo root. That `.pcs/` had no `.gitignore` and showed up as untracked, and the auditor saved screenshots with bare filenames (`sim-desktop-error.png`) into the cwd instead of the output dir.
  - Fix: `ResolveRoot` treats a directory as a pcs root only when it holds `.pcs/flows/` or `.pcs/active`; otherwise it falls back to the git toplevel, then the cwd. `pcs hook user-prompt-submit` and `pcs hook subagent-stop` write `<cwd>/.pcs/.gitignore` (`*`) when `<cwd>/.pcs/` exists, best effort: they never create `.pcs` and never fail. `pcs-auditor` always passes screenshot filenames as `.pcs/playwright/<name>.png`.
  - Tests: a bare `.pcs/playwright` in a subdirectory of a git repo resolves to the git toplevel (RED observed against the old resolution); `.pcs/flows` or `.pcs/active` in a subdirectory resolves to it; both hooks write the `.gitignore` only when `<cwd>/.pcs` exists and keep an existing one; `pre-tool-use` does not write it.
- [x] T9: Status-line flow segment. Route: delegated writer. Commit: `a281b7b`.
  - `pcs statusline` (also `pcs flow statusline`) prints one line for the active flow of the root resolved from `--cwd`, else the statusLine stdin `workspace.current_dir` or `cwd`: `pcs:<feature> <kind> ▸ <phase> <i>/<n>` (with `blocked`/`unverified`/`stale` when not pending), `▸ awaiting user`, `▸ close`, `▸ other session`, or `▸ inline reads r/3`. It prints nothing without a flow, never reads stdin from a terminal, and always exits 0.
  - `pcs statusline-wrap --state <.pcs-state.json>` reads stdin once, runs `flow_statusline_base_command` through Git Bash (`sh` off Windows, `cmd /C` fallback) with the same stdin and a 2s timeout, and prints the base output, ` │ `, and the segment. A failing base still prints the segment.
  - Profile installs wrap the statusLine by default (`--with-flow-statusline`, persisted as `flow_statusline`) with the same absolute binary as the hooks. The replaced value is stored as `flow_statusline_base`; opt-out and uninstall restore it exactly while the wrapper is still in place, sync is idempotent, and other scopes never wrap.
  - Deviation: `--no-statusline` also turns the default off (an explicit `--with-flow-statusline` still wraps an empty base). This keeps the persisted `--no-statusline` and legacy-migration contracts intact.
- [x] T10: Subagent record denial. Route: delegated writer. Commit: `53c5068`.
  - `pre-tool-use` denies Bash running `pcs flow record` regardless of `agent_id`, and also denies Edit/Write under `.pcs/` from subagents, so no Claude thread can self-report a phase or rewrite flow state. Other subagent tools stay ungated. Only a human terminal outside Claude can run `pcs flow record`.
- [x] T11: Verify binding (receipt idea). Route: delegated writer. Commit: `4e57b17`.
  - `subagent-stop` stores a working-tree `fingerprint` on a done `verify` or `audit` phase: SHA-256 of `git diff HEAD --binary` plus the untracked, non-ignored file names and contents, excluding `.pcs/` and `pcs/` (the empty tree when the repository has no commits; `unavailable` without git, which never marks a phase stale).
  - `pcs flow close` on `t2`/`t3` recomputes it; on a mismatch it marks `verify` `stale`, refuses with "Source changed after verify; relaunch pcs-verifier (pcs flow next)", and `pcs flow next` delegates `pcs-verifier` again. `stop` recomputes only once every phase is done, so a stale verify blocks the turn as pending.
  - Deviation: a commit made after verify also changes the fingerprint (the diff base moves), so it conservatively requires a new verify. Tests were written alongside the implementation (no separate RED run).
- [x] T4: Reinstall the profile from the branch build, then run the validation loop in credi-siian (audit and build prompts) and compare the results with gentle-ai patterns. Route: parent-run headless validation (`claude-pcs -p --output-format stream-json`), one run at a time after an OS low-memory kill of parallel runs.
  - Baseline (before this feature): an audit of `perfil` made 10 main-thread Bash reads, 0 delegations, and no artifacts.
  - Iteration 1 (audit of `perfil`, with the dev server): ran `pcs flow start`, then `pcs-explorer` and `pcs-auditor`, with 26 Playwright calls and a structured report. Bugs found: phases recorded as `unverified` (the transcript read), manual `pcs flow record`, a duplicate explorer run, and screenshots left in the project. Fixed in T5.
  - Iteration 2: both runs were killed by low memory. Their orphaned processes left flows open, which blocked later sessions. Fixed in T6.
  - Iteration 3:
    - A `t2` build of the simulator focus and button hierarchy in an isolated worktree recorded all 5 phases through the hook (`success`/`partial` envelopes, one run each) and changed 3 files. The verifier found 0 Severity 3–4 issues, and the flow closed.
    - An audit of `simulador-credito` (static, no dev server) recorded explore and audit through the hook, with 0 main-thread source reads. It wrote `pcs/audits/simulador-credito-audit.md`, saved Engram, and closed.
  - Iteration 4 (interactive blocking question): the coordinator explored, ran `pcs flow await`, asked one closed question with evidence, and ended the turn. The resumed session (`--resume`, answer `1`) cleared the wait, ran strategy, architecture, apply and verify (all recorded by the hook), and closed the flow.
  - The main `~/.claude` profile stayed byte-identical across every reinstall and sync.
  - Remaining gap compared with gentle-ai: verifier Severity 3–4 is enforced only through the `partial` → blocked mapping.

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

- T5 verification (writer, Windows): `go vet ./...` clean; `go test ./...` all 9 test packages ok; `go build -o <local-bin>/pcs.exe ./cmd/pcs` ok. Smoke test in a temp project with an active audit flow: a `SubagentStop` payload whose `last_assistant_message` ends with the observed text (`agent_type` `pcs-explorer`, missing transcript) recorded explore `done` (envelope `success`). A second run with no status line kept explore `done` and appended an `unverified` run. Main-thread Bash `pcs.exe flow record ...` was denied with the manual-record reason.
- T6 verification (writer, Windows): `go vet ./...` clean; `go test ./...` all 9 test packages ok; `go build -o <local-bin>/pcs.exe ./cmd/pcs` ok. Smoke test in a temp project with an audit flow: a `stop` payload with `session_id` A claimed the flow (`owner_session_id: "A"`) and returned `decision: block`; a `stop` payload with `session_id` B printed nothing; a main-thread `pre-tool-use` Grep on `src` from B returned `permissionDecision: deny` with "An unfinished PCD flow `demo` from another session is active. Resume it with `pcs flow adopt` or discard it with `pcs flow close --abandon --reason \"...\"`, then start yours."
- T7 verification (writer, Windows): `go vet ./...` clean; `go test ./...` all 9 test packages ok; `go build -o <scratchpad>/bin/pcs.exe ./cmd/pcs` ok. Smoke test in a temp project with a `t2` flow and session A: `stop` returned `decision: block` naming `pcs flow await --question \"...\"`; `await` without a question and `await` from session B exited 1; `await` from A printed the confirmation; `stop` printed nothing; `user-prompt-submit` returned the "The user answered your blocking question" context; `stop` blocked again.
- T8 verification (writer, Windows): `go vet ./...` clean; `go test ./...` all 9 test packages ok; `go build -o <local-bin>/pcs.exe ./cmd/pcs` ok. `pcs-auditor.md` is 6,326 bytes (cap 6,656).

- T9–T11 verification (writer, Windows): `go vet ./...` clean; `go test ./...` all 9 test packages ok; `go build -o <scratchpad>/bin/pcs.exe ./cmd/pcs` ok. Smoke tests in a temp git repo with a `t2` flow: `pcs statusline` printed `pcs:simulador-rango-monto t2 ▸ verify 5/5`; `statusline-wrap` with base `echo base` printed `base │ pcs:simulador-rango-monto t2 ▸ verify 5/5`; a subagent Bash `pcs flow record` returned `permissionDecision: deny`; after the verifier `SubagentStop` recorded a fingerprint and `src/app.tsx` was edited, `pcs flow close` exited 1 with the stale message, `pcs flow next` delegated `pcs-verifier`, and the segment read `verify 5/5 stale`.

## Next Step
MVP complete. Reinstall the profile from the branch build (`pcs sync --scope profile`) so the status line is wrapped, then confirm the segment in an interactive `claude-pcs` session.
