---
name: pcs-saas
description: "Trigger: SaaS screen, page template, compose page, organism, dashboard, table page, settings, wizard, UI inventory. Select template and compose state-complete SaaS pages."
license: Apache-2.0
metadata:
  author: yorth21
  version: "1.0"
---

## Activation Contract

Load when composing, building, or verifying any screen of a SaaS application: choosing a page template, assembling organisms, writing a Page Composition Spec, generating the project UI inventory, or checking a rendered page against its spec. Load together with `pcs-core` (methodology) and the active Stack Adapter (concrete components).

Do not load for marketing sites, content pages, native mobile apps, or backend-only work.

## Hard Rules

- Never improvise a layout: every page MUST name exactly one template ID from `references/01-page-templates.md` (`tpl-*`) and compose only catalog organisms (`org-*`) or justified project organisms.
- Select the template with `references/04-template-decision-tree.md`; record the path taken as the rationale.
- Reuse first: an inventory organism that implements the same catalog ID MUST be reused; a new one requires a written justification in the spec.
- Every region declares the full state matrix: loading, empty first-use, empty no-results, error, no-permission, partial/degraded, and success feedback (extends the `pcs-core` 5 Mandatory Negative States).
- One primary action per region; same object → same representation; same action → same label, icon, and position across screens.
- Never name a UI library, framework, or external company in this skill or in specs derived from it; concrete components belong to the Stack Adapter, which maps every `tpl-*` and `org-*` ID.

## Decision Gates

| Fork | Action | Reference |
|------|--------|-----------|
| Which page archetype | Walk the decision tree from job type and object traits | `references/04-template-decision-tree.md` |
| Page, sheet, dialog, or inline | Apply container and friction rows of the tree; defer to `pcs-core` matrices on ties | `references/04-template-decision-tree.md` |
| Which organisms fill each region | Read template anatomy, then organism rules | `references/01-page-templates.md`, `references/02-organisms.md` |
| Field, badge, format, action menu details | Apply molecule rules | `references/03-molecules-and-patterns.md` |
| Spacing, type, density, color, copy | Apply visual system rules and checklist | `references/05-visual-consistency.md` |
| Existing screens in the project | Read or generate `pcs/design/inventory.md` | `references/06-ui-inventory.md` |

## Execution Steps

1. Read `pcs/design/inventory.md`; if missing, generate it per `references/06-ui-inventory.md` (explorer phase).
2. Classify the request (job type, cardinality, complexity, frequency, risk, comparison) and walk the decision tree to a template ID, container, and friction level.
3. Fill the template anatomy region by region with organisms, then molecules.
4. Declare the data contract and the state matrix for every region.
5. Write the Page Composition Spec in the exact format of `references/04-template-decision-tree.md` §4.
6. During build, apply `references/05-visual-consistency.md`; during verify, test every acceptance check and the screenshot checklist.

## Output Contract

Return a Page Composition Spec containing: template ID with decision-tree path, region → organism → molecule tree, data contract per region, state matrix per region, actions with friction level, inventory reuse list, responsive notes, and numbered acceptance checks.

## References

- `references/01-page-templates.md` — Page archetypes (`tpl-*`): anatomy, states, responsive rules, acceptance checks.
- `references/02-organisms.md` — Reusable organisms (`org-*`): anatomy, behaviour, states, accessibility.
- `references/03-molecules-and-patterns.md` — Molecules (`mol-*`) and micro-patterns.
- `references/04-template-decision-tree.md` — Deterministic decision tree, worked examples, Page Composition Spec format.
- `references/05-visual-consistency.md` — Spacing, type, density, color roles, microcopy, screenshot checklist.
- `references/06-ui-inventory.md` — Per-project UI inventory format, procedure, and reuse rule.
