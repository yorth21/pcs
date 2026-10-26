# Planes 3 & 4 — Structure and Skeleton Specification

This reference defines the mandatory engineering rules for **Plane 3 (Structure)** and **Plane 4 (Skeleton)** in Product-Centered Development (PCD). These rules govern Information Architecture, State-Complete User Flows, Container Selection, Risk-Proportional Friction, Empirical Cognitive Laws, and Structural Wireframing.

---

## 1. Plane 3: Structure (Information Architecture & State-Complete Flows)

### 1.1 Information Architecture & Navigation Depth Rules

1. **Maximum Navigation Depth (<= 2 Levels)**:
   - Level 1: Primary application navigation (`Sidebar` with grouped sections or top-level navigation bar).
   - Level 2: Within-page view switching (`Tabs` or `Breadcrumb` trail).
   - **Prohibition**: Never nest `Tabs` inside `Tabs`, never place a secondary vertical sidebar inside a page that already has a primary sidebar, and never require more than 2 clicks to reach any core workspace from the root layout.
2. **URL as Authoritative View State**:
   - Every filter query (`?q=`), faceted status selection (`?status=`), pagination cursor (`?page=`, `?pageSize=`), sort order (`?sort=`), active tab (`?tab=`), and deep-linked record inspection (`?inspectId=`) SHOULD be serializable in URL search parameters so browser Back/Forward, bookmarking, and shared links restore exact workspace context.

### 1.2 The 5 Mandatory Negative States (State-Complete User Flow)

A user flow specification that only documents the Happy Path is **incomplete and blocked from Plane 5**. Every data-driven screen or component MUST explicitly define the Happy Path plus all **5 Mandatory Negative States**:

| # | State Name | Trigger Condition | Mandatory UX & Structural Contract |
|---|------------|-------------------|------------------------------------|
| **1** | **Zero-Data State** *(First-Use / Empty Collection)* | The underlying collection has `0` total records in the database/store before any user filter is applied. | Render a dedicated `Empty` container explaining *what* lives here, *why* it matters, and providing **1 primary onboarding CTA** (e.g., `"+ Create first procedure"`). Never show a bare table header over a blank white box. |
| **2** | **No-Results State** *(Filtered / Searched Empty)* | The underlying collection has records (`totalCount > 0`), but the active search query or faceted filters match `0` rows. | **MUST be visually and logically distinct from Zero-Data.** Keep the `DataTableToolbar` visible, state which query/filter produced zero matches, and provide an immediate **`"Clear filters"` / `"Reset search"`** secondary button. Never tell the user to "Create your first record" when records merely failed to match a filter. |
| **3** | **Contextual Loading State** | Initial data fetch or background refetch is in progress. | For initial load, render geometry-matching `Skeleton` placeholders that mirror the exact header, toolbar, and row heights (`CLS = 0`). Never block a whole page with a centered full-screen spinner when only one table or card is loading. For > 400ms mutations, disable the trigger and show an inline spinner inside the active `Button`. |
| **4** | **Partial / Network Error State** | An API query or mutation fails due to network timeout, 4xx/5xx response, or permission denial. | Isolate the failure to the affected section using an inline `Alert` (`variant="destructive"`) or section boundary with a human-readable explanation and a **`"Retry"`** button. Never crash the entire application shell when a single widget or table fails. |
| **5** | **Inline Validation Error State** | User submits or blurs a form field that violates schema constraints (`Zod` / business rules). | Render field-adjacent `<FieldError>` directly below the offending input, set `aria-invalid="true"` and `aria-describedby`, preserve all user-typed values, and focus the first invalid field on submit. Never clear a form on error or rely solely on a transient toast for field validation errors. |

---

## 2. Plane 4: Skeleton (Containers, Friction, Cognitive Laws & Wireframes)

### 2.1 Context-Dependent Container Decision Matrix

**Core Engineering Rule:** Never select a `Dialog`, `Sheet`, or `Page` based solely on input field count. Always evaluate **Context Dependency** (Does the operator need to read or compare background screen data while completing the task?) and **Cognitive Load** first.

| Container Type | Context Dependency | Typical Complexity / Scope | When to Select | Anti-Patterns (Strictly Forbidden) |
|----------------|--------------------|----------------------------|----------------|------------------------------------|
| **Inline Edit / Popover / Command** | **Maximum** (100% in-place) | 1–2 attributes (status change, assignee picker, quick rename, tag selection) | Rapid scanning or triage in a table/list where opening an overlay breaks visual rhythm. | Multi-field creation forms inside a tiny `Popover`. |
| **Modal `Dialog`** | **Zero** (Background is dimmed & inert) | Short, self-contained atomic task (typically 1–4 fields, single-step) | Quick creation or focused confirmation where the user needs **zero reference** to occluded background rows. | Stacking a `Dialog` on top of another `Dialog`; placing long scrollable forms (> 5 fields) or multi-tab views inside a `Dialog`. |
| **Side `Sheet`** *(or bottom `Drawer` on mobile)* | **High** (Background table/list remains partially visible and scroll position is preserved) | Contextual creation, editing, or record inspection (typically 4–12 fields, audit log, or detail summary) | Operator is iterating through a list/table and needs to inspect or edit a record without losing scroll offset, active filters, or spatial context. | Putting a full 10-column data table inside a narrow `Sheet`. |
| **Full Page / Split Workbench** (`ResizablePanelGroup`) | **Dedicated Workspace** | High complexity (12+ fields across logical sections, multi-object composition, document preview + form side-by-side) | Deep authoring, multi-section settings, or side-by-side comparison (e.g., PDF/invoice on left, reconciliation form on right). | Forcing a complex 15-field configuration flow into a cramped modal overlay. |

