# PCD Task List — `<feature-name>`

- **Feature Slug**: `<feature-name>`
- **Spec**: `pcs/specs/<feature-name>.md` (§3 Page Composition)
- **Inventory**: `pcs/design/inventory.md`
- **Engram Topic Keys**: `pcs/<feature-name>/tasks`, `pcs/<feature-name>/apply`
- **Active Stack Adapter**: `<adapter skill name>`
- **Scope**: front-end only — existing API contracts or typed mocks; no backend, schema, or server changes.

---

## Authorized Scope

- **Feature directory**: `<feature source directory per the adapter folder rules>`
- **Shared components touched**: `<none | paths marked extend/new in Inventory Reuse>`
- **Inventory updates**: `<new screens and shared components to append>`

---

## Tasks

One task per route or region. Each task is built in order by `pcs-builder` in a single apply delegation.

- [ ] **T-1: `<Page name>` — `<route>` (`tpl-<id>`)**
  - **Regions**: `<region> -> org-<id>`; `<region> -> org-<id>`
  - **Start from**: `<adapter>/assets/templates/tpl-<id>.tsx` via `<adapter>/references/05-saas-pattern-mapping.md`
  - **Reuse**: `<inventory component path>` (`org-<id>`, reuse | extend: `<capability>`)
  - **Target files**: `<exact paths>`
  - **Data**: `<existing contract path>` | `typed mock: <path>` (with empty and error switches)
  - **States**: `S1`, `S2`, `S3`, `S4`, `S5`, `S6`, `S7` per the State Matrix (list `n/a` cells with reason)
  - **Acceptance checks**: `AC-<TEMPLATE>-1`, ..., `AC-PAGE-1`
  - **Result**: `<pending | files changed; states implemented; typecheck/lint/build results>`

- [ ] **T-2: `<Region or secondary task>` (`org-<id>`, container `<dialog | sheet | inline | page>`)**
  - **Start from**: `<adapter blueprint or organism mapping>`
  - **Friction**: `<rule R.. and confirmation copy>`
  - **Target files**: `<exact paths>`
  - **States**: `<cells>`
  - **Acceptance checks**: `<IDs>`
  - **Result**: `<pending | ...>`

---

## Fix Rounds

Filled by `pcs-builder` in fix mode, one block per round.

### Fix round `<N>/3`
| Finding ID | Severity | Result | Change (`file:line`) or reason |
|------------|----------|--------|--------------------------------|
| `F-1` | `3` | resolved \| unresolved | `<location or reason>` |

- **Checks**: `<typecheck / lint / build results>`
