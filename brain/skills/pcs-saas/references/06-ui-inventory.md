# 06 — Per-Project UI Inventory (`pcs/design/inventory.md`)

The UI inventory records what a project already has so new pages reuse it instead of reinventing it. The explorer generates or refreshes it; the composer reads it before writing a Page Composition Spec; the verifier checks new pages against it. It is a project artifact, written in English, and never names this catalog's adapter components unless the project actually uses them.

---

## 1. When to Generate or Refresh

| Situation | Action |
|-----------|--------|
| File missing and the project has `>= 1` UI screen | Generate the full inventory before composing. |
| File missing and the project has no UI yet | Create it with the shell section "planned" and empty lists; conventions come from catalog defaults. |
| File exists, `Last refreshed` older than the last commit touching UI directories, or a screen in scope is not listed | Refresh the affected sections only; keep manual notes. |
| Feature added new screens or organisms | Builder/verifier append them before the flow closes. |

## 2. Procedure (Explorer)

1. Locate routes/pages: query the codebase graph for route definitions and page entry components; fall back to bounded file-pattern searches of the routing directory.
2. Locate the shell: find the layout that wraps authenticated routes; record sidebar items (order, labels, icons), top bar contents, banner slot, and breakpoints.
3. For each screen, classify it against `01-page-templates.md` by its anatomy (not its name) and record the template ID; mark `unclassified` with a reason when no template fits.
4. Locate shared components used by `>= 2` screens or living in a shared components directory; map each to the catalog organism (`org-*`) or molecule (`mol-*`) it implements; record its path and its public variants/props in plain words.
5. Extract tokens and conventions: color roles, spacing scale, radii, type sizes, density, date/money formatting helpers, status vocabularies per object, action labels, toast/feedback helper, and confirmation helper.
6. Record deviations: places where screens contradict each other or the catalog (different labels for the same action, two table implementations, color-only status).
7. Write the file in the format of §3 and set `Last refreshed` to the current date and commit.

Limits: keep the inventory under ~400 lines; list at most 50 screens individually (group the rest by area); never paste source code.

## 3. File Format

```markdown
# UI Inventory — {project name}

- **Last refreshed**: {YYYY-MM-DD} at commit `{short sha}`
- **UI root**: `{path}`; **routing**: `{path or mechanism}`

## App Shell
- **Implements**: `tpl-app-shell` (variant: sidebar | top-nav | minimal)
- **Layout component**: `{path}`
- **Sidebar items** (in order): {label} → `{route}` ; …
- **Top bar**: {workspace switcher yes/no, global search yes/no + shortcut, notifications yes/no, account menu yes/no}
- **Breakpoints**: {values}; **mobile navigation**: {drawer | bottom bar | none}

## Screens
| Route | Screen name | Template ID | Primary object | Container for create/edit | Notes |
|-------|-------------|-------------|----------------|---------------------------|-------|
| `/customers` | Customers | `tpl-index-table` | Customer | dialog | — |

## Organisms and Molecules
| Project component | Path | Implements | Variants / options | Used by (screens) | Gaps vs catalog |
|-------------------|------|------------|--------------------|-------------------|-----------------|
| `{ComponentName}` | `{path}` | `org-data-table` | sortable, selectable, density toggle | Customers, Invoices | no bulk bar |

## Tokens and Conventions
- **Color roles**: {role → token name}
- **Spacing scale**: {steps in use}; **radii**: {values}; **type sizes**: {values}
- **Density default**: {compact | comfortable} for {templates}
- **Formatting helpers**: money `{path}`, dates `{path}`, numbers `{path}`
- **Feedback helpers**: toast `{path}`, confirmation `{path}`, empty state `{path}`
- **Status vocabularies**: {Object}: {status → role}, …
- **Action glossary**: {canonical label} (not: {synonyms found})

## Deviations
| ID | Where | Deviation | Catalog rule | Decision (fix / accepted + reason) |
|----|-------|-----------|--------------|------------------------------------|
| `DEV-01` | Invoices page | "Remove" used for permanent delete | `05` §9.3 | fix when touched |
```

## 4. Reuse Rule (Composer and Builder)

Apply in this order for every organism and molecule in a Page Composition Spec:

1. **Reuse**: an inventory component implements the same catalog ID and covers the needed variant → use it unchanged. Mandatory.
2. **Extend**: it implements the same ID but lacks a needed capability (e.g., bulk selection) → extend it in place, backward compatible; record "extend" with the capability added.
3. **New**: no inventory component implements the ID → create one shared component that implements the catalog contract, placed in the project's shared components location; record "new" with "no component implements `{org-id}`".
4. **Deviate**: an inventory component exists but cannot be used (documented constraint) → justification required naming the constraint; add a `DEV-*` row.

Hard constraints:

- Two components implementing the same catalog ID in one project is a deviation, never a silent choice.
- New pages adopt the inventory's tokens, status vocabularies, and action glossary; conflicts with the catalog are resolved in favor of the inventory unless the conflict breaks accessibility or a hard rule (color-only meaning, missing states), which is logged as a deviation to fix.
- The verifier fails a page (severity 2) that introduces a new component where the inventory lists a reusable one, and (severity 2) that uses a label or status representation different from the glossary/vocabulary.
