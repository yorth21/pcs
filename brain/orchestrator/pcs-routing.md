<!-- pcs:ux-routing -->
## UX & Product Routing — Product-Centered Development (PCD)

PCD is the mandatory workflow for every frontend, UI, or product request. Execute it; never only describe it.

### Flow Kinds (choose at `pcs flow start`)

| Kind | Use for | Phases, in `pcs flow next` order |
|------|---------|----------------------------------|
| `audit` | Read-only review of EXISTING UI; no spec. Never inline. | `explore` (`pcs-explorer`) -> `audit` (`pcs-auditor`) |
| `inline` | Tier 1: one-file token, spacing, `aria-*`, or copy fix; <=3 source reads, 1 edited file | none; main thread, then `pcs flow close` |
| `t2` | Bounded subfeature (form, table filters/columns, one region, state refactor) | `explore` -> `strategy` (`pcs-strategist`) -> `compose` (`pcs-architect`) -> `apply` (`pcs-builder`) -> `verify` (`pcs-verifier`); blocked verify -> fix round (`pcs-builder`) -> verify, max 3 rounds |
| `t3` | New screen, user flow, or feature redesign | as `t2`, one Page Composition block per page |

When unsure, choose the deeper kind. If an `inline` budget runs out: `pcs flow close --abandon --reason "..."`, then start `t2` or `audit`.

### PCD Protocol (MANDATORY, in order)

1. **Start.** Audits and reviews are read-only (`audit`) unless the user asks to build; if ambiguous, ask 1 question. Run `pcs flow start`; if `.codegraph/` is missing, run `pcs codegraph init --cwd <project-root>` once.
2. **Intake (`t2`/`t3`).** At most one round per the Autonomy Rule, then no more questions.
3. **Explore (`pcs-explorer`).** Routes, app shell, shared components, data contracts, UX gaps (<=2k tokens, `path:line`); in build flows create or refresh `pcs/design/inventory.md`. Engram `pcs/<feature>/explore`.
4. **Audit (`audit` flows, `pcs-auditor`).** Pass the explorer handoff and dev-server URL. Write the Audit Report verbatim to `pcs/audits/<feature>.md`, save Engram `pcs/<feature>/audit` (`capture_prompt: false`), run `pcs flow close`, report the Severity 3–4 count and top fixes.
5. **Strategy (`pcs-strategist`).** Role and environment, primary and secondary jobs, ORCA objects with action risk, classification inputs (cardinality, frequency), data source, scope slice, decision-driving assumptions -> spec Planes 1–2, `pcs/<feature>/strategy`.
6. **Compose (`pcs-architect`).** Decision tree -> one catalog template per page; Page Composition Spec (region -> organism tree, data contract, `S1`–`S7` state matrix, actions and friction, inventory reuse, `AC-*` checks) -> spec; one task per route or region with template IDs and the adapter blueprint -> `pcs/tasks/<feature>.md`; Engram `pcs/<feature>/compose` and `pcs/<feature>/tasks`. No source write before this phase.
7. **Apply (`pcs-builder`).** One delegation builds every task: adapter blueprint first, inventory reuse, every state cell, project typecheck/lint/build, per-task report.
8. **Verify (`pcs-verifier`).** Renders the UI and checks the spec contract: anatomy, `AC-*`, every state forced live, `VC-*`, inventory reuse, keyboard and WCAG 2.2 AA, `1440x900` and `390x844` captures in `.pcs/playwright/`; fix-ready findings into the Verification Report. Envelope `success` only with zero Severity `3–4`; `partial` starts the fix loop.
9. **Fix loop.** Follow `pcs flow next`: fix-round `delegate` to `pcs-builder` with the blocking findings, verify re-runs automatically; after 3 rounds `await` the user.
10. **Close & resume.** Run the `close` transition, then report the template chosen, verified states, verdict, and next step. Resume with `pcs flow status`, then `mem_search` (`pcs/<feature>/`) -> `mem_get_observation`, reconciled with `pcs/specs/` and `pcs/tasks/`.

### App Credentials
Pass user-supplied app credentials only inside the delegated `pcs-auditor` or `pcs-verifier` prompt. Never write them to files, Engram, flow state, or summaries.

<!-- pcs:remote-authorization -->
## Remote operation authorization
Local permission does not authorize remote execution or file transfer. Require explicit user authorization for destination, operation, and credential/session before any remote work; never reuse ambient SSH/credentials. Pass authorized scope to delegates; delegation cannot expand it.
<!-- /pcs:remote-authorization -->
<!-- /pcs:ux-routing -->
