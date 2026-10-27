# 04 — Template Decision Tree and Page Composition Spec

This reference turns a request into a template ID, a container, and a friction level without judgment calls, and defines the Page Composition Spec the composer MUST output. It applies the `pcs-core` container decision matrix and risk-proportional friction matrix; where both apply, this tree decides and `pcs-core` explains why.

---

## 1. Classification Inputs

Derive every input from the JTBD and the OOUX ORCA matrix (`pcs-core` `references/01-planes-strategy-and-scope.md`). Record the value and its evidence in the spec; if unknown, use the stated default and log a Declared Assumption.

| Input | Values | How to decide | Default |
|-------|--------|---------------|---------|
| `J` Job type | `monitor`, `find`, `process`, `inspect`, `create`, `edit`, `configure`, `transfer`, `authenticate`, `onboard`, `recover` | Main verb of the JTBD "I want to …" clause | — (required) |
| `N` Object cardinality | `one`, `few` (`<= 20`), `many` (`> 20`) | Expected records per user/workspace at 12 months | `many` |
| `C` Object complexity | `simple` (`<= 6` attributes, no sub-collections), `medium` (`7–12` attributes, no sub-collections), `complex` (`> 12` attributes or `>= 1` sub-collection or own history) | Count ORCA attributes shown to this role | `medium` |
| `F` Frequency | `daily` (several times per day), `weekly`, `rare` (monthly or one-time) | Role profile frequency | `weekly` |
| `R` Action risk | `A` reversible, `B` unsaved-work loss, `C` destructive/irreversible/affects others or money | Worst CTA in scope, per `pcs-core` friction matrix | `A` |
| `X` Comparison need | `yes` if the user compares `>= 4` attributes across records or reads the list while acting; `no` otherwise | JTBD and HTA steps | `no` |
| `V` Visual recognition | `yes` if records are recognized by image, preview, or icon more than by text | Object attributes | `no` |
| `S` Sequential dependency | `yes` if later inputs depend on earlier answers or a preview must precede commit | Form analysis | `no` |

## 2. Decision Tree

Evaluate top to bottom; the first matching rule wins. Record the rule ID as the rationale.

### 2.1 Template

| Rule | Condition | Template |
|------|-----------|----------|
| `T01` | `J = authenticate` | `tpl-auth` |
| `T02` | `J = recover` (user hit a missing, forbidden, or failed resource) | `tpl-system-page` |
| `T03` | `J = onboard` | `tpl-onboarding` (`welcome-steps` if before first use, `checklist` if in-product) |
| `T04` | `J = transfer` (bulk in/out) | `tpl-import-export` |
| `T05` | `J = configure` and subject is plan, payment, or usage | `tpl-billing` |
| `T06` | `J = configure` | `tpl-settings` |
| `T07` | `J = monitor` and subject spans `>= 2` object types or health over time | `tpl-dashboard` |
| `T08` | `J = monitor` and subject is "who did what" history | `tpl-activity-log` |
| `T09` | `J = process` (each item needs a decision by this user) | `tpl-inbox-queue` |
| `T10` | `J = find` across object types (global search) | `tpl-search-results` |
| `T11` | `J ∈ {create, edit}` and `S = yes` | `tpl-wizard` |
| `T12` | `J ∈ {create, edit}` and (`C = complex` or fields `> 12` or sections `>= 3`) | `tpl-form-long` |
| `T13` | `J ∈ {create, edit}` | `tpl-form-short` (container per §2.2) |
| `T14` | `J = inspect` and `C = complex` | `tpl-record-detail` |
| `T15` | `J ∈ {find, inspect}`, `N ∈ {few, many}`, `F = daily`, `X = no`, and records are opened in sequence | `tpl-list-detail` |
| `T16` | `J = find`, `V = yes`, `X = no`, and expected records `<= 200` | `tpl-index-cards` |
| `T17` | `J ∈ {find, monitor}` with one object type | `tpl-index-table` |
| `T18` | `J = inspect` and `C ∈ {simple, medium}` | No page: `org-detail-panel` (sheet) from the index template of that object |
| `T19` | `N = one` (singleton object, e.g., workspace profile) | `tpl-settings` if configurable, else `tpl-record-detail` |

