# Planes 1 & 2 — Product Strategy and Scope Specification

This reference defines the rules for **Plane 1 (Strategy)** and **Plane 2 (Scope)** in Product-Centered Development (PCD). Its output is the input of the page composer: every item recorded here MUST change a page decision (template, container, friction, density, columns, or scope). Do not write sections that change nothing.

---

## 1. Plane 1: Product Strategy

Plane 1 answers **who operates this interface, in what environment, and which job the page must complete**.

### 1.1 Operational Role & Environment

Never invent demographic personas (age, hobbies, photos). Profile the **operational role** across the dimensions that govern layout density, control sizing, and interaction speed:

| Dimension | Occasional End | Daily Workbench End | Page Impact |
|-----------|----------------|---------------------|-------------|
| **Frequency of use** (`F`: `daily` \| `weekly` \| `rare`) | Monthly or one-time setup | 20–200+ executions per shift | Occasional -> guided clarity, explicit descriptions, comfortable density. Daily -> compact density, keyboard paths, inline edits, no redundant confirmations. |
| **Domain expertise** | Novice or cross-functional viewer | Domain specialist | Novice -> progressive disclosure, plain labels, hints. Specialist -> many attributes visible, fast filtering. |
| **Environment & viewport** | Desktop office, large monitor | Tablet, shared terminal, split screen, frequent interruptions | Interruptions -> URL state, drafts or dirty-close guards, visible resume cues. Small viewports -> mobile rules first. |
| **Error consequence** | Cosmetic or reversible | Financial, legal, clinical, irreversible, or affects others | High consequence -> review before commit and risk-proportional friction. |

### 1.2 Jobs-to-be-Done (JTBD)

State one **primary job** with the canonical formula:

> **When** `[triggering situation and constraint]`,
> **I want to** `[core action, without prescribing UI mechanics]`,
> **so I can** `[user and business outcome]`.

- The main verb of the "I want to" clause maps to the job type `J` of `pcs-saas` `references/04-template-decision-tree.md` §1 (`monitor`, `find`, `process`, `inspect`, `create`, `edit`, `configure`, `transfer`, `authenticate`, `onboard`, `recover`).
- List **secondary jobs** (0–4) the same page must also serve, one line each with its job type. A secondary job either adds a region or action to the page or becomes a separate page; say which.

### 1.3 Questions and Declared Assumptions

- Ambiguities that change the design (user role, primary job, frequency, record volume) are asked once, in the coordinator's single intake round. Agents never ask the user directly.
- Everything else becomes a **Declared Assumption** only when it drives a design decision. Each assumption names the decision or rule it drives and the risk if wrong:

| ID | Declared Assumption | Drives | Risk if Wrong |
|----|---------------------|--------|---------------|
| `A-01` | Operators process 15–50 records per session and return to the list after each record. | `N = many`, `F = daily` -> `T17` index table + `K03` sheet | High — a full-page record view would reset filters and scroll on every inspection. |

### 1.4 Optional: Outcome Metrics

Outcome metrics (interaction cost, context preservation, recovery rate) are optional. Add them only when the request or the team asks for measurable targets; never invent numeric targets to fill the section.

---

## 2. Plane 2: Product Scope & OOUX

Plane 2 translates the jobs into a **coherent object model** and a **bounded build slice**. It prevents scattered objects (the same object looking different across screens) and workflow bloat.

### 2.1 Object-Oriented UX (OOUX) — The ORCA Matrix

Extract the nouns from the jobs and map them before choosing any page:

1. **O — Objects**: real domain objects in the user's mental model. One **primary object** per page and at most **3 related objects**.
2. **R — Relationships**: cardinality (`1:1`, `1:N`, `M:N`) and how the user moves between related objects without losing context.
3. **C — Calls-to-Action**: every verb on the object, the role allowed to perform it, and its **risk tier**:
   - `A` — reversible (archive, toggle, reassign).
   - `B` — unsaved-work loss (closing a modified form).
   - `C` — destructive, irreversible, money movement, external send, or affects other users' data.
4. **A — Attributes**: ranked into four display tiers:
   - **T1 Identity**: human identifier (name, code, reference) — always visible, first column.
   - **T2 State**: lifecycle status or health — status badge with text.
   - **T3 Quantitative / Temporal**: amounts, counts, dates — formatted, tabular figures, right-aligned in tables.
   - **T4 Detail**: notes, history, long text — deferred to detail views.

**Anti-fragmentation rule**: when an object appears as a card, a row, and a detail view, its T1 identity and T2 status use identical wording, color role, and format in all three.

### 2.2 Classification Inputs

Fill the `pcs-saas` classification inputs from the ORCA matrix with one line of evidence each (explorer map, API contract, intake answer, or Declared Assumption ID). Unknown values take the default of `pcs-saas` `04` §1.

| Input | Meaning | Typical Evidence |
|-------|---------|------------------|
| `J` | Job type of the primary job | JTBD verb |
| `N` | Cardinality per user/workspace at 12 months (`one`, `few`, `many`) | API pagination, existing data, intake answer |
| `C` | Object complexity (`simple`, `medium`, `complex`) | Attribute count shown to this role, sub-collections |
| `F` | Frequency (`daily`, `weekly`, `rare`) | Role profile |
| `R` | Worst action risk in scope (`A`, `B`, `C`) | ORCA CTAs |
| `X` | Comparison need across records | Jobs |
| `V` | Visual recognition (image or preview) | Attributes |
| `S` | Sequential dependency between inputs | Form analysis |

### 2.3 Data Source (Front-End Only)

Name the existing API contract, client, or query hook that serves each object (path). When none exists, declare **typed mocks** that the builder creates inside the feature, shaped like the expected contract. Never plan backend, schema, or server changes.

### 2.4 Anti-Bloat Pruning Rules

- **Zero redundant confirmations**: never confirm non-destructive create or update actions.
- **Zero decorative dashboards**: every metric or chart answers an operational question or acts as a quick filter.
- **Zero duplicate filter surfaces**: one filter bar per collection.

### 2.5 Scope Slice

Separate scope into two buckets:
- **Walking skeleton (build now)**: the minimum end-to-end flow across the required objects, state-complete.
- **Deferred enhancements**: secondary bulk actions, exports, analytics, and anything not needed by the primary job.

### 2.6 Optional: Task Step Budget

A hierarchical task analysis or keystroke-level step budget is optional. Use it only when the request is about reducing interaction cost; otherwise the composer's decision tree and the verifier's walkthrough cover step count.
