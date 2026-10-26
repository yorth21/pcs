# Folder Structure, Promotion Rule and Component Architecture

This reference defines the mandatory directory layout, the **Cross-Feature Promotion Rule**, Container-Presentational separation, and naming conventions for React 19 + TypeScript projects using `pcs-adapter-react-shadcn`.

---

## 1. Repository Directory Architecture (Screaming Feature Structure)

Organize source code so the folder tree immediately communicates **business capabilities (`src/features/`)** while isolating **cross-cutting UI infrastructure (`src/components/`)**:

```text
src/
├── components/                        # Cross-cutting components shared by >= 2 features
│   ├── ui/                            # Vanilla shadcn/ui primitives (Button, Card, Dialog, Sheet, etc.)
│   ├── form/                          # Shared RHF field adapters (FormInput, FormSelect, SubmitButton)
│   ├── data-table/                    # Shared TanStack Table shell (DataTable, DataTableToolbar, DataTablePagination)
│   └── layout/                        # App shell & structural headers (AppSidebar, DashboardMain, PageHeader)
├── features/                          # Domain modules (Screaming Architecture)
│   └── [feature]/                     # e.g., billing, inventory, procedures
│       └── [subfeature]/              # e.g., invoice-reconciliation, catalog (optional if flat feature)
│           ├── components/            # Feature-local presentational & container components
│           │   ├── [feature]-table.tsx
│           │   ├── [feature]-columns.tsx
│           │   ├── [feature]-sheet.tsx
│           │   ├── [feature]-delete-dialog.tsx
│           │   └── [feature]-skeleton.tsx
│           ├── hooks/                 # TanStack Query hooks & stateful orchestration hooks
│           │   └── use-[feature].ts
│           ├── schemas/               # Zod v4 validation & URL search-param schemas
│           │   └── [feature].schema.ts
│           ├── types/                 # Domain TypeScript interfaces (OOUX ORCA objects)
│           │   └── [feature].types.ts
│           └── index.tsx              # Feature screen entry point composed into routes
├── routes/                            # @tanstack/react-router route definitions (thin wiring only)
├── hooks/                             # Global UI hooks (use-mobile, use-debounced-value)
└── lib/                               # Stateless utilities (cn, formatters, api-client)
```

---

## 2. The Cross-Feature Promotion Rule (1 vs. >= 2 Features)

Never pollute `src/components/` with single-feature business components, and never duplicate generic primitives across multiple feature folders:

| Reusability Scope | Mandatory Location | Rule |
|-------------------|--------------------|------|
| **Used by 1 Feature only** | `src/features/[feature]/[subfeature]/components/` | Keep all domain-specific cards, sheets, dialogs, column definitions, and status badges inside the feature folder. |
| **Used by `>= 2` Distinct Features** | `src/components/{ui,form,data-table,layout}/` | When a second feature requires the exact same structural pattern (e.g., a generic `CurrencyInput`, `DataTableFacetedFilter`, or `PageHeader`), extract domain logic out via props/generics and promote the component to `src/components/`. |
| **Route Files (`src/routes/`)** | `src/routes/**` | Route files MUST remain thin: define `validateSearch` (Zod schema), loader/prefetch wiring, and render the feature entry component from `src/features/[feature]/index.tsx`. Never write 400-line UI trees directly inside a route file. |

---

## 3. Prohibition of Atomic Design Suffixes

**Hard Rule:** Never create directories, filenames, or TypeScript identifiers containing `Atom`, `Molecule`, `Organism`, or `Template` (e.g., `src/components/atoms/`, `ButtonAtom`, `InvoiceRowMolecule`, `HeaderOrganism`).

**Why:** Atomic Design classifications create subjective debates over whether a search bar or form field is a "molecule" or "organism" while obscuring functional purpose. Classify components strictly by **functional role**:
- `src/components/ui/` — Low-level interactive primitives (`shadcn/ui`).
- `src/components/form/` — Form-bound input primitives and submit buttons.
- `src/components/data-table/` — Tabular collection primitives.
- `src/components/layout/` — Page shells, headers, sidebars, and split workbenches.
- `src/features/[feature]/components/` — Domain-specific compositions named after the **OOUX Object + Role** (e.g., `InvoiceReconciliationSheet`, `ProcedureDeleteDialog`, `ProcedureCatalogSkeleton`).

---

## 4. Container-Presentational & Custom Hook Separation

Keep components easy to test, scan, and verify by separating **data orchestration** from **visual surface composition**:

1. **Custom Domain Hooks (`hooks/use-[feature].ts`)**:
   - Encapsulate `@tanstack/react-query` `useQuery` / `useMutation` calls, cache invalidation, optimistic updates, and `sonner` toast notifications inside dedicated hooks.
   - Expose clean semantic state (`records`, `totalCount`, `isLoading`, `isError`, `refetch`, `createRecord`, `updateRecord`, `deleteRecord`) to the screen entry component.
2. **Feature Screen Entry (`src/features/[feature]/index.tsx`)**:
   - Reads validated URL search params (`useSearch`), invokes the domain hook, manages active overlay state (`openCreateDialog`, `editingRecord`, `deletingRecord`), and composes the page header, metric strip, table/empty branches, and overlays.
3. **Presentational Sub-Components (`components/[feature]-*.tsx`)**:
   - Receive typed domain objects, `form` instances, and callback props (`onSubmit`, `onOpenChange`, `onClearFilters`).
   - Remain free of direct HTTP client imports so they can be rendered deterministically in tests and Playwright stories.

---

## 5. React 19 Component & Prop Idioms (Zero `forwardRef`)

1. **No Unnecessary `React.forwardRef`**:
   - In React 19, `ref` is a standard prop on function components. Never wrap new components in `React.forwardRef`.
2. **Use `React.ComponentProps<"element">` or `React.ComponentProps<typeof Primitive>`**:
   - Type HTML/primitive extensions with `React.ComponentProps<"div">` (or `React.ComponentProps<typeof Button>`), destructuring `className` and merging classes with `cn(...)`:
     ```tsx
     export function MetricSummaryStrip({
       className,
       children,
       ...props
     }: React.ComponentProps<"section">) {
       return (
         <section
           data-slot="metric-summary-strip"
           className={cn("grid grid-cols-1 gap-4 md:grid-cols-3", className)}
           {...props}
         >
           {children}
         </section>
       );
     }
     ```
