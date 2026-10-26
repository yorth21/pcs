# Planes 1 & 2 — Product Strategy and Scope Specification

This reference defines the mandatory engineering rules for **Plane 1 (Strategy)** and **Plane 2 (Scope)** in Product-Centered Development (PCD). Execute these steps before designing navigation trees, selecting containers, or writing UI code.

---

## 1. Plane 1: Product Strategy

Plane 1 answers **Who is operating this interface, in what real-world environment, and what measurable outcome must they achieve?**

### 1.1 Goal-Directed Role & Environment Profiling

Never invent fictional demographic personas (e.g., age, hobbies, stock photos). Profile the **Operational Role** across four concrete engineering dimensions that directly govern layout density, control sizing, and interaction speed:

| Dimension | Low-Intensity / Occasional Spectrum | High-Intensity / Daily Workbench Spectrum | Concrete UI Impact |
|-----------|-------------------------------------|-------------------------------------------|--------------------|
| **Frequency of Use** | Monthly or one-time setup (onboarding, billing config, role permissions) | 20–200+ executions per shift (triage queues, order reconciliation, scheduling) | Occasional -> guided step-by-step clarity, explicit descriptions. Daily -> compact density, keyboard shortcuts, inline edits, zero redundant confirmations. |
| **Domain Expertise** | Novice or cross-functional viewer | Domain specialist (auditor, dispatcher, clinician, analyst) | Novice -> progressive disclosure, plain-language labels, contextual hints. Specialist -> high information scent, simultaneous multi-attribute visibility, rapid filtering. |
| **Workspace Environment** | Quiet desktop office, large external monitor (`>= 1440px`) | Field tablet, shared terminal, multi-window split screen (`768px–1280px`), high interruption rate | High interruption -> persistent URL state, auto-saved drafts or dirty-close guards, clear visual resume cues. |
| **Error Consequence** | Cosmetic or easily reversible (renaming a tag, toggling a view filter) | Financial, legal, clinical, or irreversible data mutation | High consequence -> explicit audit previews, Risk-Proportional Friction (`AlertDialog` with impact statement). |

### 1.2 Jobs-to-be-Done (JTBD) Formulation

Every feature specification MUST state the core Job-to-be-Done using the canonical causal formula:

> **When** `[specific triggering situation and operational constraint]`,
> **I want to** `[core action/capability without prescribing UI mechanics]`,
> **so I can** `[measurable user and business outcome]`.

Break the JTBD outcomes into three mandatory dimensions:

1. **Functional Outcome**: What concrete task is completed faster, with fewer steps, or with zero manual cross-referencing?
2. **Emotional Outcome**: How does the interface eliminate operator anxiety (e.g., confidence that filters are active, certainty about whether edits are saved, safety against accidental deletion)?
3. **Social / Organizational Outcome**: How does the result make the operator's work auditable, transparent, and trustworthy to teammates or downstream consumers?

### 1.3 Declared Assumptions Ledger

When requirements do not specify every operational detail:
- If an ambiguity creates a **major architectural fork** (e.g., single-record approval vs. batch multi-record approval), stop and ask **at most ONE high-leverage question** using Lossless Blocking Prompts.
- For bounded operational details, record each assumption in the **Declared Assumptions Ledger** with its risk if wrong and concrete validation method:

| Assumption ID | Declared Assumption | Confidence (`High`/`Med`/`Low`) | Risk if Wrong & UX Impact | Validation Method |
|---------------|---------------------|---------------------------------|---------------------------|-------------------|
| `A-01` | Operators process 15–50 records per session and need persistent filter state across row inspections. | `High` | High — full-page navigation would wipe scroll/filter context on every edit; requires a contextual side `Sheet` + URL search params. | Inspect existing API pagination payloads and verify workflow with 3 sequential row edits. |

### 1.4 Measurable Outcome Metrics

Every Plane 1 Strategy section MUST define verifiable product and interaction outcome targets so Plane 2 Scope and Verification can test whether the design succeeded:

| Metric Category | Engineering Target | Verification Mechanism |
|-----------------|--------------------|------------------------|
| **Interaction Cost (KLM Budget)** | `<= 3 clicks` for primary CRUD creation/edit; `1–2 clicks` for high-frequency status toggles | HTA/KLM step count in Plane 2 & Cognitive Walkthrough in Verification |
| **Context Preservation** | `0` lost filter/scroll states when inspecting or editing collection records | URL search-param persistence + `Sheet` container verification |
| **Error Recovery & Clarity** | `100%` of negative paths (`Zero-Data`, `No-Results`, `Loading`, `Partial Error`, `Validation Error`) have an explicit recovery CTA | Static & Playwright state-completeness audit (`0` Severity 3–4 issues) |

---

