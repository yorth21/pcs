# UX & Product Specification — `<feature-name>`

- **Feature Slug**: `<feature-name>`
- **Execution Tier**: `Tier 2 (Bounded Flow)` | `Tier 3 (Full Feature)`
- **Active Stack Adapter**: `pcs-adapter-react-shadcn`
- **Status**: `Draft` | `Approved for Build` | `Verified`

---

## 1. Plane 1 — Product Strategy (`pcs-strategist`)

### 1.1 Operational Role & Environment Profile
- **Primary Operator Role**: `<e.g., Billing Operations Specialist>`
- **Frequency of Use**: `<e.g., High-frequency daily workbench (40+ executions/shift) | Low-frequency setup>`
- **Domain Expertise**: `<e.g., Specialist — requires high information scent and rapid filtering>`
- **Workspace & Viewport Context**: `<e.g., Desktop 1440px primary, 768px tablet secondary; high interruption rate>`
- **Error Consequence**: `<e.g., High — financial ledger impact on destructive actions>`

### 1.2 Jobs-to-be-Done (JTBD)
> **When** `<triggering operational situation>`,
> **I want to** `<core capability/action>`,
> **so I can** `<measurable user and business outcome>`.

- **Functional Outcome**: `<concrete task speedup or step reduction>`
- **Emotional Outcome**: `<confidence, zero anxiety over accidental loss or hidden filters>`
- **Social / Auditability Outcome**: `<clear team visibility and status traceability>`

### 1.3 Declared Assumptions Ledger
| ID | Declared Assumption | Confidence (`High`/`Med`/`Low`) | Risk if Wrong & UX Impact | Validation Method |
|----|---------------------|---------------------------------|---------------------------|-------------------|
| `A-01` | `<assumption statement>` | `High` | `<risk and what container or flow would change>` | `<codebase check, API schema inspection, or user question>` |

### 1.4 Measurable Outcome Metrics
| Metric | Target | Verification Method |
|--------|--------|---------------------|
| **Primary Task KLM Budget** | `<= <N> clicks / <M> view transitions` | HTA/KLM step walkthrough |
| **Context Preservation** | `0` filter/scroll resets across inspections | URL search params + contextual container check |
| **Negative Path Completeness** | `5/5` mandatory negative states with recovery CTAs | Heuristic & Playwright verification |

---

## 2. Plane 2 — Product Scope & OOUX (`pcs-strategist`)

### 2.1 OOUX ORCA Object Matrix
| Object (Noun) | Role (`Primary`/`Secondary`) | Relationships & Cardinality (`1:1`, `1:N`, `M:N`) | Permitted CTAs & Role Permissions | Core Attributes (`T1 Identity`, `T2 State`, `T3 Quantitative`) vs. Metadata (`T4 Detail`) |
|---------------|------------------------------|---------------------------------------------------|-----------------------------------|-------------------------------------------------------------------------------------------|
| `<ObjectName>` | `Primary` | `<e.g., 1:N with LineItem>` | `Create`, `Edit`, `Archive` (`Low Risk`), `Delete` (`High Risk / Admin`) | **Core — T1**: `code`, `name`<br/>**Core — T2**: `status`<br/>**Core — T3**: `amount`, `updatedAt` (`tabular-nums`)<br/>**Metadata — T4**: `notes`, `auditTrail` |

### 2.2 HTA / KLM Task Step Budget & KISS Pruning
| User Sub-Goal | Unpruned Baseline (Clicks / Keystrokes / Views) | KISS-Pruned Budget (Clicks / Keystrokes / Views) | Chosen Interaction Path | Pruned Bloat (What We Refused to Build) |
|---------------|-------------------------------------------------|--------------------------------------------------|-------------------------|-----------------------------------------|
| `<Sub-goal 1>` | `<e.g., 6 clicks, 2 page loads, 1 confirm modal>` | `<e.g., <= 3 clicks + typing, 0 page loads>` | `<e.g., Direct Sheet form with smart defaults>` | `<e.g., Eliminated intermediate confirmation modal>` |

