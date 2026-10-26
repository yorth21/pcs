import * as React from "react";
import type { ColumnDef } from "@tanstack/react-table";
import {
  AlertCircle,
  FolderOpen,
  MoreHorizontal,
  Plus,
  RotateCcw,
  SearchX,
  Trash2,
} from "lucide-react";
import { Controller, type UseFormReturn } from "react-hook-form";
import { toast } from "sonner";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

/**
 * Product Composition System (`pcs`) — Canonical React 19 + shadcn/ui Blueprints
 *
 * Engineering rules demonstrated in this file:
 * 1. KISS & Vanilla shadcn/ui First: Composes Card, Dialog, Sheet, AlertDialog,
 *    Alert, ResizablePanelGroup, DropdownMenu, Badge, Button, Input, and Skeleton
 *    directly without redundant wrapper abstractions.
 * 2. Complete 5 Mandatory Negative States:
 *    - State #1: Zero-Data State (first-use onboarding CTA)
 *    - State #2: No-Results State (filtered/searched empty state with "Clear filters")
 *    - State #3: Contextual Loading Skeleton (geometry-matched, CLS = 0)
 *    - State #4: Partial / Section Error State (Alert + Retry button)
 *    - State #5: Inline Form Validation Error (aria-invalid + aria-describedby + inline error)
 * 3. Context-Dependent Container Selection:
 *    - Quick Atomic Dialog (1-3 fields, zero background context dependency)
 *    - Contextual Edit Sheet (4-12 fields, preserves background table scroll/filter state)
 *    - Master-Detail Split Workbench (ResizablePanelGroup for high-complexity comparison)
 * 4. Risk-Proportional Friction:
 *    - Reversible action -> immediate execution + sonner Undo toast
 *    - Dirty form dismissal -> unsaved changes guard
 *    - Destructive action -> AlertDialog naming target entity and permanent impact
 * 5. Numeric & Token Discipline:
 *    - tabular-nums applied to all counts, currencies, codes, and timestamps
 *    - Zero Atomic Design suffixes (Atom, Molecule, Organism, Template)
 */

export interface ProcedureRecord {
  id: string;
  code: string;
  name: string;
  category: string;
  unitPrice: number;
  isActive: boolean;
  updatedAt: string;
}

export interface ProcedureFormValues {
  code: string;
  name: string;
  category: string;
  unitPrice: number;
}

// -----------------------------------------------------------------------------
// 1. Contextual Loading State Blueprint (Negative State #3 — CLS = 0)
// -----------------------------------------------------------------------------