## 2. Plane 2: Product Scope, OOUX & Task Budgeting

Plane 2 translates Strategy into a **coherent domain model** and a **minimalist interaction budget**. It prevents two classic frontend failures: **scattered domain entities** (where the same object looks and behaves differently across screens) and **workflow bloat** (where a 2-click task takes 6 modals).

### 2.1 Object-Oriented UX (OOUX) — The ORCA Matrix

Before sketching screens or pages, extract the **nouns** from the JTBD and map them in an **ORCA Matrix**:

1. **O — Objects**: Core real-world domain entities in the user's mental model (e.g., `Invoice`, `DiscrepancyNote`, `PaymentBatch`). Limit a single feature workspace to **1 Primary Object** and at most **2–3 Secondary/Related Objects**.
2. **R — Relationships**: Explicit cardinality between objects (`1:1`, `1:N`, `M:N`) and how the user traverses between them without losing context.
3. **C — Calls-to-Action (CTAs)**: Every permitted verb on the object, classified by risk level (`Read/Inspect`, `Create`, `Update/Inline Toggle`, `Destructive/Irreversible`).
4. **A — Attributes**: The exact data properties of each object, ranked strictly into four visual priority tiers:
   - **Tier 1 (Identity)**: Unique human identifier (Name, Code, Reference ID) — always visible, left-aligned.
   - **Tier 2 (State / Urgency)**: Lifecycle status or health indicator — rendered as a semantic badge or status dot.
   - **Tier 3 (Quantitative / Temporal)**: Amounts, counts, timestamps — rendered with `tabular-nums`, right-aligned in tables.
   - **Tier 4 (Secondary / Detail)**: Notes, audit logs, full descriptions — deferred to detail `Sheet` or expandable views (`Progressive Disclosure`).

#### OOUX Anti-Fragmentation Rule
If the same domain Object appears in a summary card, a table row, and a detail panel, its **Tier 1 Identity** and **Tier 2 State Badge** MUST use identical terminology, status colors, and formatting across all three representations.

### 2.2 Hierarchical Task Analysis (HTA) & Keystroke-Level Model (KLM) Budget

Decompose the primary user goal into sequential steps, compare the **Naive / Unpruned Baseline** against the **KISS-Pruned Budget** (clicks, keystrokes, and view transitions), and enforce a strict **Interaction Step Budget**:

| Task Frequency / Type | Unpruned Baseline Anti-Pattern | KISS-Pruned Target Budget | Maximum Container Transitions | KLM Optimization Rule |
|-----------------------|--------------------------------|---------------------------|-------------------------------|-----------------------|
| **High-Frequency Micro-Task** (status toggle, quick filter, single-field edit) | 4–5 clicks across a row menu + modal + `"Are you sure?"` dialog | **1–2 clicks** (or 1 keyboard shortcut) | `0` (Inline or Popover) | Never open a modal dialog for a binary toggle or single-select status change; use optimistic update + Undo toast. |
| **Standard CRUD Creation / Edit** (3–10 attributes) | 6–8 clicks across a full page redirect + confirmation popup + manual back navigation | **3–5 clicks** + typing | `1` (`Dialog` or `Sheet`) | Pre-fill smart defaults (Tesler's Law); auto-focus the first input on open; submit on `Enter` / `Cmd+Enter`. |
| **Complex Multi-Step Workflow** (reconciliation, bulk import, configuration) | 5+ fragmented wizard screens hiding cross-section context | **<= 3 logical stages** | `1` (`Full Page` or `Split Workbench`) | Combine related inputs onto a single scrollable surface with clear section headers before resorting to a multi-step wizard. |

### 2.3 KISS Anti-Bloat Pruning Rules

During Plane 2, actively prune any proposed UI element that violates these rules:
- **Zero Redundant Confirmation Modals**: Never add an `"Are you sure you want to save?"` dialog to non-destructive create/update actions.
- **Zero Decorative Dashboards**: Every metric card or chart at the top of a screen MUST either (a) answer a daily operational triage question or (b) act as a clickable quick-filter for the collection below.
- **Zero Duplicate Filter Surfaces**: Do not combine a left-hand filter sidebar, a top filter bar, AND column-header filter popovers on the same table. Use a single cohesive `DataTableToolbar` (search input + 1–3 faceted filter pills + reset action).

### 2.4 User Story Mapping — Walking Skeleton vs. Deferred Scope

Separate scope into two explicit buckets in `pcs/specs/<feature-name>.md`:
- **Walking Skeleton (MVP Slice — Must Build Now)**: The minimum end-to-end flow across all required objects, including all **5 Mandatory Negative States**.
- **Deferred Enhancements (Out of Current Slice)**: Secondary bulk actions, advanced export formats, or historical analytics explicitly excluded to keep cognitive load and review scope bounded.