### 2.3 User Story Mapping Slice
- **Walking Skeleton (MVP — Must Build Now)**:
  - `<End-to-end core capability + all 5 Mandatory Negative States>`
- **Deferred Enhancements (Out of Current Slice)**:
  - `<Explicitly deferred secondary features>`

---

## 3. Plane 3 — Structure: IA & State-Complete Flows (`pcs-architect`)

### 3.1 Information Architecture & URL State Contract
- **Navigation Path (<= 2 levels)**: `<e.g., Sidebar: Operations -> Procedures>`
- **URL Search Parameters**:
  - `?q=` (string search filter)
  - `?status=` (faceted status filter)
  - `?page=` & `?pageSize=` (pagination state)
  - `?inspectId=` (optional deep-linked record in `Sheet`)

### 3.2 Happy Path & 5 Mandatory Negative States
| Flow / State | Trigger Condition | UI Component & Copy Contract | Recovery / Primary CTA |
|--------------|-------------------|------------------------------|------------------------|
| **Happy Path** | `totalCount > 0` and `filteredCount > 0` | `DataTable` + `DataTableToolbar` + `DataTablePagination` | Primary header CTA + row action menu |
| **1. Zero-Data State** | `totalCount === 0` (no records exist yet) | `Empty` container with icon, value proposition title, and helper text | `"+ Create first <entity>"` (`Button` default) |
| **2. No-Results State** | `totalCount > 0` and `filteredCount === 0` | Filtered empty banner inside table body showing active query/filters | `"Clear filters"` (`Button` outline/secondary) |
| **3. Contextual Loading** | `isLoading === true` or `isSubmitting === true` | Geometry-matched `Skeleton` rows (`CLS = 0`); inline spinner on submit button | Automatic transition on resolve |
| **4. Partial / Network Error** | `isError === true` on query or mutation | Section-scoped `Alert` (`variant="destructive"`) with human-readable cause | `"Retry"` button (`refetch()`) |
| **5. Inline Validation Error** | Schema validation failure (`zodResolver`) | Field-adjacent `FieldError` below input + `aria-invalid="true"` | Focus first invalid field; preserve typed input |

---

## 4. Plane 4 — Skeleton: Containers, Friction & Wireframe (`pcs-architect`)

### 4.1 Context-Dependent Container Decision Matrix
| Workflow / Action | Context Dependency (`High`/`Zero`) | Cognitive Load | Selected Container (`Inline`/`Dialog`/`Sheet`/`Page`) | Engineering Rationale |
|-------------------|------------------------------------|----------------|-------------------------------------------------------|-----------------------|
| `<e.g., Edit Record>` | `High` (needs table scroll & filter context) | `Medium (6 fields)` | `Sheet` (`side="right"`, `sm:max-w-lg`) | Preserves background table visibility and URL filter state. |
| `<e.g., Quick Create>` | `Zero` (atomic self-contained entry) | `Low (3 fields)` | `Dialog` (`sm:max-w-md`) | Fast focused entry requiring no reference to background rows. |

### 4.2 Risk-Proportional Friction Matrix
| Action | Risk Tier (`Low`/`Medium`/`High`) | Friction Pattern | Exact UI Copy / Mechanism |
|--------|-----------------------------------|------------------|---------------------------|
| `<e.g., Toggle Active>` | `Low (Reversible)` | Optimistic update + `sonner` Undo toast | Toast: `"<Entity> archived"` + Action: `"Undo"` |
| `<e.g., Close Dirty Sheet>` | `Medium (Unsaved Edits)` | Dirty-close guard (`formState.isDirty`) | Confirm discard if user clicks outside while `isDirty` |
| `<e.g., Delete Record>` | `High (Destructive)` | `<AlertDialog>` with explicit impact | Title: `"Delete <entityName>?"` + CTA: `"Delete <entity>"` (`variant="destructive"`) |

