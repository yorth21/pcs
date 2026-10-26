<!-- pcs:ux-routing -->
## UX & Product Routing — Product-Centered Development (PCD)

Product-Centered Development (PCD) is the mandatory workflow of this orchestrator. Every frontend, UI, or product request enters this protocol automatically, in exact sequence, without the user needing to ask for UX discovery, OOUX modeling, container analysis, or heuristic verification. Never describe PCD passively—execute it.

### PCD Protocol (MANDATORY, in this order, on every request)

1. **Authorize.**
   Establish whether the user's request authorizes UI/code mutation or is read-only.
   - Requests to audit usability, inspect a flow, review a screen, compare layouts, or propose a UX design are **read-only** unless the user explicitly asks to build, implement, refactor, or write specs/code.
   - Read-only requests may run `pcs-explorer` and `pcs-verifier` (in diagnostic mode) and present findings, heuristic scores, and wireframes inline, but MUST NOT write source files or invoke `pcs-builder`.
   - If mutation intent is ambiguous, ask one concise clarification question and remain read-only until answered.

2. **Explore (`pcs-explorer`).**
   Before proposing UX flows or writing any component, map the existing product context proportionately:
   - For localized 1-file tweaks (`Tier 1`), inspect within the inline budget (<= 3 calls, ~10k tokens).
   - For any multi-file flow, new screen, or unfamiliar module (`Tier 2` / `Tier 3`), delegate `pcs-explorer` using `CodeGraph` first (`mcp__codegraph__codegraph_explore`) to inventory:
     1. Existing routes, navigation hierarchy, and URL search-param conventions;
     2. Existing shared components in `src/components/{ui,form,data-table,layout}/`;
     3. Feature-local components and hooks in `src/features/<feature>/`;
     4. Domain entities, API payloads, and current UX bottlenecks.
   - Persist exploration findings to Engram under `pcs/<feature-name>/explore`.

3. **Resolve UX Uncertainty (Max 1 Blocking Question).**
   Evaluate whether high-consequence product context is missing across three dimensions:
   - **Role & Environment**: Who is the operator, what is their domain expertise, and what is their physical/device environment?
   - **Frequency & Urgency**: Is this a high-frequency daily workbench (optimizing speed, keyboard shortcuts, density) or a low-frequency configuration flow (optimizing guided clarity)?
   - **Business & Risk Constraints**: Which actions are irreversible/destructive vs. easily reversible?
   - If a real product fork exists that changes the architecture or scope, ask **at most ONE focused question** using Lossless Blocking Prompts, then **STOP and wait**.
   - If the context is reasonably clear from the codebase and prompt, do not interrogate the user; record explicit **Declared Assumptions** in `pcs/specs/<feature-name>.md`.

4. **Classify Depth (`Tier 1`, `Tier 2`, or `Tier 3`).**
   Select the proportionate execution tier based on exploration evidence:
   - **Tier 1 — Micro-Surface Fix**: Single-component styling, token alignment, spacing rhythm, `tabular-nums`, or `aria-*` fix where user flow, domain objects, and container type do not change. Execute inline or with a single `pcs-builder` task applying `pcs-core` Plane 5 + `pcs-adapter-react-shadcn`. No durable spec file required.
   - **Tier 2 — Bounded Flow / Subfeature Enhancement**: Adding a dialog/sheet form, adding table filters/columns, or refactoring an existing screen's states. Produce a condensed 5-Plane specification in `pcs/specs/<feature-name>.md` and task checklist in `pcs/tasks/<feature-name>.md` (via `pcs-strategist` + `pcs-architect`), then delegate `pcs-builder` and `pcs-verifier`.
   - **Tier 3 — Full Feature / New Screen / Complex Workflow**: New module, multi-view workflow, or full screen redesign. Run the full 5-Plane pipeline (`pcs-explorer` -> `pcs-strategist` -> `pcs-architect` -> human checkpoint if major trade-offs exist -> `pcs-builder` -> `pcs-verifier`).

5. **Specify & Track Before the First `.tsx` Write (MANDATORY Gate).**
   For all `Tier 2` and `Tier 3` authorized work, **zero `.tsx` source files may be created or modified** until Planes 1 through 4 are documented and persisted:
   - **Planes 1 & 2 (`pcs-strategist`)**: Write Strategy (Role Profile, JTBD Functional/Emotional/Social, Declared Assumptions) and Scope (OOUX ORCA Matrix: Objects, Relationships, CTAs, Attributes; HTA/KLM Step & Click Budget; KISS scope pruning; MVP slice) into `pcs/specs/<feature-name>.md` and mirror to Engram `pcs/<feature-name>/strategy`.
   - **Planes 3 & 4 (`pcs-architect`)**: Complete `pcs/specs/<feature-name>.md` with Structure (Information Architecture <= 2 navigation levels, State-Complete User Flow covering the **Happy Path + 5 Mandatory Negative Paths**) and Skeleton (**Context-Dependent Container Decision Matrix**, **Risk-Proportional Friction Matrix**, Empirical Cognitive Laws check, and ASCII/Text Wireframe). Create `pcs/tasks/<feature-name>.md` breaking the build into ordered work units with explicit Allowed Edit Surfaces. Mirror to Engram `pcs/<feature-name>/architecture` and `pcs/<feature-name>/tasks`.
   - Inform the user in one concise line which spec (`pcs/specs/<feature-name>.md`) and task tracker (`pcs/tasks/<feature-name>.md`) were created and how many work units are scheduled.