Every page also sits inside `tpl-app-shell` unless the template is `tpl-auth`, `tpl-onboarding` `welcome-steps`, or `tpl-system-page` `fatal`.

### 2.2 Container (for create, edit, inspect, and secondary tasks)

| Rule | Condition | Container |
|------|-----------|-----------|
| `K01` | `1–2` attributes, the user is scanning a list (`F = daily`) | `inline` (inline edit, popover, or select in place) |
| `K02` | `<= 6` fields, no need to see the background, single step | `dialog` |
| `K03` | `<= 12` fields, or inspection with `C ∈ {simple, medium}`, and the user needs the list context (`X = yes` or iterating records) | `sheet` |
| `K04` | `> 12` fields, `>= 3` sections, rich content, `C = complex`, or a destination users bookmark | `page` |
| `K05` | `S = yes` | `page` (wizard); `dialog` only for 2–3 step flows with `<= 4` fields per step |
| `K06` | Mobile (`< 640px`) | `dialog` and `sheet` render full-screen; `inline` stays inline when the target is `>= 44px` |

### 2.3 Friction

| Rule | Condition | Friction |
|------|-----------|----------|
| `R01` | `R = A` | None; optimistic update; undo toast if it removes something from view |
| `R02` | `R = B` | Dirty-close guard on dialog/sheet/page; autosave drafts when the form takes `> 2` minutes |
| `R03` | `R = C`, single record | `org-destructive-confirm` naming target and consequence |
| `R04` | `R = C`, `>= 10` records, workspace/account scope, or other users' data | `org-destructive-confirm` + `mol-confirm-input` |
| `R05` | Money movement or external send (invoice, payment, email to customers) | Review summary showing the exact amount/recipients before commit; no optimistic state |

### 2.4 Density

`F = daily` and (`N = many` or `J = process`) → `compact`; otherwise `comfortable`.

## 3. Worked Examples

### 3.1 "Users need to manage the customers of their workspace"

- Inputs: `J = find` (manage = find + inspect + create/edit), `N = many`, `C = complex` (14 attributes, sub-collections invoices and contacts), `F = daily`, `R = C` (delete customer), `X = yes`, `V = no`, `S = no`.
- Template: `T17` → `tpl-index-table` (customers list). Inspection: `T14` → `tpl-record-detail` (customer page with tabs Overview, Invoices, Contacts, Activity). Create: `T13` with 6 required fields → `tpl-form-short`, `K02` → `dialog`. Edit: on the record page, inline per field (`K01`) for single attributes.
- Friction: delete `R03` → `org-destructive-confirm` naming the customer and "{n} invoices remain but lose their customer link". Archive `R01` → undo toast.
- Density: `compact` table.

### 3.2 "Managers approve employee expense reports"

- Inputs: `J = process`, `N = many`, `C = medium`, `F = daily`, `R = A` for approve (reversible within 24h by policy), `R = C` for reject with notification, `X = no`.
- Template: `T09` → `tpl-inbox-queue` with tabs Pending/Approved/Rejected; detail pane shows receipt preview and line items.
- Friction: approve `R01` → undo toast + auto-advance; reject → required reason field in the decision bar (the reason is the friction, no extra dialog).
- Density: `compact`.

### 3.3 "Admins configure single sign-on for the workspace"

- Inputs: `J = configure`, `N = one`, `C = medium`, `F = rare`, `R = C` (misconfiguration locks users out), `S = yes` (provider choice determines fields; a test must precede enabling).
- Template: `T06` → `tpl-settings` page "Security" with card "Single sign-on". Setup action opens `T11` → `tpl-wizard` (3 steps: Choose provider → Enter details → Test connection) as a page (`K05`).
- Friction: enabling enforcement `R04` → confirmation with type-to-confirm stating "Members without SSO accounts will be signed out".
- Density: `comfortable`.

