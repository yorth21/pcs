---
name: pcs-adapter-react-shadcn
description: "Trigger: React component, shadcn/ui, Tailwind v4, TanStack Table/Router/Query, React Hook Form, Zod, UI build. Compose vanilla shadcn/ui surfaces."
license: Apache-2.0
metadata:
  author: yorth21
  version: "1.0"
---

## Activation Contract

Load alongside `pcs-core` when architecting, building, refactoring, or verifying React 19 + TypeScript surfaces with Tailwind CSS v4, `shadcn/ui`, `radix-ui`, TanStack Router/Query/Table, `react-hook-form`, and `zod`.

Do not load for non-React stacks or backend-only services.

## Hard Rules

- Enforce **KISS & Vanilla `shadcn/ui` First**: compose `Card`, `Dialog`, `Sheet`, `AlertDialog`, `Table`, `Empty`, `Skeleton`, `Alert`, `Field`, `FieldError`, `Button`, and `DropdownMenu` directly from `@/components/ui/*` with zero pass-through wrapper abstractions.
- Import `radix-ui` directly ONLY when a required primitive does not exist in `src/components/ui/`.
- Enforce the **Promotion Rule**: keep single-feature code in `src/features/[feature]/[subfeature]/{components,hooks,schemas,types}/`; promote to `src/components/{ui,form,data-table,layout}/` only when shared by `>= 2` features.
- Never use Atomic Design suffixes (`Atom`, `Molecule`, `Organism`, `Template`) in paths or identifiers.
- Implement separate `Zero-Data` (`Empty` with onboarding CTA) and `No-Results` (filtered empty state with `"Clear filters"`) branches.
- Apply `tabular-nums` to all numeric, currency, date, and ID values, and wire `aria-invalid` + `<FieldError>` on all validated inputs.

## Decision Gates

| Implementation Fork | Adapter Action | Reference |
|---------------------|----------------|-----------|
| Placing a component or hook | 1 feature -> `src/features/`; `>= 2` features -> `src/components/` | `references/01-folder-and-component-architecture.md` |
| Mapping Plane 4 containers to code | Select vanilla `Dialog`, `Sheet`, `AlertDialog`, or `ResizablePanelGroup` | `references/02-vanilla-shadcn-and-radix-catalog.md` |
| URL search state, queries, tables, or forms | Apply TanStack Router/Query/Table + RHF + Zod v4 patterns | `references/03-tanstack-rhf-zod-patterns.md` |
| Styling tokens, spacing, and transitions | Apply Tailwind v4 `@theme inline` tokens, `tabular-nums`, and 150–200ms motion | `references/04-tailwind-v4-tokens-and-motion.md` |

## Execution Steps

1. Read `references/01-folder-and-component-architecture.md` to confirm target paths and hook separation.
2. Read `references/02-vanilla-shadcn-and-radix-catalog.md` to map Skeleton containers and friction tiers to `@/components/ui/*`.
3. Read `references/03-tanstack-rhf-zod-patterns.md` to wire URL search params, query skeletons, `DataTable` states, and RHF/Zod validation.
4. Read `references/04-tailwind-v4-tokens-and-motion.md` and `assets/composition-blueprints.tsx` to enforce semantic tokens, `tabular-nums`, and keyboard focus rules.

## Output Contract

Return:
- Production TypeScript/React components inside authorized `src/features/` or `src/components/` paths.
- Complete implementation of all 5 Mandatory Negative States with vanilla `shadcn/ui` primitives.
- Typecheck-clean code (`tsc --noEmit`) with zero custom wrapper bloat.

## References

- `references/01-folder-and-component-architecture.md` — Feature folders, Promotion Rule, and hook separation.
- `references/02-vanilla-shadcn-and-radix-catalog.md` — Vanilla `shadcn/ui` catalog and `radix-ui` escape hatch.
- `references/03-tanstack-rhf-zod-patterns.md` — TanStack Router v1, Query v5, Table v8, and RHF v7 + Zod v4.
- `references/04-tailwind-v4-tokens-and-motion.md` — Tailwind CSS v4 tokens, spacing rhythm, `tabular-nums`, and motion.
- `assets/composition-blueprints.tsx` — Canonical React 19 + TypeScript composition blueprints.
