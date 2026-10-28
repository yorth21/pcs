# TanStack Router, Query, Table and React Hook Form + Zod Patterns

This reference defines the concrete implementation contracts for `@tanstack/react-router` v1, `@tanstack/react-query` v5, `@tanstack/react-table` v8, and `react-hook-form` v7 + `zod` v4 inside `pcs-adapter-react-shadcn`.

---

## 1. `@tanstack/react-router` v1 — URL State & Search Params Contract

Treat the URL search string as the **single source of truth** for collection filters, pagination, active view tabs, and deep-linked side-sheet inspections:

```tsx
import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";
import { ProcedureCatalogScreen } from "@/features/procedures/catalog";

const procedureSearchSchema = z.object({
  q: z.string().optional().catch(undefined),
  status: z.enum(["all", "active", "archived"]).default("all").catch("all"),
  page: z.number().int().min(1).default(1).catch(1),
  pageSize: z.number().int().min(10).max(100).default(10).catch(10),
  inspectId: z.string().optional().catch(undefined),
});

export type ProcedureSearch = z.infer<typeof procedureSearchSchema>;

export const Route = createFileRoute("/_authenticated/procedures/")({
  validateSearch: procedureSearchSchema,
  component: ProcedureCatalogScreen,
});
```

### Router Rules
- Always attach `.default(...).catch(...)` on search-param fields so malformed URLs gracefully recover to valid defaults without throwing a route error screen.
- Reset `page: 1` whenever the user changes the search query `q` or a faceted filter (`status`) so they never land on an out-of-range page number.

---

## 2. `@tanstack/react-query` v5 — Async State, Skeletons & Partial Errors

1. **Hierarchical Query Key Factories**:
   - Define deterministic query keys per feature so mutations invalidate only the affected slice:
     ```ts
     export const procedureKeys = {
       all: ["procedures"] as const,
       lists: () => [...procedureKeys.all, "list"] as const,
       list: (params: ProcedureSearch) => [...procedureKeys.lists(), params] as const,
       details: () => [...procedureKeys.all, "detail"] as const,
       detail: (id: string) => [...procedureKeys.details(), id] as const,
     };
     ```
2. **State Mapping to the 5 Mandatory Negative States**:
   - `isPending` (initial load with no cached data): Render the feature's geometry-matched `<FeatureSkeleton />`.
   - `isFetching && !isPending` (background filter/page transition): Keep previous data visible (`placeholderData: keepPreviousData`) and display a subtle loading indicator in the toolbar.
   - `isError`: Render a section-scoped `<Alert variant="destructive">` with a `<Button variant="outline" size="sm" onClick={() => refetch()}>Retry</Button>`.
3. **Optimistic Updates & Reversible Toast Undo**:
   - For Risk Tier A (reversible toggles/archives), update cache or invalidate on success and emit a `sonner` toast with an explicit `Undo` callback that dispatches the inverse mutation.

---

## 3. `@tanstack/react-table` v8 — Collection & Empty State Separation

Every `DataTable` implementation MUST distinguish **Negative State #1 (`Zero-Data`)** from **Negative State #2 (`No-Results`)**:

```tsx
const hasActiveFilters = Boolean(search.q?.trim()) || search.status !== "all";
const isZeroData = !isPending && totalUnfilteredCount === 0 && !hasActiveFilters;
const isNoResults = !isPending && rows.length === 0 && (hasActiveFilters || totalUnfilteredCount > 0);
```

