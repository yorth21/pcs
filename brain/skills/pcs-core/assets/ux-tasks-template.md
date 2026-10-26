# PCD Task Tracker — `<feature-name>`

- **Feature Slug**: `<feature-name>`
- **Spec Locator**: `pcs/specs/<feature-name>.md`
- **Engram Topic Keys**: `pcs/<feature-name>/tasks`, `pcs/<feature-name>/apply`
- **Active Stack Adapter**: `pcs-adapter-react-shadcn`
- **Delivery Strategy**: `ask-on-risk` (400-line review budget heuristic per slice)

---

## Authorized Scope & Promotion Check

- **Feature Directory**: `src/features/<feature>/<subfeature>/`
- **Cross-Cutting Promotions (`>= 2` features required)**: `<None | List exact files in src/components/{ui,form,data-table,layout}/>`
- **Prohibitions**: Zero custom wrappers around `src/components/ui/*`; zero Atomic Design suffixes (`Atom`, `Molecule`, `Organism`, `Template`).

---

## Work Units Checklist

- [ ] **WU-1: Schemas, Types & Query/Mutation Hooks**
  - **Allowed Edit Surfaces**:
    - `src/features/<feature>/<subfeature>/types/<feature>.types.ts`
    - `src/features/<feature>/<subfeature>/schemas/<feature>.schema.ts`
    - `src/features/<feature>/<subfeature>/hooks/use-<feature>.ts`
  - **Requirements**:
    - Define OOUX domain interfaces matching Plane 2 ORCA attributes.
    - Define `zod` validation schemas with actionable human-readable messages.
    - Wire `@tanstack/react-query` hooks with cache invalidation and optimistic updates where specified.
  - **Verification Evidence**: `<Pending | tsc --noEmit passed>`

- [ ] **WU-2: Collection Surface, Metric Strip & 5 Mandatory Negative States**
  - **Allowed Edit Surfaces**:
    - `src/features/<feature>/<subfeature>/components/<feature>-columns.tsx`
    - `src/features/<feature>/<subfeature>/components/<feature>-skeleton.tsx`
    - `src/features/<feature>/<subfeature>/components/<feature>-table.tsx`
    - `src/features/<feature>/<subfeature>/index.tsx`
  - **Requirements**:
    - Implement `DataTable` columns (`<= 6–7` visible columns, `tabular-nums` on numeric/date/ID cells, single row `<DropdownMenu>` with destructive item separated).
    - Implement distinct **Zero-Data State** (`Empty` with onboarding CTA) vs. **No-Results State** (filtered empty state with `"Clear filters"` action).
    - Implement geometry-matched **Contextual Loading (`Skeleton`)** and section-scoped **Partial Error (`Alert` + Retry)**.
  - **Verification Evidence**: `<Pending | States verified>`

- [ ] **WU-3: Contextual Container Flows (`Dialog` / `Sheet` / `AlertDialog`)**
  - **Allowed Edit Surfaces**:
    - `src/features/<feature>/<subfeature>/components/<feature>-sheet.tsx`
    - `src/features/<feature>/<subfeature>/components/<feature>-delete-dialog.tsx`
  - **Requirements**:
    - Implement selected container from Plane 4 (`Sheet` or `Dialog`) using vanilla `shadcn/ui` primitives + `react-hook-form` + `Field` / `FieldError` (`aria-invalid`).
    - Implement dirty-close protection when `formState.isDirty` is true.
    - Implement `<AlertDialog>` for High-Risk destructive actions with explicit consequence copy and verb-specific CTA.
  - **Verification Evidence**: `<Pending | Keyboard & form states verified>`

- [ ] **WU-4: Heuristic, Accessibility & Playwright Verification (`pcs-verifier`)**
  - **Allowed Edit Surfaces**:
    - `pcs/specs/<feature-name>.md`
    - `pcs/tasks/<feature-name>.md`
  - **Requirements**:
    - Score all 10 Usability Heuristics (`0–4` scale; confirm `0` findings of Severity `3` or `4`).
    - Execute 4-Question Cognitive Walkthrough across all steps.
    - Verify WCAG 2.2 AA contrast, `:focus-visible` rings, `aria-label` on icon buttons, and Playwright desktop/mobile rendering.
  - **Verification Evidence**: `<Pending | Verified by pcs-verifier>`