6. **Compose Surface Task-by-Task (`pcs-builder`).**
   Delegate implementation work units sequentially to `pcs-builder`, injecting:
   - Exact `## Feature locator` (`pcs/specs/<feature-name>.md`, `pcs/tasks/<feature-name>.md`, and Engram `pcs/<feature-name>/*`);
   - Exact `## Allowed edit surfaces` for the current work unit;
   - Exact `## Skills to load before work` (`pcs-core/SKILL.md`, `pcs-adapter-react-shadcn/SKILL.md`, `pcs-phase-common.md`);
   - Requirement to close the response with `## Key Learnings`.
   - `pcs-builder` MUST enforce:
     - **KISS & Vanilla `shadcn/ui` First**: Compose primitives from `src/components/ui/*` directly; never create redundant custom wrappers around `Card`, `Dialog`, `Sheet`, or `Button`. Use `radix-ui` directly only when a required primitive does not exist in `shadcn/ui`.
     - **Strict Folder & Promotion Rules**: Place single-feature code in `src/features/<feature>/<subfeature>/{components,hooks,schemas,types}/`; promote to `src/components/{ui,form,data-table,layout}/` only when reused across >= 2 distinct features. Zero Atomic Design suffixes (`Atom`, `Molecule`, `Organism`, `Template`).
     - **Complete State Implementation**: Implement all 5 Mandatory Negative States (`Zero-Data` with onboarding CTA, `No-Results` with clear-filters CTA, contextual `Skeleton` loading, isolated `Partial Error` with retry, and inline `FieldError` validation with `aria-invalid`).
     - **Surface Discipline**: 60-30-10 semantic color budget, 3-level typography, >= 25% whitespace floor, and `tabular-nums` on all numeric/currency/date/ID values.
   - After each work unit, verify the changed files, update `pcs/tasks/<feature-name>.md` and Engram `pcs/<feature-name>/apply` and `pcs/<feature-name>/tasks`.

7. **Verify UX, Accessibility & Live Rendering (`pcs-verifier`).**
   Once `pcs-builder` completes the surface tasks, delegate `pcs-verifier` (passing `## Feature locator`, `## Allowed edit surfaces`, and `## Skills to load before work`) to audit the implementation against `pcs/specs/<feature-name>.md`:
   - **10 Usability Heuristics Audit**: Score each heuristic on the `0–4` Severity Scale (`0` = No issue, `1` = Cosmetic, `2` = Minor, `3` = Major Usability Blocker, `4` = Usability Catastrophe).
   - **4-Question Cognitive Walkthrough**: Verify every task step for goal alignment, control visibility, label association, and immediate (< 400ms Doherty threshold) feedback.
   - **5 Mandatory Negative States Check**: Confirm `Zero-Data` and `No-Results` are distinct components/branches in code and all error/loading states exist.
   - **WCAG 2.2 AA & Keyboard Walk**: Confirm semantic landmarks, visible `:focus-visible` rings, Escape/Tab focus traps in overlays, and label associations.
   - **Playwright Live Verification (Safe Server Lifecycle)**: Probe whether a local dev server is already running before starting one (reusing an active server without stopping it; or, if starting a temporary server, cleaning up the process via an `EXIT` trap so zero orphan processes remain). Inspect desktop (`1440x900`) and mobile (`390x844`) viewports via Playwright to verify visual hierarchy, absence of horizontal overflow, and interactive states; if no live server is requested or startable, perform full static code verification and note the skipped live browser check.
   - **Hard Verification Gate**: **Zero findings of Severity `3` or `4` are permitted at task closure.** If `pcs-verifier` reports any Severity `3` or `4` defect, reopen the affected task in `pcs/tasks/<feature-name>.md` and route a targeted fix to `pcs-builder` before closing.

8. **Close & Persist.**
   Ensure `pcs/specs/<feature-name>.md`, `pcs/tasks/<feature-name>.md`, and their Engram mirrors (`pcs/<feature-name>/verify` and `pcs/<feature-name>/tasks`) are synchronized. Deliver a concise final synthesis to the user covering:
   - UX & Container decisions applied;
   - Files created/updated and states verified;
   - Heuristic & Playwright verification verdict (including any residual Severity 1–2 polish notes or skipped live checks if no dev server was available);
   - Recommended next step.

### Resuming Interrupted PCD Features

When resuming an existing feature across sessions or after compaction:
1. Call `mem_context`, then `mem_search` scoped to the current project and `pcs/<feature-name>/`, followed by `mem_get_observation` for full untruncated content.
2. Read the on-disk `pcs/specs/<feature-name>.md` and `pcs/tasks/<feature-name>.md` files.
3. Reconcile disk state with Engram observations before delegating the next unfinished work unit.

<!-- gentle-ai:remote-authorization -->
## Remote operation authorization

Permission to develop locally does not authorize remote execution or file transfer. Before remote work, require explicit user authorization for the destination, operation, and credential/session to use. If any part is missing or ambiguous, ask and remain local; do not probe the destination to resolve the ambiguity.

- Do not discover, inspect, or reuse ambient SSH agents, ControlMaster sockets, credentials, authenticated sessions, or other remote access channels without explicit authorization. Their availability is not permission to use them.
- Apply this boundary regardless of the tool or spelling: direct commands, wrappers, interpreters, libraries, and delegated work do not bypass it. Pass the authorized scope to delegates; delegation cannot expand it.
- Explicitly authorized remote work is allowed within that scope. Preserve stricter user instructions and runtime restrictions; do not weaken them or change approval settings to proceed.
- Native ask rules are an additional runtime mechanism, not authorization inferred from local-development access. Automation modes and remembered approvals may suppress prompts. This behavioral contract is not a sandbox and does not guarantee a fresh human prompt for every execution.
<!-- /gentle-ai:remote-authorization -->
<!-- /pcs:ux-routing -->
