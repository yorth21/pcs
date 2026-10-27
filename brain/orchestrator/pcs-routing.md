<!-- pcs:ux-routing -->
## UX & Product Routing — Product-Centered Development (PCD)

PCD is the mandatory workflow for every frontend, UI, or product request. Execute it; never only describe it.

### Flow Kinds (choose at `pcs flow start`)

| Kind | Use for | Mandatory phases, in `pcs flow next` order |
|------|---------|--------------------------------------------|
| `audit` | Read-only review, critique, or usability audit of EXISTING UI; no spec needed. Never inline. | `explore` (`pcs-explorer`) -> `audit` (`pcs-auditor`) |
| `inline` | Tier 1: one-file token, spacing, `tabular-nums`, `aria-*`, or copy fix; unchanged flow and containers; <=3 source reads, 1 edited file | none; work in the main thread, then `pcs flow close` |
| `t2` | Tier 2: bounded subfeature (dialog/sheet form, table filters/columns, state refactor) | `explore` -> `strategy` (`pcs-strategist`) -> `architecture` (`pcs-architect`) -> `apply` (`pcs-builder`) -> `verify` (`pcs-verifier`) |
| `t3` | Tier 3: new screen, user flow, or feature redesign | as `t2`, full 5-Plane depth, human checkpoint on major trade-offs |

When unsure, choose the deeper kind. If an `inline` budget runs out: `pcs flow close --abandon --reason "..."`, then start `t2` or `audit`.

### PCD Protocol (MANDATORY, in order)

1. **Authorize & start.** Audits, flow reviews, and layout comparisons are read-only (`audit`) unless the user asks to build, implement, refactor, or write specs/code; if ambiguous, ask 1 question and stay read-only. Read-only means no source edits; `pcs/` artifacts are still written. Run `pcs flow start`; if `.codegraph/` is missing and CodeGraph is available, run `pcs codegraph init --cwd <project-root>` once.
2. **Explore (`pcs-explorer`).** Map routes, URL search params, shared `src/components/{ui,form,data-table,layout}/`, `src/features/<feature>/`, domain entities, and UX gaps (<=2k-token handoff, `path:line` evidence). Engram `pcs/<feature>/explore`.
3. **Resolve UX uncertainty.** Check Role & Environment, Frequency & Urgency, and Business & Risk. For a genuine fork, run `pcs flow await --question "..."`, ask ONE question via Lossless Blocking Prompts, and end the turn; the Stop hook allows it until the user answers. Non-interactive runs (no human) record a Declared Assumption instead. Otherwise record Declared Assumptions (`pcs-strategist` in the spec; the Audit Report in `audit` flows).
4. **Audit (`audit` flows, `pcs-auditor`).** Pass the explorer handoff and the dev-server URL if known. Closure: write the Audit Report verbatim to `pcs/audits/<feature>.md`, save Engram `pcs/<feature>/audit` (`capture_prompt: false`), run `pcs flow close`, and report the verdict (Severity 3–4 count) and top fixes.
5. **Specify before the first `.tsx` write (`t2`/`t3` gate).** `pcs-strategist`: Planes 1–2 (Role, JTBD, Declared Assumptions, Metrics; ORCA matrix, HTA/KLM step budget, MVP slice) -> spec + `pcs/<feature>/strategy`. `pcs-architect`: Planes 3–4 (IA <=2 levels, URL params, Happy Path + 5 Negative Paths; Container Matrix, Friction Matrix, 7 Cognitive Laws, ASCII wireframe) -> spec, `pcs/tasks/<feature>.md`, `pcs/<feature>/architecture` and `pcs/<feature>/tasks`.
6. **Compose (`pcs-builder`).** One work unit per delegation. Enforce vanilla `shadcn/ui` (no pass-through wrappers; `radix-ui` only when missing); feature code in `src/features/<feature>/<subfeature>/{components,hooks,schemas,types}/`, promoted to `src/components/{ui,form,data-table,layout}/` only across >=2 features; no Atomic Design suffixes; 5 Negative States (separate `Zero-Data` and `No-Results`, geometry-matched `Skeleton` with `CLS = 0`, isolated `Partial Error` retry, inline `FieldError` with `aria-invalid`); 60-30-10 color, 3-level typography, >=25% whitespace, `tabular-nums`.
7. **Verify (`pcs-verifier`, post-build).** Audit against the spec: 10 Usability Heuristics (`0–4`), 4-Question Cognitive Walkthrough (<400ms), 5 Negative States, WCAG 2.2 AA keyboard walk, safe Playwright lifecycle (reuse a running dev server and never kill it; `1440x900` and `390x844`, or static fallback). **Hard Gate**: zero Severity `3–4` at closure.
8. **Close, persist & resume.** Sync spec, tasks, and Engram (`pcs/<feature>/verify`, `pcs/<feature>/tasks`), run the `close` transition, then report UX decisions, verified states, verdict, and next step. Resume with `pcs flow status`, then `mem_context` -> `mem_search` (`pcs/<feature>/`) -> `mem_get_observation`, reconciled with `pcs/specs/` and `pcs/tasks/`.

### App Credentials
Pass app login credentials the user supplies only inside the delegated `pcs-auditor` or `pcs-verifier` prompt. Never write them to files, Engram, flow state, or summaries.

<!-- pcs:remote-authorization -->
## Remote operation authorization
Local permission does not authorize remote execution or file transfer. Require explicit user authorization for destination, operation, and credential/session before any remote work; never reuse ambient SSH/credentials. Pass authorized scope to delegates; delegation cannot expand it.
<!-- /pcs:remote-authorization -->
<!-- /pcs:ux-routing -->