export function ProcedureCatalogSkeleton() {
  return (
    <div className="space-y-6" aria-busy="true" aria-label="Loading procedures">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-2">
          <Skeleton className="h-7 w-48" />
          <Skeleton className="h-4 w-80" />
        </div>
        <Skeleton className="h-9 w-40" />
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {Array.from({ length: 3 }).map((_, idx) => (
          <Card key={idx}>
            <CardHeader className="pb-2">
              <Skeleton className="h-4 w-28" />
              <Skeleton className="h-7 w-20" />
            </CardHeader>
          </Card>
        ))}
      </div>

      <div className="overflow-hidden rounded-lg border border-border bg-card">
        <div className="divide-y divide-border p-4">
          {Array.from({ length: 5 }).map((_, idx) => (
            <div key={idx} className="flex items-center justify-between py-3">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-4 w-48" />
              <Skeleton className="h-5 w-16 rounded-full" />
              <Skeleton className="h-4 w-20" />
              <Skeleton className="size-8 rounded-md" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// -----------------------------------------------------------------------------
// 2. Page Header + Clickable Metric Strip (`tabular-nums`) + State-Complete Collection
//    Explicitly separates Negative State #1 (Zero-Data) from #2 (No-Results) and #4 (Error)
// -----------------------------------------------------------------------------

export function ProcedureCatalogSurface({
  records,
  totalUnfilteredCount,
  activeFilterSummary,
  isLoading,
  isError,
  onRetry,
  onClearFilters,
  onOpenCreate,
  onInspectRow,
  onRequestDelete,
  onToggleArchive,
}: {
  records: ProcedureRecord[];
  totalUnfilteredCount: number;
  activeFilterSummary?: string;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  onClearFilters: () => void;
  onOpenCreate: () => void;
  onInspectRow: (record: ProcedureRecord) => void;
  onRequestDelete: (record: ProcedureRecord) => void;
  onToggleArchive: (record: ProcedureRecord) => Promise<void>;
}) {
  if (isLoading) {
    return <ProcedureCatalogSkeleton />;
  }

  const hasActiveFilters = Boolean(activeFilterSummary);
  const isZeroData = totalUnfilteredCount === 0 && !hasActiveFilters;
  const isNoResults = records.length === 0 && !isZeroData;

  const activeCount = records.filter((item) => item.isActive).length;
  const averagePrice =
    records.length > 0
      ? records.reduce((acc, item) => acc + item.unitPrice, 0) / records.length
      : 0;

  return (
    <div className="space-y-6">
      {/* Z-Pattern Header: Title + Description on left, 1 Primary CTA on right */}
      <header className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-1">
          <h1 className="text-xl font-semibold tracking-tight text-foreground">
            Clinical Procedures
          </h1>
          <p className="text-xs text-muted-foreground">
            Manage standardized procedure codes, operational categories, and base tariffs.
          </p>
        </div>
        <Button onClick={onOpenCreate}>
          <Plus className="mr-2 size-4" />
          Create procedure
        </Button>
      </header>

      {/* Metric Strip: Strict tabular-nums alignment on all quantitative metrics */}
      <section
        aria-label="Procedure summary metrics"
        className="grid grid-cols-1 gap-4 md:grid-cols-3"
      >
        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs">Total Procedures</CardDescription>
            <CardTitle className="text-2xl font-semibold tracking-tight tabular-nums">
              {totalUnfilteredCount}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs">Active in Catalog</CardDescription>
            <CardTitle className="text-2xl font-semibold tracking-tight tabular-nums">
              {activeCount}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs">Average Unit Tariff</CardDescription>
            <CardTitle className="text-2xl font-semibold tracking-tight tabular-nums">
              ${averagePrice.toFixed(2)}
            </CardTitle>
          </CardHeader>
        </Card>
      </section>

      {/* Negative State #4: Section-Scoped Partial Error with Retry */}
      {isError ? (
        <Alert variant="destructive">
          <AlertCircle className="size-4" />
          <AlertTitle>Unable to load procedure catalog</AlertTitle>
          <AlertDescription className="mt-2 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <span>
              A network error prevented fetching the latest procedures. Your filters are preserved.
            </span>
            <Button variant="outline" size="sm" onClick={onRetry}>
              <RotateCcw className="mr-2 size-3.5" />
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      ) : isZeroData ? (
        /* Negative State #1: Zero-Data State (First-Use / Empty Collection) */
        <Empty className="min-h-[300px] border border-dashed border-border bg-card/40">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <FolderOpen className="size-5" />
            </EmptyMedia>
            <EmptyTitle>No clinical procedures registered yet</EmptyTitle>
            <EmptyDescription>
              Create your first procedure code to enable scheduling, billing rules, and tariff assignment.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button size="sm" onClick={onOpenCreate}>
              <Plus className="mr-2 size-4" />
              Create first procedure
            </Button>
          </EmptyContent>
        </Empty>
      ) : isNoResults ? (
        /* Negative State #2: No-Results State (Filtered / Searched Empty) */
        <Empty className="min-h-[260px] border border-border bg-card">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <SearchX className="size-5" />
            </EmptyMedia>
            <EmptyTitle>No procedures match your active filters</EmptyTitle>
            <EmptyDescription>
              No matches found for {activeFilterSummary ?? "the current query"}. Try broadening your search or reset filters.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button variant="outline" size="sm" onClick={onClearFilters}>
              Clear filters
            </Button>
          </EmptyContent>
        </Empty>
      ) : (
        /* Happy Path: F-Pattern Table (<= 6 columns, right-aligned tabular-nums, 1 row action menu) */
        <div className="overflow-x-auto rounded-lg border border-border bg-card">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-3 font-medium">Code</th>
                <th className="px-4 py-3 font-medium">Procedure Name</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 text-right font-medium">Unit Tariff</th>
                <th className="px-4 py-3 text-right font-medium">Updated</th>
                <th className="w-12 px-4 py-3 text-right font-medium">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {records.map((row) => (
                <tr key={row.id} className="transition-colors hover:bg-muted/40">
                  <td className="px-4 py-3 font-medium tabular-nums text-foreground">
                    {row.code}
                  </td>
                  <td className="px-4 py-3 text-foreground">{row.name}</td>
                  <td className="px-4 py-3">
                    <Badge variant={row.isActive ? "default" : "secondary"}>
                      {row.isActive ? "Active" : "Archived"}
                    </Badge>
                  </td>
                  <td className="px-4 py-3 text-right font-medium tabular-nums text-foreground">
                    ${row.unitPrice.toFixed(2)}
                  </td>
                  <td className="px-4 py-3 text-right text-xs tabular-nums text-muted-foreground">
                    {row.updatedAt}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <ProcedureRowActions
                      record={row}
                      onEdit={() => onInspectRow(row)}
                      onToggleArchive={() => handleReversibleToggle(row, onToggleArchive)}
                      onDelete={() => onRequestDelete(row)}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// -----------------------------------------------------------------------------
// 3. Risk-Proportional Friction: Reversible Action with Sonner Undo vs Destructive AlertDialog
// -----------------------------------------------------------------------------

async function handleReversibleToggle(
  record: ProcedureRecord,
  onToggleArchive: (record: ProcedureRecord) => Promise<void>
) {
  await onToggleArchive(record);
  toast.success(
    record.isActive
      ? `Procedure ${record.code} archived`
      : `Procedure ${record.code} restored`,
    {
      action: {
        label: "Undo",
        onClick: () => {
          void onToggleArchive(record);
        },
      },
    }
  );
}

export function ProcedureRowActions({
  record,
  onEdit,
  onToggleArchive,
  onDelete,
}: {
  record: ProcedureRecord;
  onEdit: () => void;
  onToggleArchive: () => void;
  onDelete: () => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          aria-label={`Open actions for procedure ${record.code}`}
        >
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onEdit}>Edit in panel</DropdownMenuItem>
        <DropdownMenuItem onClick={onToggleArchive}>
          {record.isActive ? "Archive procedure" : "Restore procedure"}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onClick={onDelete}
          className="text-destructive focus:text-destructive"
        >
          <Trash2 className="mr-2 size-4" />
          Delete procedure
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function ProcedureDeleteAlertDialog({
  record,
  open,
  onOpenChange,
  onConfirmDelete,
}: {
  record: ProcedureRecord | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirmDelete: (record: ProcedureRecord) => void;
}) {
  if (!record) return null;

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            Delete procedure {record.code} ({record.name})?
          </AlertDialogTitle>
          <AlertDialogDescription>
            This action permanently removes {record.code} from the active catalog and cannot be
            undone. Historical invoices referencing this code will retain their snapshot values.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => onConfirmDelete(record)}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          >
            Delete procedure {record.code}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

// -----------------------------------------------------------------------------
// 4. Context-Dependent Containers: Quick Atomic Dialog vs Contextual Edit Sheet
//    Demonstrates Negative State #5 (Inline Validation Error + aria-invalid) & Dirty Guard
// -----------------------------------------------------------------------------

export function QuickCreateProcedureDialog({
  open,
  onOpenChange,
  form,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  form: UseFormReturn<ProcedureFormValues>;
  onSubmit: (values: ProcedureFormValues) => void;
}) {
  const handleSafeOpenChange = (nextOpen: boolean) => {
    if (!nextOpen && form.formState.isDirty && !form.formState.isSubmitting) {
      const discard = window.confirm("Discard unsaved procedure details?");
      if (!discard) return;
    }
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={handleSafeOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Quick Create Procedure</DialogTitle>
          <DialogDescription>
            Enter the standardized code, name, and base tariff to register a new procedure.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4" noValidate>
          <Controller
            control={form.control}
            name="code"
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid} className="gap-1.5">
                <FieldLabel htmlFor="dialog-procedure-code">Procedure Code</FieldLabel>
                <Input
                  {...field}
                  id="dialog-procedure-code"
                  placeholder="e.g., PROC-104"
                  className="tabular-nums"
                  aria-invalid={fieldState.invalid}
                  aria-describedby={fieldState.error ? "dialog-code-error" : "dialog-code-desc"}
                />
                <FieldDescription id="dialog-code-desc">
                  Unique billing reference used across schedules and claims.
                </FieldDescription>
                {fieldState.invalid && (
                  <FieldError id="dialog-code-error" errors={[fieldState.error]} />
                )}
              </Field>
            )}
          />

          <Controller
            control={form.control}
            name="name"
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid} className="gap-1.5">
                <FieldLabel htmlFor="dialog-procedure-name">Descriptive Name</FieldLabel>
                <Input
                  {...field}
                  id="dialog-procedure-name"
                  placeholder="e.g., Comprehensive Diagnostic Evaluation"
                  aria-invalid={fieldState.invalid}
                  aria-describedby={fieldState.error ? "dialog-name-error" : undefined}
                />
                {fieldState.invalid && (
                  <FieldError id="dialog-name-error" errors={[fieldState.error]} />
                )}
              </Field>
            )}
          />

          <DialogFooter className="gap-2 pt-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => handleSafeOpenChange(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={form.formState.isSubmitting}>
              {form.formState.isSubmitting ? "Saving..." : "Create procedure"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function ContextualProcedureEditSheet({
  open,
  onOpenChange,
  form,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  form: UseFormReturn<ProcedureFormValues>;
  onSubmit: (values: ProcedureFormValues) => void;
}) {
  const handleSafeOpenChange = (nextOpen: boolean) => {
    if (!nextOpen && form.formState.isDirty && !form.formState.isSubmitting) {
      const discard = window.confirm("Discard unsaved edits to this procedure?");
      if (!discard) return;
    }
    onOpenChange(nextOpen);
  };

  return (
    <Sheet open={open} onOpenChange={handleSafeOpenChange}>
      <SheetContent side="right" className="flex flex-col p-0 sm:max-w-lg">
        <SheetHeader className="border-b border-border px-6 py-4">
          <SheetTitle>Edit Procedure Details</SheetTitle>
          <SheetDescription>
            Update tariff and classification attributes while keeping the catalog table in view.
          </SheetDescription>
        </SheetHeader>

        <form
          onSubmit={form.handleSubmit(onSubmit)}
          className="flex flex-1 flex-col overflow-hidden"
          noValidate
        >
          <div className="grid flex-1 grid-cols-1 gap-4 overflow-y-auto px-6 py-4 md:grid-cols-12">
            <div className="md:col-span-5">
              <Controller
                control={form.control}
                name="code"
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid} className="gap-1.5">
                    <FieldLabel htmlFor="sheet-procedure-code">Code</FieldLabel>
                    <Input
                      {...field}
                      id="sheet-procedure-code"
                      className="tabular-nums"
                      aria-invalid={fieldState.invalid}
                      aria-describedby={fieldState.error ? "sheet-code-error" : undefined}
                    />
                    {fieldState.invalid && (
                      <FieldError id="sheet-code-error" errors={[fieldState.error]} />
                    )}
                  </Field>
                )}
              />
            </div>

            <div className="md:col-span-7">
              <Controller
                control={form.control}
                name="unitPrice"
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid} className="gap-1.5">
                    <FieldLabel htmlFor="sheet-procedure-price">Unit Tariff (USD)</FieldLabel>
                    <Input
                      {...field}
                      id="sheet-procedure-price"
                      type="number"
                      step="0.01"
                      className="text-right tabular-nums"
                      aria-invalid={fieldState.invalid}
                      aria-describedby={fieldState.error ? "sheet-price-error" : undefined}
                    />
                    {fieldState.invalid && (
                      <FieldError id="sheet-price-error" errors={[fieldState.error]} />
                    )}
                  </Field>
                )}
              />
            </div>

            <div className="md:col-span-12">
              <Controller
                control={form.control}
                name="name"
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid} className="gap-1.5">
                    <FieldLabel htmlFor="sheet-procedure-name">Procedure Name</FieldLabel>
                    <Input
                      {...field}
                      id="sheet-procedure-name"
                      aria-invalid={fieldState.invalid}
                      aria-describedby={fieldState.error ? "sheet-name-error" : undefined}
                    />
                    {fieldState.invalid && (
                      <FieldError id="sheet-name-error" errors={[fieldState.error]} />
                    )}
                  </Field>
                )}
              />
            </div>
          </div>

          <SheetFooter className="border-t border-border bg-muted/30 px-6 py-4">
            <Button
              type="button"
              variant="outline"
              onClick={() => handleSafeOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={!form.formState.isDirty || form.formState.isSubmitting}
            >
              {form.formState.isSubmitting ? "Saving changes..." : "Save changes"}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}

// -----------------------------------------------------------------------------
// 5. TanStack Table v8 Column Definitions + Master-Detail Split Workbench
//    Demonstrates React 19 `React.ComponentProps<"div">` (zero `forwardRef`)
// -----------------------------------------------------------------------------

export function createProcedureColumns({
  onInspectRow,
  onToggleArchive,
  onRequestDelete,
}: {
  onInspectRow: (record: ProcedureRecord) => void;
  onToggleArchive: (record: ProcedureRecord) => Promise<void>;
  onRequestDelete: (record: ProcedureRecord) => void;
}): ColumnDef<ProcedureRecord>[] {
  return [
    {
      accessorKey: "code",
      header: "Code",
      cell: ({ row }) => (
        <span className="font-medium tabular-nums text-foreground">
          {row.original.code}
        </span>
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
    {
      accessorKey: "updatedAt",
      header: () => <div className="text-right">Updated</div>,
      cell: ({ row }) => (
        <div className="text-right text-xs tabular-nums text-muted-foreground">
          {row.original.updatedAt}
        </div>
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="text-right">
          <ProcedureRowActions
            record={row.original}
            onEdit={() => onInspectRow(row.original)}
            onToggleArchive={() => handleReversibleToggle(row.original, onToggleArchive)}
            onDelete={() => onRequestDelete(row.original)}
          />
        </div>
      ),
    },
  ];
}

export function ProcedureReconciliationWorkbench({
  queuePanel,
  detailPanel,
  className,
  ...props
}: React.ComponentProps<"div"> & {
  queuePanel: React.ReactNode;
  detailPanel: React.ReactNode;
}) {
  return (
    <div
      data-slot="procedure-reconciliation-workbench"
      className={cn(
        "h-[calc(100vh-8rem)] overflow-hidden rounded-lg border border-border bg-card",
        className
      )}
      {...props}
    >
      <ResizablePanelGroup direction="horizontal">
        <ResizablePanel defaultSize={38} minSize={28} className="overflow-y-auto p-4">
          {queuePanel}
        </ResizablePanel>
        <ResizableHandle withHandle />
        <ResizablePanel defaultSize={62} minSize={40} className="overflow-y-auto p-6">
          {detailPanel}
        </ResizablePanel>
      </ResizablePanelGroup>
    </div>
  );
}