### 2.2 Risk-Proportional Friction Matrix

Friction must scale strictly with **irreversibility and blast radius**:

| Action Risk Tier | Examples | Required Interaction Pattern |
|------------------|----------|------------------------------|
| **Tier A — Low Risk / Easily Reversible** | Archiving an item, toggling active/inactive status, changing a filter, updating a non-critical attribute | **Zero confirmation dialog.** Execute immediately (optimistic UI update where safe) and display a non-blocking `sonner` toast with an explicit **`Undo`** button for 5–8 seconds. |
| **Tier B — Medium Risk / Unsaved Work Loss** | Clicking outside a `Sheet` or `Dialog` or pressing `Escape` after modifying form inputs (`isDirty === true`) | **Dirty-Close Guard.** If `isDirty` is `false`, close immediately. If `isDirty` is `true`, intercept dismissal and prompt to discard changes or continue editing. |
| **Tier C — High Risk / Destructive or Irreversible** | Permanently deleting a record, revoking access, voiding a financial transaction, executing a bulk irreversible mutation | **Mandatory `<AlertDialog>`.** Must state: (1) the exact entity name/ID, (2) the concrete irreversible consequence, and (3) a verb-specific destructive button (`variant="destructive"`, e.g., `"Delete procedure"`), paired with `"Cancel"`. For high-blast-radius bulk deletions, require typing the resource name or count to enable the destructive button. |

### 2.3 Empirical Cognitive & Perceptual Laws Checklist

Every Plane 4 Skeleton specification MUST verify compliance with these seven laws:

1. **Fitts's Law (Target Acquisition)**:
   - Interactive controls must have a minimum clickable/tappable area of **`36x36px` on desktop** (`h-9`) and **`44x44px` on touch/mobile** (`min-h-11 min-w-11`).
   - Place the primary submit CTA at the natural terminal anchor of the container (bottom-right of `DialogFooter`/`SheetFooter`/`CardFooter`, or top-right of `PageHeader`) with at least `8px` (`gap-2`) separation from `"Cancel"` or destructive buttons.
2. **Hick's Law (Decision Latency)**:
   - Expose at most **5 top-level actions or navigation items** in a single toolbar or row.
   - In table rows, show at most **1 inline quick action** (or `0`) and consolidate secondary/destructive row actions inside a single `<DropdownMenu>` triggered by a `MoreHorizontal` icon button with `aria-label`.
3. **Miller's Law (Working Memory Chunking)**:
   - Group related form fields into visual chunks of **3–7 fields** separated by `FieldSet` / `Card` sections or subtle dividers.
   - Limit data tables to **5–7 visible columns by default**; move secondary attributes to the row inspection `Sheet` or optional column visibility toggles.
4. **Jakob's Law (Mental Model Consistency)**:
   - Preserve standard web and SaaS conventions: search input at top-left of toolbars, primary creation CTA at top-right of page header, `"Cancel"` to the left of `"Save"` in footers, and Escape to close overlays.
5. **Tesler's Law (Conservation of Complexity)**:
   - Shift complexity from the user to the system: pre-populate today's date, current user's organization, sensible default pagination (`10` or `25` rows), and automatic formatting rather than forcing manual entry.
6. **Doherty Threshold (< 400ms System Response)**:
   - Every user interaction must produce visual acknowledgment within **100–400ms** (instant hover/active states, optimistic toggles, or immediate skeleton/button-spinner transitions).
7. **Gestalt Principles (Proximity, Similarity, Common Region, Figure-Ground)**:
   - **Proximity**: Label-to-input gap (`gap-1.5` / `6px`) MUST be strictly smaller than input-to-next-field gap (`gap-4` / `16px`), which MUST be smaller than section-to-section gap (`gap-6` or `gap-8` / `24–32px`).
   - **Common Region**: Group related metrics, filters, or form sections inside a single shared border (`Card` or `bg-muted/30` container) rather than wrapping every single field in its own box.

### 2.4 Structural ASCII Wireframe Notation

Before handing off to `pcs-builder`, represent the Skeleton layout in `pcs/specs/<feature-name>.md` using a clean structural wireframe that maps regions to exact component primitives and grid spans:

```text
+-----------------------------------------------------------------------------------+
| PageHeader: Title + Description (left)                  [+ Primary CTA] (right)   |
+-----------------------------------------------------------------------------------+
| MetricStrip (Optional, max 3-4 clickable filter cards, grid-cols-1 md:grid-cols-3)|
| [ Total Active (142) ]      [ Needs Review (7) ]        [ Overdue ($12,400.00) ]  |
+-----------------------------------------------------------------------------------+
| DataTableToolbar: [Search Input (w-72)] [Status Filter] [Reset]    [Column View]  |
+-----------------------------------------------------------------------------------+
| DataTable (5-7 columns max, tabular-nums on numeric/date/ID, 1 row action menu)   |
| CODE      | NAME                  | STATUS      | AMOUNT (right) | ACTIONS        |
| PROC-101  | General Consultation  | [Active]    |        $120.00 | [...]          |
+-----------------------------------------------------------------------------------+
| DataTablePagination: Showing 1-10 of 142 rows           [< Prev] [1] [2] [Next >] |
+-----------------------------------------------------------------------------------+
```