### 3.4 "Show the team how the account is doing this month"

- Inputs: `J = monitor`, spans projects, invoices, and usage (`>= 2` types), `F = daily`, `R = A`, read-only.
- Template: `T07` → `tpl-dashboard`: `org-kpi-row` (Revenue, Open invoices, Overdue amount, Active projects; each links to a filtered index), attention list "Overdue invoices" (5 rows), 2 `org-chart-panel` (Revenue by week; Invoices by status), `org-activity-feed` (last 10).
- Friction: none. Density: `comfortable` cards, `compact` lists.

## 4. Page Composition Spec Format (MANDATORY)

The composer writes one block per page into `pcs/specs/<feature-name>.md` under `## Page Composition`. Use this exact structure and headings; every placeholder is required unless marked optional.

```markdown
### Page: {Page name} — `{route}`

#### 1. Template
- **Template**: `tpl-…` (variant: … | none)
- **Shell**: `tpl-app-shell` | minimal | none
- **Decision path**: inputs J=… N=… C=… F=… R=… X=… V=… S=…; rules fired: `T..`, `K..`, `R..`; density: compact | comfortable
- **Rationale**: 1–3 sentences linking the JTBD to the template.

#### 2. Region Tree
1. `{region name}` → `org-…` ({inventory component or "new"})
   - `mol-…`: {purpose / content}
   - `mol-…`: …
2. `{region name}` → `org-…`
   - …

#### 3. Data Contract
| Region | Object | Fields shown (order) | Sort keys (default) | Filter keys | URL params | Source |
|--------|--------|----------------------|---------------------|-------------|------------|--------|
| … | … | … | … | … | … | {endpoint/query or "TBD + assumption ID"} |

#### 4. State Matrix
| Region | S1 Loading | S2 Empty first-use | S3 No-results | S4 Error | S5 No-permission | S6 Partial/degraded | S7 Success feedback |
|--------|-----------|--------------------|---------------|----------|------------------|---------------------|---------------------|
| … | {exact rendering} | {title + action} or n/a (+reason) | … | … | … | … | … |

#### 5. Actions and Friction
| Action (label) | Region | Level (primary/secondary/tertiary/menu) | Container | Risk | Friction rule | Feedback |
|----------------|--------|------------------------------------------|-----------|------|---------------|----------|

#### 6. Inventory Reuse
| Catalog ID | Decision | Project component (path) | Justification (required for "new" or "deviate") |
|------------|----------|--------------------------|--------------------------------------------------|
| `org-…` | reuse / extend / new / deviate | `src/…` or — | … |

#### 7. Responsive Notes
- Desktop (`>= 1024px`): …
- Tablet (`640–1023px`): …
- Mobile (`< 640px`): …

#### 8. Acceptance Checks
1. `AC-…` (from template) — {how to verify: action + expected observation}
2. `AC-PAGE-1` (page-specific) — …
```

### 4.1 Spec Validity Rules

1. Exactly one template ID; the decision path lists the inputs and the first rule that matched.
2. Every region in the template anatomy appears in the region tree or is marked "omitted: {reason}".
3. Every data region has a state matrix row; `n/a` requires a reason ("read-only region: S7 n/a").
4. `S2` and `S3` cells differ for every collection region.
5. Every action has a verb-first label identical to the label used for the same action elsewhere in the inventory.
6. Every organism appears in the inventory reuse table; `new` and `deviate` require a justification naming why no inventory organism fits.
7. Acceptance checks include all template `AC-*` checks plus at least one page-specific check per primary action; each check states an observable result the verifier can test on the rendered UI or by keyboard.
8. No UI library, framework, or component names appear in sections 1–5 and 7–8; only catalog IDs and project paths (section 6).
