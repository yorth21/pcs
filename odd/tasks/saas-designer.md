# Feature: saas-designer

## Objective

Raise pcs from a disciplined UI auditor to a tool that designs and builds complete SaaS front-end screens at the level of a semi-senior product designer executing: the user sends a task and receives a finished, state-complete, visually consistent interface without supervising each step.

## Problem

- The PCD flow (explore → strategy → architecture → apply → verify) produces documents, but the two decisions with the most impact on the final page are improvised: which page template and which organisms compose it, and how the result is corrected after verification.
- Parts of the spec are checklist theater (KLM numbers, outcome metrics, declared cognitive-law compliance) that the model fills fluently without improving the page.
- The adapter ships one example blueprint (`ProcedureCatalog*`), not a SaaS pattern library.
- Nothing enforces consistency with the screens a project already has.
- The flow ends at verify: findings go back to the user instead of being fixed.
- Background subagents make the Stop hook block while a delegated phase is still running (observed in credi-siian session b9e4c116: 5 of 5 delegations).
- No measurement of UI quality exists; all previous validation measured process compliance.

## Why

The target user is a team of developers with no dedicated product designer. pcs must cover the execution part of that role: pattern selection, composition, states, consistency, heuristics, and accessibility. Real user research and business product decisions stay with the humans.

## Scope

- Stack-agnostic SaaS catalog in the brain (templates, organisms, decision tree, state matrices, visual consistency rules).
- React + shadcn/ui blueprints per template in the adapter.
- Shorter t2/t3 flow: explore → strategy (trimmed) → compose → apply → verify, with a bounded verify → fix loop.
- Per-project UI inventory (`pcs/design/inventory.md`) and reuse rule.
- Verifier checks the Page Composition Spec contract on the rendered UI.
- In-flight tracking for background subagents in the hooks.
- A/B benchmark (Opus + frontend-design vs claude-pcs) with a blind rubric judge.

Out of scope: more flow machinery beyond the fix loop and in-flight tracking, other stack adapters, backend work, real user research.

## Constraints

- The brain (`pcs-core`, `pcs-saas`) stays stack-agnostic; only the adapter names React, shadcn/ui, Tailwind, TanStack, React Hook Form, or Zod.
- Go standard library only; tests use `t.TempDir()`; `go test ./...` and `go vet ./...` pass (`-race` unavailable on this machine: no cgo).
- Every artifact in English; no external company names in `brain/`.
- Never modify the main `~/.claude` profile; test credentials only in test prompts, never persisted.
- Run headless pcs test sessions one at a time (low memory).

## Delivery strategy

`ask-on-risk`, deferred: the feature stays on branch `feat/saas-designer` (based on `feat/deterministic-pcd-flow`); push and PR remain the user's decision. Forecast: about 2.5k–3.5k authored lines, mostly brain content.

## Tasks

- [x] T1 — Flow: rename phase `architecture` → `compose` for t2/t3 (agent stays `pcs-architect`), bounded verify → fix loop (max 3 fix rounds, then await the user), and in-flight tracking of background delegations so the Stop hook does not block while a delegated phase runs. Route: delegated (2+ non-trivial Go files).
- [x] T2 — Brain (commit `b89c8ef`; 18 templates, 26 organisms, 19 molecules, S1–S7, Page Composition Spec in 04 §4, VC-01..16): new stack-agnostic `pcs-saas` skill (page templates, organisms, molecules, template decision tree, state matrices, container and friction rules, visual consistency and density rules, Page Composition Spec format). Route: delegated (multi-file content).
- [x] T3 — Adapter: React + shadcn/ui blueprint per page template plus pattern-to-component mapping reference. Route: delegated.
- [x] T4 — Agents and routing: trimmed strategist, architect → composer of the Page Composition Spec, builder fix mode, verifier contract checks with fix-ready findings, explorer UI inventory, orchestrator autonomy (one intake round), spec template and pcs-core references trimmed. Route: delegated.
- [x] T5 — Benchmark (commit `e2f4605`): tasks, rubric, blind judge protocol, and runner notes under `docs/bench/`. Route: delegated.
- [ ] T6 — Run the benchmark (baseline vs pcs) on the credi-siian test worktree, record scores, iterate on the catalog where pcs loses. Route: delegated per run.

## Acceptance criteria

- `go test ./...` and `go vet ./...` pass; new flow behavior covered by tests.
- A t2 flow on a real task runs explore → strategy → compose → apply → verify (→ fix → verify) without the user, and the Stop hook does not block while a background subagent runs.
- The composer outputs a Page Composition Spec that names a catalog template, the organism tree, the state matrix, and inventory reuse.
- Benchmark: pcs scores at least as high as the baseline on every rubric dimension and clearly higher on states and consistency.

## Progress

- 2026-10-27: Branch `feat/saas-designer` created; RDD is off globally (no native review ceremony).
- T1 done (delegated writer, trigger: 2+ non-trivial Go files) — commit `6500a6f`. New transition kinds `wait` and `await`; `FixRounds` counts every pcs-builder run while verify is blocked; `in_flight: [{agent_type, phase, started_at}]` with a 30 min TTL. Known risk: an allowed launch that never runs keeps its in-flight entry until the TTL.
- T5 benchmark protocol written in `docs/bench/README.md` (inline, single doc). Tasks B1 agreements directory, B2 settings, B3 operations dashboard on Portal Convenios; baseline worktrees `credi-siian-worktrees/bench-b{1,2,3}-base` from `ada76ff1`.

- T3 done (delegated) — commit `b69b947`: `05-saas-pattern-mapping.md` covers all 63 catalog IDs; 8 template blueprints under `assets/templates/`.
- T4 done (delegated) — commit `6504512`; follow-up `1ab6dd5` adds `--caps network` to the pcs Playwright MCP because `browser_run_code` does not exist in 0.0.83 and `browser_route`/`browser_network_state_set` need the network capability.
- T6 started: baselines committed (B1 `d2961e3e`, B2 `fd53ed80`, B3 `a6b95e07`); pcs binary rebuilt at `b69b947`, profile synced, doctor HEALTHY; pcs run B1 launched headless on port 5181.

## Verification evidence

- T1: `go vet ./...` and `go test ./...` pass (all packages ok; parent spot check after the writer's run).

## Next step

T2 catalog (in progress), baseline runs B1–B3, then T3 and T4.
