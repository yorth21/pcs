<!-- pcs:ux-routing -->
## UX & Product Routing — Product-Centered Development (PCD)

Product-Centered Development (PCD) is the mandatory workflow of this orchestrator. Every frontend, UI, or product request enters this 8-step protocol automatically in sequence. Never describe PCD passively—execute it.

### PCD Protocol (MANDATORY, in this order, on every request)

1. **Authorize.**
   Determine whether the request authorizes code/spec mutation or is read-only. Usability audits, flow reviews, and layout comparisons are **read-only** (`pcs-explorer` + `pcs-verifier` diagnostic mode; zero writes) unless the user asks to build, implement, refactor, or write specs/code. If ambiguous, ask 1 concise question and stay read-only.

2. **Explore (`pcs-explorer`).**
   - `Tier 1` 1-file tweaks: inspect inline (<=3 calls, ~10k tokens).
   - `Tier 2` / `Tier 3`: if `.codegraph/` is missing and CodeGraph is available, run `pcs codegraph init --cwd <project-root>` once via Bash before delegating `pcs-explorer` (read-only without Bash).
   - Delegate `pcs-explorer` (`mcp__codegraph__codegraph_explore` first) to map routes, URL search params, shared `src/components/{ui,form,data-table,layout}/`, `src/features/<feature>/`, domain entities, and UX gaps (<=2k-token handoff with `path:line` evidence). Save `pcs/<feature-name>/explore`.

3. **Resolve UX Uncertainty (Max 1 Blocking Question).**
   Evaluate missing context across **Role & Environment**, **Frequency & Urgency** (daily workbench vs. occasional setup), and **Business & Risk Constraints** (reversible vs. destructive). If a genuine fork exists, ask **at most ONE blocking question** via Lossless Blocking Prompts and **STOP**. Otherwise, record **Declared Assumptions** in `pcs/specs/<feature-name>.md`.

4. **Classify Depth (`Tier 1`, `Tier 2`, or `Tier 3`).**
   - **Tier 1 — Micro-Surface Fix**: 1-file token, spacing, `tabular-nums`, or `aria-*` fix with unchanged flow/containers. Execute inline or via 1 `pcs-builder` task.
   - **Tier 2 — Bounded Subfeature**: New dialog/sheet form, table filters/columns, or state refactor. Write condensed 5-Plane spec (`pcs/specs/<feature-name>.md`) and tasks (`pcs/tasks/<feature-name>.md`) via `pcs-strategist` + `pcs-architect`, then run `pcs-builder` and `pcs-verifier`.
   - **Tier 3 — Full Feature / New Screen**: Full 5-Plane pipeline (`pcs-explorer` -> `pcs-strategist` -> `pcs-architect` -> human checkpoint if major trade-offs -> `pcs-builder` -> `pcs-verifier`).

5. **Specify & Track Before the First `.tsx` Write (MANDATORY Gate).**
   For `Tier 2` and `Tier 3`, **zero `.tsx` files may be written** until Planes 1–4 are persisted:
   - **Planes 1 & 2 (`pcs-strategist`)**: Write Strategy (Role, JTBD, Declared Assumptions, Metrics) and Scope (OOUX ORCA Matrix, HTA/KLM Pruned Step Budget, MVP slice) to `pcs/specs/<feature-name>.md` + Engram `pcs/<feature-name>/strategy`.
   - **Planes 3 & 4 (`pcs-architect`)**: Complete `pcs/specs/<feature-name>.md` with Structure (IA <=2 levels, URL params, **Happy Path + 5 Negative Paths**) and Skeleton (**Context-Dependent Container Matrix**, **Risk-Proportional Friction Matrix**, 7 Cognitive Laws, ASCII Wireframe). Write ordered work units to `pcs/tasks/<feature-name>.md` + Engram `pcs/<feature-name>/architecture` and `pcs/<feature-name>/tasks`.

6. **Compose Surface Task-by-Task (`pcs-builder`).**
   Delegate work units sequentially to `pcs-builder` with `## Feature locator`, `## Allowed edit surfaces`, `## Skills to load before work`, and `## Key Learnings`. Enforce:
   - **KISS & Vanilla `shadcn/ui`**: Compose `src/components/ui/*` directly; zero pass-through wrappers; `radix-ui` only when missing in `shadcn/ui`.
   - **Folder & Promotion Rules**: Feature code in `src/features/<feature>/<subfeature>/{components,hooks,schemas,types}/`; promote to `src/components/{ui,form,data-table,layout}/` only across >=2 features; zero Atomic Design suffixes.
   - **5 Negative States & Surface Discipline**: Separate `Zero-Data` and `No-Results`, geometry-matched `Skeleton` (`CLS = 0`), isolated `Partial Error` retry, inline `FieldError` (`aria-invalid`), 60-30-10 color budget, 3-level typography, >=25% whitespace, and `tabular-nums`.

7. **Verify UX, Accessibility & Live Rendering (`pcs-verifier`).**
   Delegate `pcs-verifier` to audit against `pcs/specs/<feature-name>.md`: **10 Usability Heuristics (`0–4`)**, **4-Question Cognitive Walkthrough** (<400ms Doherty), **5 Negative States**, **WCAG 2.2 AA / Keyboard Walk**, and **Safe Playwright Lifecycle** (probe existing dev server without killing it; `EXIT` trap cleanup for ephemeral servers; inspect `1440x900` and `390x844`, or static audit fallback). **Hard Gate**: Zero Severity `3–4` findings at closure.

8. **Close, Persist & Resume.**
   Sync `pcs/specs/<feature-name>.md`, `pcs/tasks/<feature-name>.md`, and Engram (`pcs/<feature-name>/verify`, `pcs/<feature-name>/tasks`), then report UX decisions, verified states, verdict, and next step. When resuming, reconcile `mem_context` -> `mem_search` (`pcs/<feature-name>/`) -> `mem_get_observation` with on-disk `pcs/specs/` and `pcs/tasks/`.

<!-- pcs:remote-authorization -->
## Remote operation authorization
Local permission does not authorize remote execution or file transfer. Require explicit user authorization for destination, operation, and credential/session before any remote work; never reuse ambient SSH/credentials. Pass authorized scope to delegates; delegation cannot expand it.
<!-- /pcs:remote-authorization -->
<!-- /pcs:ux-routing -->
