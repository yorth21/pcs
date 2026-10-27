# UX & Product Specification — `<feature-name>`

- **Feature Slug**: `<feature-name>`
- **Flow Kind**: `t2` | `t3`
- **Active Stack Adapter**: `<adapter skill name>`
- **UI Inventory**: `pcs/design/inventory.md` (refreshed `<date>`)
- **Status**: `Draft` | `Composed` | `Built` | `Verified`

---

## 1. Plane 1 — Strategy (`pcs-strategist`)

### 1.1 Operational Role & Environment
- **Role**: `<e.g., Billing operations specialist>`
- **Domain expertise**: `<novice | specialist>` — `<page impact>`
- **Frequency (`F`)**: `<daily | weekly | rare>` — `<evidence>`
- **Environment & primary viewport**: `<e.g., desktop 1440px, frequent interruptions>`
- **Error consequence**: `<e.g., high — ledger impact on void>`

### 1.2 Jobs
- **Primary JTBD** (`J = <job type>`):
  > **When** `<situation>`, **I want to** `<action>`, **so I can** `<outcome>`.
- **Secondary jobs**:
  - `<job>` (`J = <job type>`) — `<adds region/action to this page | separate page>`

---

## 2. Plane 2 — Scope (`pcs-strategist`)

### 2.1 OOUX ORCA Matrix
| Object | Role | Relationships (cardinality) | Actions (role, risk `A`/`B`/`C`) | Attributes (T1 / T2 / T3 / T4) |
|--------|------|-----------------------------|----------------------------------|--------------------------------|
| `<Object>` | Primary | `1:N <Related>` | Create (member, `A`); Archive (member, `A`); Delete (admin, `C`) | T1 `code`, `name`; T2 `status`; T3 `amount`, `updatedAt`; T4 `notes`, history |

### 2.2 Classification Inputs
| Input | Value | Evidence |
|-------|-------|----------|
| `J` job type | `<value>` | `<JTBD verb>` |
| `N` cardinality | `<one/few/many>` | `<API pagination, data, intake answer, or A-xx>` |
| `C` complexity | `<simple/medium/complex>` | `<attribute count, sub-collections>` |
| `F` frequency | `<daily/weekly/rare>` | `<role profile>` |
| `R` worst action risk | `<A/B/C>` | `<CTA>` |
| `X` comparison | `<yes/no>` | `<job>` |
| `V` visual recognition | `<yes/no>` | `<attributes>` |
| `S` sequential dependency | `<yes/no>` | `<form analysis>` |

### 2.3 Data Source (front-end only)
| Object | Existing contract (path) or typed mock | Notes |
|--------|----------------------------------------|-------|
| `<Object>` | `<path to API client or query hook>` \| `typed mock in feature` | `<pagination, filters supported>` |

### 2.4 Scope Slice
- **Walking skeleton (build now)**: `<end-to-end flow, state-complete>`
- **Deferred**: `<explicitly excluded enhancements>`

### 2.5 Declared Assumptions (only those that drive a decision)
| ID | Assumption | Drives | Risk if Wrong |
|----|------------|--------|---------------|
| `A-01` | `<statement>` | `<input or rule, e.g., N = many -> T17>` | `<what would change>` |

<!-- Optional, only when requested: Outcome Metrics, Task Step Budget. -->

---

## 3. Page Composition

<!-- Planes 3–4, written by pcs-architect. This section is the `## Page Composition` section of pcs-saas references/04-template-decision-tree.md §4: one block per page with Template, Region Tree, Data Contract, State Matrix, Actions and Friction, Inventory Reuse, Responsive Notes, and Acceptance Checks. The region tree is mandatory; a compact wireframe may follow it. -->

### Page: `<Page name>` — `<route>`
<!-- Sections 1–8 per pcs-saas 04 §4. -->

---

## 4. Navigation & URL State (`pcs-architect`, optional)
- **Navigation path (<= 2 levels)**: `<sidebar item -> page -> tab>`
- **URL parameters**: `<listed per region in each Data Contract>`

---

## 5. Build Notes (`pcs-builder`)
- **Blueprints used**: `<adapter template per page>`
- **Inventory changes**: `<components reused, extended, or added>`
- **Checks**: `<typecheck / lint / build results>`

---

## 6. Verification Report (`pcs-verifier`)

- **Run**: `<initial | after fix round N>` — `<date>`
- **Verdict**: `Pass` (0 blocking) | `Fail` (`<n>` blocking)
- **Static checks**: `<typecheck / lint / build>`
- **Captures**: `.pcs/playwright/<page>-<viewport>-<state>.png` ...

### 6.1 Blocking Findings (Severity 3–4)
| ID | Sev | Check | Evidence | Location | Expected vs Actual | Fix | Status |
|----|-----|-------|----------|----------|--------------------|-----|--------|
| `F-1` | `3` | `S3` | `.pcs/playwright/list-desktop-s3.png` | `<file:line>` | `<expected>` vs `<actual>` | `<one concrete instruction>` | open \| resolved |

### 6.2 Other Findings (Severity 0–2)
| ID | Sev | Check | Evidence | Location | Expected vs Actual | Fix |
|----|-----|-------|----------|----------|--------------------|-----|

### 6.3 Contract Coverage
| Page | Anatomy | `AC-*` passed / total | States verified live / static / n/a | `VC-*` failed | Inventory reuse |
|------|---------|-----------------------|-------------------------------------|---------------|-----------------|

### 6.4 Heuristics, Walkthrough & Laws (rendered UI)
| Heuristic | Severity (`0–4`) | Evidence |
|-----------|------------------|----------|
| H1 Visibility of system status | `<0–4>` | `<evidence>` |
| H2 Match with the real world | | |
| H3 User control and freedom | | |
| H4 Consistency and standards | | |
| H5 Error prevention | | |
| H6 Recognition rather than recall | | |
| H7 Flexibility and efficiency | | |
| H8 Aesthetic and minimalist design | | |
| H9 Recognize, diagnose, recover from errors | | |
| H10 Help and documentation | | |

- **Walkthrough (Q1–Q4)**: `<per step: pass or finding ID>`
- **Cognitive laws (measured)**: `<per law: observation or finding ID>`
- **Keyboard & WCAG 2.2 AA**: `<pass or finding IDs>`
