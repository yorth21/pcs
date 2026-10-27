# Usability Heuristics, Cognitive Walkthrough and Accessibility Verification

This reference defines the mandatory verification rubric executed by `pcs-verifier` (and self-checked by `pcs-builder`) before any PCD feature is marked `done`, and by `pcs-auditor` for read-only baseline audits of existing UI without a spec.

---

## 1. The 10 Usability Heuristics Rubric for Product & SaaS Interfaces

Score every heuristic on the **`0–4` Severity Scale**:
- **`0` — Pass / No Issue**: Full compliance.
- **`1` — Cosmetic**: Minor visual or copy polish; zero impact on task completion speed or accuracy.
- **`2` — Minor Usability Friction**: Noticeable hesitation or suboptimal layout; user recovers without assistance.
- **`3` — Major Usability Blocker (FAIL)**: Missing negative state, context-destroying container, ambiguous error recovery, or broken keyboard flow.
- **`4` — Usability Catastrophe (FAIL)**: Data loss risk, unguarded destructive action, dead-end flow, or inaccessible primary action.

> **Hard Gate**: Any score of **`3` or `4`** blocks task closure and requires an immediate remediation pass by `pcs-builder`.

| # | Heuristic | Concrete SaaS Pass Criteria | Automatic Severity `3–4` Fail Triggers |
|---|-----------|-----------------------------|----------------------------------------|
| **H1** | **Visibility of System Status** | Immediate (< 400ms) visual feedback on every fetch, filter, and mutation: geometry-matched `Skeleton` on load, inline button spinner (`isSubmitting`) during save, visible active filter badges, and confirmation toast on completion. | Clicking `"Save"` or `"Delete"` shows no loading state, allowing double-submission; active filters are hidden with no visual indicator. |
| **H2** | **Match Between System and Real World** | Labels, column headers, and status badges use the operator's domain vocabulary (from Plane 2 OOUX ORCA) rather than database column names, raw enum keys (`STATUS_PENDING_2`), or internal UUIDs. | Exposing raw camelCase keys, technical stack traces, or database error codes in the UI. |
| **H3** | **User Control and Freedom** | Every overlay (`Dialog`, `Sheet`) supports `Escape`, close button, and explicit `"Cancel"`. Dirty forms (`isDirty === true`) warn before discarding edits. Reversible actions provide a toast `"Undo"` action. | Trapping the user in a modal with no cancel path, or silently wiping 10 typed fields on an accidental backdrop click. |
| **H4** | **Consistency and Standards** | Identical domain objects share identical status badge colors and terminology across cards, tables, and sheets. Primary CTA is consistently placed (`top-right` in page header, `bottom-right` in footers). | Placing `"Cancel"` on the right and `"Save"` on the left in one modal and reversing them in another; inventing custom wrappers that deviate from vanilla `shadcn/ui`. |
| **H5** | **Error Prevention** | Destructive/irreversible mutations require `<AlertDialog>` naming the entity and consequence. Invalid state transitions are disabled or constrained using typed selects, date constraints, and input masks. | Executing a permanent delete on a single unguarded row click; allowing form submission while `isSubmitting` is already true. |
| **H6** | **Recognition Rather Than Recall** | Contextual `Sheet` keeps background table rows visible during inspection/edit. Search inputs retain the typed query; faceted filters show active counts and clear labels. | Opening an opaque modal that hides the reference numbers the operator needs to copy/compare from the background table. |
| **H7** | **Flexibility and Efficiency of Use** | Full keyboard operability (`Tab`, `Shift+Tab`, `Enter` to submit, `Escape` to close, arrow keys in menus/command palettes). Table state (`?q=`, `?status=`, `?page=`) lives in URL search params. | Requiring mouse-only hover to reveal critical actions; resetting table pagination and filters to page 1 on every row edit. |
| **H8** | **Aesthetic and Minimalist Design** | 60-30-10 color budget enforced; at least 25% open whitespace; at most 1 solid primary CTA per viewport; max 5–7 default table columns; `tabular-nums` on all numeric/date/ID values. | Multiple competing primary buttons (`variant="default"`) in the same header; crammed 12-column tables causing horizontal scroll clipping. |
| **H9** | **Help Users Recognize, Diagnose, and Recover from Errors** | Validation errors appear inline (`<FieldError>`) directly beneath the invalid field with `aria-invalid="true"`, stating *what* is wrong and *how* to fix it. Partial API errors show a section-scoped `<Alert>` with a `"Retry"` button. | Showing a generic `"Error 400: Bad Request"` toast while clearing the user's form inputs; crashing the whole page when one query fails. |
| **H10** | **Contextual Help and Documentation** | First-use `Zero-Data` empty state explains the feature's purpose and provides the primary onboarding CTA. Non-obvious form inputs include a concise 1-line `<FieldDescription>`. | Rendering a blank white area or an empty table header on first use with no guidance on how to populate data. |

---

## 2. The 4-Question Cognitive Walkthrough Protocol

Walk through every step of the primary user task (and each negative recovery path) and evaluate these four questions:

1. **Q1 — Goal Alignment (Intent)**: Will the operator try to achieve the right effect at this step based on their domain mental model?
2. **Q2 — Control Visibility (Affordance)**: Is the control visibly present and perceivable without hidden hover-only discovery or deep scrolling?
3. **Q3 — Label-to-Goal Association (Signifier)**: Does the button/input label unambiguously match the operator's intended action (e.g., `"Create invoice"` instead of `"Submit"`)?
4. **Q4 — Progress Feedback (Closure)**: After acting, does the interface immediately (< 400ms) confirm that progress was made toward the goal?

Record any `No` answer as a **Severity `2` or `3`** defect and specify the exact component and line requiring correction.

---

## 3. State Completeness & Accessibility Audit Checklist (WCAG 2.2 AA)

Verify every item below in static source code and (when available) via live Playwright inspection:

- [ ] **Distinct Empty States**: `Zero-Data` (`totalRecords === 0`) and `No-Results` (`filteredRecords === 0` with active query/filter) are implemented as separate branches with distinct copy and CTAs.
- [ ] **Contextual Skeletons**: Loading states match the geometry of the loaded surface (`CLS = 0`).
- [ ] **Partial Error Boundary**: Failed queries render a localized error state with a working retry trigger.
- [ ] **Accessible Form Wiring**: Every input is paired with a programmatic `<Label>` / `<FieldLabel>`, sets `aria-invalid={!!error}`, and links error text via `aria-describedby`.
- [ ] **Icon-Only Button Labels**: Every icon-only button (`size="icon"`, e.g., row actions `MoreHorizontal`, close `X`) includes an explicit `aria-label` or `<span className="sr-only">`.
- [ ] **Keyboard & Focus Management**: All interactive elements display a visible `:focus-visible` ring (`outline-ring/50 ring-[3px]`), overlays trap focus properly, and closing an overlay returns focus to its trigger.
- [ ] **Responsive Viewport Integrity**: Audited at Desktop (`1440x900`) and Mobile (`390x844`) viewports with zero unintended horizontal overflow (`overflow-x-auto` applied to tables).
- [ ] **Safe Playwright Server Lifecycle**: When running live browser checks, reuse an already-running dev server without stopping it; if starting a temporary server, register an `EXIT` trap (`trap 'kill $SERVER_PID 2>/dev/null || true' EXIT`) so zero orphan processes remain after verification. When no server is requested or startable, complete the full static JSX/TSX audit and document that live browser execution was skipped.
