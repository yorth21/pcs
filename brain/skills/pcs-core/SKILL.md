---
name: pcs-core
description: "Trigger: UX design, new feature, screen flow, wireframe, usability audit, empty states, PCD planes. Apply technology-agnostic 5-Plane UX rules."
license: Apache-2.0
metadata:
  author: yorth21
  version: "1.0"
---

## Activation Contract

Load when planning, specifying, building, or auditing any product feature, user flow, container choice, or UI surface across the 5 Planes of Product-Centered Development (PCD).

Do not load for non-interactive backend scripts or database-only migrations.

## Hard Rules

- Never write Plane 5 (`Surface`) code for `Tier 2` or `Tier 3` features before Planes 1–4 are documented in `pcs/specs/<feature-name>.md` and tracked in `pcs/tasks/<feature-name>.md`.
- Never cite external product companies as design shorthand; apply the cognitive laws, matrices, and token rules in `references/`.
- Map domain entities with the **OOUX ORCA Matrix** (Objects, Relationships, CTAs, Attributes) before defining screens.
- Specify the Happy Path plus all **5 Mandatory Negative States**: `Zero-Data`, `No-Results` (filtered empty), `Contextual Loading`, `Partial Error`, and `Inline Validation Error`.
- Choose containers (`Inline`, `Dialog`, `Sheet`, `Full Page`) by **Context Dependency** and **Cognitive Load** first, never field count alone.
- Scale friction to risk: optimistic Undo toast for reversible actions; explicit `<AlertDialog>` naming consequences for destructive actions.
- Enforce the **60-30-10 color budget**, **3-level typography**, **>= 25% whitespace floor**, and **0 Severity 3–4 heuristic blockers**.

## Decision Gates

| Decision Fork | Action | Reference |
|---------------|--------|-----------|
| Ambiguous user role or task cadence | Ask at most 1 blocking question or log a Declared Assumption | `references/01-planes-strategy-and-scope.md` |
| Selecting `Inline` vs `Dialog` vs `Sheet` vs `Page` | Evaluate background context dependency and cognitive load | `references/02-planes-structure-and-skeleton.md` |
| Reversible vs destructive mutation | Use immediate Undo toast vs consequence `AlertDialog` | `references/02-planes-structure-and-skeleton.md` |
| Visual hierarchy, whitespace, or numeric data | Apply 60-30-10 budget, 3-level type, and `tabular-nums` | `references/03-plane-surface-and-visual-hierarchy.md` |
| Post-build usability & accessibility audit | Score 10 Heuristics (`0–4`) and 4-Question Walkthrough | `references/04-heuristic-and-walkthrough-verification.md` |

## Execution Steps

1. Load `references/01-planes-strategy-and-scope.md` to define Role Profile, JTBD, Declared Assumptions, OOUX ORCA, and HTA/KLM step budget.
2. Load `references/02-planes-structure-and-skeleton.md` to define IA (<= 2 levels), 5 Negative States, Container Matrix, Friction Matrix, and Wireframe.
3. Write `pcs/specs/<feature-name>.md` (`assets/ux-spec-template.md`) and `pcs/tasks/<feature-name>.md` (`assets/ux-tasks-template.md`).
4. Load `references/03-plane-surface-and-visual-hierarchy.md` alongside the Stack Adapter during Plane 5 build.
5. Load `references/04-heuristic-and-walkthrough-verification.md` to verify 0 Severity `3–4` blockers.

## Output Contract

Return:
- Completed 5-Plane specification in `pcs/specs/<feature-name>.md` and work units in `pcs/tasks/<feature-name>.md`.
- Container and friction choices justified by cognitive laws.
- Verification scorecard across the 10 Heuristics (`0–4`), 4-Question Walkthrough, and 5 Negative States.

## References

- `references/01-planes-strategy-and-scope.md` — Strategy (Role, JTBD) and Scope (OOUX ORCA, HTA/KLM).
- `references/02-planes-structure-and-skeleton.md` — Structure (IA, 5 Negative States) and Skeleton (Containers, Friction, Laws).
- `references/03-plane-surface-and-visual-hierarchy.md` — Surface (60-30-10 color, 3-level type, scanning, `tabular-nums`).
- `references/04-heuristic-and-walkthrough-verification.md` — 10 Heuristics (`0–4`), 4-Question Walkthrough, WCAG 2.2 AA.
- `assets/ux-spec-template.md` — Template for `pcs/specs/<feature-name>.md`.
- `assets/ux-tasks-template.md` — Template for `pcs/tasks/<feature-name>.md`.
