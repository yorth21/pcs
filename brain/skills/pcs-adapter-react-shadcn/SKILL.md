---
name: pcs-adapter-react-shadcn
description: "Trigger: React component, shadcn/ui, Tailwind v4, TanStack Table/Router/Query, React Hook Form, Zod, UI build. Compose vanilla shadcn/ui surfaces."
license: Apache-2.0
metadata:
  author: yorth21
  version: "1.1"
---

## Activation Contract

Load alongside `pcs-core` and `pcs-saas` when architecting, building, refactoring, or verifying React 19 + TypeScript surfaces with Tailwind CSS v4, `shadcn/ui`, `radix-ui`, TanStack Router/Query/Table, `react-hook-form`, and `zod`.

Do not load for non-React stacks or backend-only services.

## Hard Rules

- **Blueprint first**: build each page from the `assets/templates/` blueprint of its `tpl-*` ID and each spec node from its row in `references/05-saas-pattern-mapping.md`; never improvise layout, states, or ARIA.
- **Inventory first**: apply the `pcs-saas` `references/06-ui-inventory.md` Reuse Rule to every `org-*`/`mol-*` node; an inventory component implementing the same ID MUST be reused or extended.
- **Vanilla `shadcn/ui` first**: compose `@/components/ui/*` directly, no pass-through wrappers; import `radix-ui` only when no primitive exists.
- **Promotion Rule**: single-feature code stays in `src/features/[feature]/[subfeature]/`; promote to `src/components/{ui,form,data-table,layout}/` only when `>= 2` features share it.
- Never use `Atom`, `Molecule`, `Organism`, or `Template` in paths or identifiers.
- Render every `S1`–`S7` state the spec declares, with `S2` (first-use) and `S3` (no-results) as separate branches.
- Apply `tabular-nums` to numeric, currency, date, and ID values; wire `aria-invalid` + `<FieldError>` on validated inputs.

## Decision Gates

| Implementation Fork | Adapter Action | Reference |
|---------------------|----------------|-----------|
| Spec node → component, classes, ARIA, state branch | Read the node's row; start from the template blueprint | `references/05-saas-pattern-mapping.md`, `assets/templates/` |
| Placing a component or hook | 1 feature -> `src/features/`; `>= 2` features -> `src/components/` | `references/01-folder-and-component-architecture.md` |
| Mapping containers and friction tiers | Select vanilla `Dialog`, `Sheet`, `AlertDialog`, or `ResizablePanelGroup` | `references/02-vanilla-shadcn-and-radix-catalog.md` |
| URL search state, queries, tables, or forms | Apply TanStack Router/Query/Table + RHF + Zod v4 patterns | `references/03-tanstack-rhf-zod-patterns.md` |
| Tokens, spacing, breakpoints, motion | Apply Tailwind v4 `@theme inline` tokens and 150–200ms motion | `references/04-tailwind-v4-tokens-and-motion.md` |

## Execution Steps

1. Read the Page Composition Spec and `pcs/design/inventory.md`; resolve reuse/extend/new per node.
2. Copy the spec's `tpl-*` blueprint (or the closest one named in mapping §5) into the feature folder; split it per its header comment.
3. Fill each region from the mapping rows; keep every state branch and `AC-*` comment.
4. Wire URL params, queries, forms, and tokens per references 03 and 04.
5. Run `tsc --noEmit`; verify every `AC-*` check and state cell on the rendered page.

## Output Contract

Return:
- TypeScript/React components in authorized `src/features/` or `src/components/` paths, traceable to one `tpl-*` blueprint.
- Every spec `S1`–`S7` state rendered with vanilla primitives; `AC-*` comments on the satisfying elements.
- Typecheck-clean code (`tsc --noEmit`) and an inventory entry for each new shared component.

## References

- `references/01-folder-and-component-architecture.md` — Feature folders, Promotion Rule, and hook separation.
- `references/02-vanilla-shadcn-and-radix-catalog.md` — Vanilla `shadcn/ui` catalog and `radix-ui` escape hatch.
- `references/03-tanstack-rhf-zod-patterns.md` — TanStack Router v1, Query v5, Table v8, and RHF v7 + Zod v4.
- `references/04-tailwind-v4-tokens-and-motion.md` — Tailwind CSS v4 tokens, spacing rhythm, `tabular-nums`, and motion.
- `references/05-saas-pattern-mapping.md` — Every `tpl-*`, `org-*`, `mol-*` ID mapped to components, classes, ARIA, states, and file location.
- Page blueprints (`tpl-form-long` also covers `tpl-form-short`): `assets/templates/tpl-index-table.tsx`, `assets/templates/tpl-list-detail.tsx`, `assets/templates/tpl-record-detail.tsx`, `assets/templates/tpl-form-long.tsx`, `assets/templates/tpl-wizard.tsx`, `assets/templates/tpl-settings.tsx`, `assets/templates/tpl-dashboard.tsx`, `assets/templates/tpl-inbox-queue.tsx`.
- `assets/composition-blueprints.tsx` — Legacy negative-state example; superseded by `assets/templates/`.
