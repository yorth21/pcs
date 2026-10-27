# Planes 3 & 4 — Structure and Skeleton Rules

This reference defines the technology-agnostic rules for **Plane 3 (Structure)** and **Plane 4 (Skeleton)**. The artifact of both planes is the **Page Composition Spec** of `pcs-saas` (`references/04-template-decision-tree.md` §4): template, region -> organism tree, data contract, state matrix, actions and friction, inventory reuse, responsive notes, and acceptance checks. This file explains the rules the spec applies; it never duplicates the catalog.

---

## 1. Plane 3: Structure (Information Architecture & State-Complete Flows)

### 1.1 Navigation Depth

1. **Maximum depth of 2 levels**: level 1 is the application navigation (sidebar or top navigation); level 2 is within-page view switching (tabs or breadcrumb).
2. Never nest tabs inside tabs, never add a second vertical navigation inside a page that already has the primary one, and never require more than 2 clicks from the shell to any core workspace.

### 1.2 URL as Authoritative View State

Search query, facet filters, sort, page and page size, active tab, and deep-linked record inspection SHOULD be serialized in URL search parameters so Back/Forward, bookmarks, and shared links restore the exact view. The Data Contract of the Page Composition Spec lists them per region.

### 1.3 The 5 Mandatory Negative States

A flow that documents only the happy path is incomplete. The state matrix of `pcs-saas` (`S1`–`S7`) is the required superset; these five are the minimum every data screen MUST cover:

| # | State | `pcs-saas` Code | Trigger | Contract |
|---|-------|-----------------|---------|----------|
| 1 | **Zero-Data** (first use) | `S2` | Collection total is `0` before any filter | Explains what lives here and why, with one primary create action. Never a bare table header over a blank area. |
| 2 | **No-Results** (filtered empty) | `S3` | Total `> 0`, active query or filters match `0` | Distinct from Zero-Data: filter bar stays visible, names the query or filters, offers "Clear filters". Never offers "Create first". |
| 3 | **Contextual Loading** | `S1` | Initial fetch or refetch in flight | Skeletons that match the final geometry (no layout shift); no full-page spinner after the shell renders. Mutations over 400 ms disable the trigger and show progress inside it. |
| 4 | **Partial / Network Error** | `S4` | A query or mutation fails | Error scoped to the failing region with a plain-language cause and "Retry"; shell, header, and sibling regions stay usable. |
| 5 | **Inline Validation Error** | form fields | A field violates a rule on blur or submit | Message adjacent to the field, programmatically linked to it and marked invalid; typed values preserved; focus moves to the first invalid field on submit. |

`S5` (no permission), `S6` (partial or degraded), and `S7` (success feedback) complete the matrix.

---

## 2. Plane 4: Skeleton (Containers, Friction, Layout)

### 2.1 Context-Dependent Container Matrix

Never choose a container by field count alone. Evaluate **context dependency** (does the user need to read the background while acting?) and **cognitive load** first. The `pcs-saas` container rules `K01`–`K06` apply this matrix deterministically.

| Container | Context Dependency | Typical Scope | Select When | Forbidden |
|-----------|--------------------|---------------|-------------|-----------|
| **Inline / popover** | Maximum (in place) | 1–2 attributes | Rapid scanning or triage in a list | Multi-field creation in a popover |
| **Modal dialog** | Zero (background inert) | Short atomic task, `<= 6` fields, single step | Focused create or confirm with no need to see the background | Dialog over dialog; long scrolling forms or tabs in a dialog |
| **Side sheet** (full-screen on mobile) | High (list stays visible, scroll preserved) | Contextual create, edit, or inspection, `<= 12` fields | Iterating through records without losing scroll, filters, or position | A wide multi-column table inside a sheet |
| **Full page / split workbench** | Dedicated workspace | `> 12` fields, `>= 3` sections, complex objects, side-by-side comparison, bookmarkable destinations | Deep authoring, multi-section settings, comparison | Forcing a long configuration into an overlay |

### 2.2 Risk-Proportional Friction Matrix

Friction scales with irreversibility and blast radius; the `pcs-saas` friction rules `R01`–`R05` apply it.

| Risk Tier | Examples | Pattern |
|-----------|----------|---------|
| **A — Reversible** | Archive, toggle status, change a filter, edit a non-critical attribute | No confirmation. Execute immediately (optimistic where safe) and offer "Undo" in a non-blocking notification for 5–8 seconds when the item leaves the view. |
| **B — Unsaved work loss** | Escape, backdrop click, or navigation away from a modified form | Dirty-close guard: close immediately when clean; ask to discard or keep editing when modified. |
| **C — Destructive or irreversible** | Permanent delete, revoke access, void a transaction, bulk irreversible change | Confirmation dialog naming the exact target, the concrete consequence, and a verb-specific destructive button ("Delete invoice #1042") next to "Cancel". High blast radius requires typing the name or count. Money movement or external send shows a review summary before commit. |

### 2.3 Layout Rules the Composer Applies

- One primary action per region; secondary actions step down in emphasis; row actions consolidate in one overflow menu with at most 1 inline quick action.
- Default visible table columns `<= 7`, identity first; secondary attributes move to the detail view or column visibility.
- Form fields grouped in sections of 3–7 fields; one column by default, two columns only for tightly coupled short pairs.
- Conventional positions: search at the top-left of the filter bar, primary create action at the top-right of the page header, "Cancel" before the confirming action in footers, Escape closes overlays.
- Smart defaults (current date, current organization, default page size, default status) keep complexity in the system.

These rules are written into the region tree and actions table of the Page Composition Spec. Compliance is not declared in the spec: the verifier measures it on the rendered UI (`references/04-heuristic-and-walkthrough-verification.md` §5).

### 2.4 Wireframes

The region -> organism tree of the Page Composition Spec is mandatory. A compact structural wireframe MAY accompany it to show placement; a wireframe alone never satisfies Plane 4.

```text
+--------------------------------------------------------------+
| page-header: title + description              [primary CTA]  |
+--------------------------------------------------------------+
| filter-bar: search | 1-3 facet filters | reset | view options |
+--------------------------------------------------------------+
| data-table (identity first, <= 7 columns, row overflow menu) |
+--------------------------------------------------------------+
| pagination: range of total                     prev | next   |
+--------------------------------------------------------------+
```