### Column Definition Rules (`[feature]-columns.tsx`)
1. **Column Count Cap (Miller's Law)**: Expose **5–7 visible columns maximum** by default.
2. **Leftmost Primary Identifier**: Column 0 (`code` or `name`) uses `font-medium text-foreground`.
3. **Right-Aligned Quantitative Columns**: All currency, count, and numeric columns use `text-right font-mono tabular-nums` or `text-right tabular-nums` in both `header` and `cell`.
4. **Single Row Action Menu (Hick's Law)**: Final column (`id: "actions"`) renders a single `<DropdownMenu>` trigger (`MoreHorizontal` icon with `aria-label="Open row actions"`). Separate destructive actions beneath `<DropdownMenuSeparator />`.

```tsx
import type { ColumnDef } from "@tanstack/react-table";
import { Badge } from "@/components/ui/badge";

export const procedureColumns: ColumnDef<ProcedureRecord>[] = [
  {
    accessorKey: "code",
    header: "Code",
    cell: ({ row }) => (
      <span className="font-medium tabular-nums text-foreground">{row.original.code}</span>
    ),
  },
  {
    accessorKey: "name",
    header: "Procedure Name",
  },
  {
    accessorKey: "isActive",
    header: "Status",
    cell: ({ row }) => (
      <Badge variant={row.original.isActive ? "default" : "secondary"}>
        {row.original.isActive ? "Active" : "Archived"}
      </Badge>
    ),
  },
  {
    accessorKey: "unitPrice",
    header: () => <div className="text-right">Unit Tariff</div>,
    cell: ({ row }) => (
      <div className="text-right font-medium tabular-nums text-foreground">
        ${row.original.unitPrice.toFixed(2)}
      </div>
    ),
  },
];
```

---

## 4. `react-hook-form` v7 + `zod` v4 — Validation & Dirty-Close Guard

1. **Actionable Schema Messages**:
   - Write specific, human-readable error strings in `zod`:
     ```ts
     export const procedureFormSchema = z.object({
       code: z.string().trim().min(3, "Enter a procedure code of at least 3 characters (e.g., PROC-101)."),
       name: z.string().trim().min(3, "Enter a descriptive procedure name."),
       unitPrice: z.coerce.number().min(0, "Unit price cannot be negative."),
     });
     ```
2. **Field & Accessibility Wiring (`Field`, `FieldLabel`, `FieldDescription`, `FieldError`)**:
   - Bind inputs via `<Controller>` with `@/components/ui/field` primitives (`Field`, `FieldLabel`, `FieldDescription`, `FieldError`) or shared `src/components/form/*` adapters (`FormInput`, `FormSelect`).
   - Always set `data-invalid={fieldState.invalid}` on `<Field>`, pass `aria-invalid={fieldState.invalid}` to the input control, and render `<FieldError errors={[fieldState.error]} />` directly below the control so screen readers and keyboard users receive immediate inline diagnosis:
     ```tsx
     <Controller
       control={form.control}
       name="code"
       render={({ field, fieldState }) => (
         <Field data-invalid={fieldState.invalid}>
           <FieldLabel htmlFor="procedure-code">Procedure Code</FieldLabel>
           <Input
             {...field}
             id="procedure-code"
             placeholder="e.g., PROC-104"
             className="tabular-nums"
             aria-invalid={fieldState.invalid}
           />
           <FieldDescription>
             Unique billing identifier used across invoices and schedules.
           </FieldDescription>
           {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
         </Field>
       )}
     />
     ```
3. **Dirty-Form Close Protection (Risk Tier B)**:
   - On any `Dialog` or `Sheet` containing a form, intercept `onOpenChange`:
     ```tsx
     const handleOpenChange = (nextOpen: boolean) => {
       if (!nextOpen && form.formState.isDirty && !form.formState.isSubmitting) {
         const shouldDiscard = window.confirm("Discard unsaved changes?");
         if (!shouldDiscard) return;
       }
       onOpenChange(nextOpen);
     };
     ```
   - Alternatively, trigger an inline confirmation alert when `isDirty` is true so users never lose typed work on an accidental backdrop click.

## Zod 4 Input and Output Types

When a schema uses `z.coerce`, `.default()`, `.catch()`, or `.transform()`, its input type differs from its output type, and `useForm<z.infer<typeof schema>>` with `zodResolver(schema)` fails to typecheck. Type the form with both:

```tsx
type FormInput = z.input<typeof schema>;   // what the fields hold
type FormValues = z.output<typeof schema>; // what onSubmit receives
const form = useForm<FormInput, unknown, FormValues>({ resolver: zodResolver(schema) });
```

Pass `UseFormReturn<FormInput, unknown, FormValues>` to child field components, store drafts as `Partial<FormInput>`, and render coerced fields with `value={String(field.value ?? "")}`.