### 4.3 Empirical Cognitive Laws Verification
- **Fitts's Law**: All interactive targets >= `36px` desktop (`h-9`) / >= `44px` touch; Primary CTA anchored top-right of page and bottom-right of footers.
- **Hick's Law**: Single Primary CTA in header; row actions consolidated into `<DropdownMenu>`.
- **Miller's Law**: Table capped at `<= 6` visible columns; form inputs chunked into logical groups of `<= 5` fields.
- **Jakob's Law**: Standard toolbar search at top-left, primary creation CTA at top-right, `"Cancel"` left of `"Save"`, `Escape` closes overlays.
- **Tesler's Law**: Smart defaults pre-populated (pagination, default status/category, current date/org) so complexity stays in the system.
- **Doherty Threshold**: Instant skeleton/optimistic feedback `< 400ms`.
- **Gestalt Proximity & Common Region**: Field label gap (`gap-1.5`) < field-to-field gap (`gap-4`) < section gap (`gap-6`); related sections grouped in shared `Card` / `FieldSet` borders.

### 4.4 Structural ASCII Wireframe & Component Map
```text
+-----------------------------------------------------------------------------------+
| PageHeader: <Title> + <Description>                     [+ Primary CTA Button]    |
+-----------------------------------------------------------------------------------+
| DataTableToolbar: [Search Input] [Status Filter] [Reset]                          |
+-----------------------------------------------------------------------------------+
| DataTable (Columns: Code | Name | Status Badge | Amount [tabular-nums] | Actions) |
+-----------------------------------------------------------------------------------+
| DataTablePagination                                                               |
+-----------------------------------------------------------------------------------+
```

---

## 5. Plane 5 — Surface & Token Rules (`pcs-builder`)
- **60-30-10 Color Budget**: `60%` `background`/`card`, `30%` `muted`/`border`, `10%` `primary` (1 solid primary CTA per viewport) and semantic status badges.
- **Typography**: 3 levels max (`Title` `text-lg/xl font-semibold`, `Body/Cell` `text-sm`, `Metadata` `text-xs text-muted-foreground`).
- **Numeric Discipline**: `tabular-nums` applied to all amounts, dates, counts, and codes.
- **Component Hygiene**: 100% vanilla `shadcn/ui` primitives from `@/components/ui/*` + shared `@/components/{form,data-table,layout}/*`. Zero custom wrapper abstractions; zero Atomic Design suffixes.

---

## 6. Verification & Heuristic Audit (`pcs-verifier`)

### 6.1 10 Usability Heuristics Scorecard (0–4 Scale; 0 Blockers >= 3 Allowed)
| Heuristic | Severity (`0–4`) | Evidence / Observation |
|-----------|------------------|------------------------|
| H1: Visibility of System Status | `0` | `<evidence>` |
| H2: Match Between System and Real World | `0` | `<evidence>` |
| H3: User Control and Freedom | `0` | `<evidence>` |
| H4: Consistency and Standards | `0` | `<evidence>` |
| H5: Error Prevention | `0` | `<evidence>` |
| H6: Recognition Rather Than Recall | `0` | `<evidence>` |
| H7: Flexibility and Efficiency of Use | `0` | `<evidence>` |
| H8: Aesthetic and Minimalist Design | `0` | `<evidence>` |
| H9: Recognize, Diagnose, and Recover from Errors | `0` | `<evidence>` |
| H10: Contextual Help and Documentation | `0` | `<evidence>` |

### 6.2 4-Question Cognitive Walkthrough & Playwright Check
- **Q1 (Intent)** / **Q2 (Visibility)** / **Q3 (Association)** / **Q4 (Feedback)**: `<Pass / Findings>`
- **WCAG 2.2 AA & Keyboard Navigation**: `<Pass / Findings>`
- **Playwright Viewport Audit (`1440x900` & `390x844`)**: `<Pass / Skipped (no running server)>`
