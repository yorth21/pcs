---
name: pcs-core
description: "Trigger: UX design, new feature, screen flow, usability audit, empty states, PCD planes. Apply technology-agnostic 5-Plane UX rules and verification."
license: Apache-2.0
metadata:
  author: yorth21
  version: "1.1"
---

## Activation Contract

Load when planning, specifying, building, or auditing any product feature, user flow, container choice, or UI surface across the 5 Planes of Product-Centered Development (PCD). For SaaS screens, load together with `pcs-saas` (page catalog and Page Composition Spec) and the active Stack Adapter.

Do not load for non-interactive backend scripts or database-only migrations.

## Hard Rules

- Never write Plane 5 (`Surface`) code for `Tier 2` or `Tier 3` features before Planes 1–2 and the Page Composition Spec (Planes 3–4, `pcs-saas` `04` §4) are in `pcs/specs/<feature-name>.md` and tasks are in `pcs/tasks/<feature-name>.md`.
- Never cite external product companies as design shorthand; apply the laws, matrices, and rules in `references/`.
- Map domain objects with the **OOUX ORCA Matrix** (Objects, Relationships, CTAs with risk tier, Attributes) before choosing a page.
- Record only what changes the page: role, jobs, objects, cardinality, frequency, risk, scope slice. Declared Assumptions only when they drive a design decision. KLM budgets and outcome metrics are optional.
- Every data region declares the full state matrix (`S1`–`S7` of `pcs-saas`), a superset of the **5 Mandatory Negative States**.
- Choose containers by **Context Dependency** and **Cognitive Load**, never field count alone; scale friction to risk.
- Cognitive laws and heuristics are verified on the rendered UI with observations, never declared as compliant in the spec.
- Zero Severity `3–4` findings at build closure.

## Decision Gates

| Decision Fork | Action | Reference |
|---------------|--------|-----------|
| Ambiguous role, job, or volume | Use the intake answer, else log a decision-driving Declared Assumption | `references/01-planes-strategy-and-scope.md` |
| Template, organisms, state matrix | Write the Page Composition Spec | `pcs-saas` `references/04-template-decision-tree.md` |
| `Inline` vs `Dialog` vs `Sheet` vs `Page`; friction | Context dependency, cognitive load, risk tier | `references/02-planes-structure-and-skeleton.md` |
| Visual hierarchy or numeric data | Apply surface rules | `references/03-plane-surface-and-visual-hierarchy.md` |
| Post-build or audit verification | Contract checks, forced states, heuristics, laws on rendered UI | `references/04-heuristic-and-walkthrough-verification.md` |

## Execution Steps

1. Strategy: apply `references/01-planes-strategy-and-scope.md` and write spec §1–2 (`assets/ux-spec-template.md`).
2. Compose: apply `references/02-planes-structure-and-skeleton.md` and `pcs-saas`; write spec §3 (Page Composition) and `pcs/tasks/<feature-name>.md` (`assets/ux-tasks-template.md`).
3. Build: apply `references/03-plane-surface-and-visual-hierarchy.md` with the Stack Adapter.
4. Verify: apply `references/04-heuristic-and-walkthrough-verification.md`; write spec §6 (Verification Report).

## Output Contract

Return:
- `pcs/specs/<feature-name>.md` with Planes 1–2, the Page Composition Spec, and the Verification Report; work units in `pcs/tasks/<feature-name>.md`.
- Fix-ready findings (ID, severity `0–4`, evidence, location, expected vs actual, fix) and zero Severity `3–4` at closure.

## References

- `references/01-planes-strategy-and-scope.md` — Strategy (role, jobs) and Scope (ORCA, classification inputs, slice).
- `references/02-planes-structure-and-skeleton.md` — Structure and Skeleton rules behind the Page Composition Spec.
- `references/03-plane-surface-and-visual-hierarchy.md` — Surface hierarchy, color budget, type, numeric display.
- `references/04-heuristic-and-walkthrough-verification.md` — Contract verification, forced states, heuristics, laws, finding format.
- `assets/ux-spec-template.md` — Template for `pcs/specs/<feature-name>.md`.
- `assets/ux-tasks-template.md` — Template for `pcs/tasks/<feature-name>.md`.
