# pcs UI Quality Benchmark

Measures whether pcs produces better SaaS screens than the same model without pcs. Previous validation measured process compliance only; this benchmark measures the rendered result.

## Variants

| Variant | How it runs |
|---|---|
| **baseline** | Same model, no pcs profile. One prompt with the task text, the instruction to follow the project's existing conventions, and the frontend design skill if available. No flow, no subagents required. |
| **pcs** | `claude-pcs` headless (`-p ... --output-format stream-json --verbose`) with the same task text, in a fresh worktree. The flow runs end to end; the intake question, if any, is answered with the task's "Intake answers". |

Both variants start from the same commit in separate git worktrees of the test project, with `node_modules` linked, and must not see each other's output.

## Test bed

Portal Convenios (credi-siian frontend): React + shadcn/ui SaaS portal for credit agreements, with authenticated layout, sidebar shell, and TanStack Router file routes. The backend runs on port 5159; each worktree serves the frontend on its own port (`npm run dev -- --port <port>`), proxying `/api` to the shared backend.

## Tasks

Task text is given verbatim to both variants.

### B1 — Agreements directory (index with table + list-detail)

> Build a new "Convenios" section at `/convenios` for the logged-in user: a directory of their agreements with search and filters, and a detail view per agreement at `/convenios/$id` showing the agreement data, its associated merchants, and quick actions to simulate a credit or start an application for that agreement. Add it to the sidebar. Use the existing endpoints `GET /api/convenios/por-usuario`, `GET /api/convenios/{id}`, and `GET /api/comercio-maestro/por-usuario`. Frontend only.

Intake answers: users are commercial advisors of partner merchants; they use the portal daily; they usually have between 1 and 30 agreements.

### B2 — Settings (sectioned settings page)

> Build a new settings page at `/configuracion` with sections for account (current user data and change password), appearance (light/dark/system theme), notification preferences (typed mock data, persisted locally until a backend exists), and read-only portal information (portal parameters and app version). Add it to the user menu. Use `GET /api/users/current`, `POST /api/users/change-password`, `GET /api/portal-convenios/parametros`, and `GET /api/version`. Frontend only.

Intake answers: same users; the page is visited rarely; changing the password must be safe and clear.

### B3 — Operations dashboard (dashboard redesign)

> Replace the current home page at `/` with an operations dashboard for the advisor: KPIs for applications in progress, closed, and disbursed (counts and total amounts), a breakdown of in-progress applications by process stage, a list of applications pending disbursement, and links into the existing application lists. Use `GET /api/flujo-credito/en-tramite`, `GET /api/flujo-credito/cerradas`, `GET /api/flujo-credito/desembolsadas` (their totals and counts), and `GET /api/portal-convenios/informacion`. Frontend only.

Intake answers: the advisor opens the portal at the start of each day to see what needs attention.

## Capture

For each variant and task, a neutral capture step (not the variant's own agent) logs in and records:

1. Desktop 1440×900 and mobile 390×844 screenshots of every new or changed route in the default data state.
2. Forced states where reachable: loading (throttled network), empty (filter with no results or empty mock), error (blocked endpoint), and any validation error in forms.
3. Keyboard pass: tab order through the main task, focus visibility, and dialog focus trapping.
4. Console errors and `npm run build` result.
5. The diff stat and the list of new files.

Screenshots are renamed to neutral labels (`A`/`B`, randomized per task) before judging.

## Rubric

Each dimension is scored 1–5 by a blind judge that sees both labeled variants side by side, the task text, and screenshots of two existing screens of the app for consistency reference. The judge must cite the screenshot behind every score below 4.

| # | Dimension | 5 means |
|---|---|---|
| R1 | Task fit and hierarchy | The primary job is obvious within 5 seconds; one primary action per region; secondary actions subordinate. |
| R2 | Template and information architecture | The page pattern fits the job and data shape; navigation placement and URL state (filters, tabs, selection) are sensible. |
| R3 | State completeness | Loading, empty first-use, no results, error, and validation states are designed, not default or missing. |
| R4 | Consistency with the app | Reuses the shell, existing composite components, spacing, typography, labels, and patterns of existing screens; nothing looks foreign. |
| R5 | Visual quality and density | Clear typographic scale, aligned grid, density appropriate to data, restrained color with semantic status roles. |
| R6 | Accessibility | Labels, keyboard operability, visible focus, non-color-only meaning, sufficient contrast. |
| R7 | Microcopy | Verb-first actions, consistent terminology, helpful empty and error messages, in the product's language. |
| R8 | Responsive behavior | Usable at 390 px without horizontal scroll; tables and actions adapt deliberately. |
| R9 | Build health | Builds, no console errors, no broken routes (pass = 5, warnings = 3, failure = 1). |

Report per task: the score table for A and B, the de-randomized mapping, and the three most important differences.

## Acceptance

pcs passes the benchmark when, summed over tasks, it scores at least the baseline on every dimension and at least +1.0 average on R3 and R4.

## Results

Results are recorded per run in `docs/bench/results/<date>-<task>.md`.
