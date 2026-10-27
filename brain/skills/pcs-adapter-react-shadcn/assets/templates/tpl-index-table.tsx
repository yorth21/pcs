/**
 * Blueprint: `tpl-index-table` — Index with Data Table (pcs-saas 01 §2)
 *
 * Domain: Invoices of a workspace (generic billing object).
 *
 * Catalog nodes composed here:
 *   org-page-header (default) · org-kpi-row (optional, filters the table) · org-filter-bar
 *   org-data-table · org-bulk-action-bar · mol-pagination · org-detail-panel (sheet, via ?inspect)
 *   org-empty-state (first-use, no-results) · org-section-error · org-banner (S6) · org-feedback (S7)
 *   org-destructive-confirm · mol-search-input · mol-facet-filter · mol-filter-chip
 *   mol-status-badge · mol-action-menu · mol-skeleton · mol-truncation
 *
 * Acceptance checks satisfied (search for the tag):
 *   AC-INDEX-1 URL serializes q, status, sort, dir, page, pageSize; reload restores the view.
 *   AC-INDEX-2 S2 and S3 render different copy and different actions.
 *   AC-INDEX-3 <= 7 default columns, identity first, numbers right-aligned with tabular-nums.
 *   AC-INDEX-4 Selecting rows shows the bulk bar with the count; clearing hides it.
 *   AC-INDEX-5 Returning from a record restores filters and page (URL state + scroll restoration).
 *
 * Split into a project (pcs-adapter 01 §1):
 *   src/routes/_app/invoices/index.tsx                         → Route (validateSearch: invoiceSearchSchema)
 *   src/features/billing/invoices/schemas/invoice-search.schema.ts
 *   src/features/billing/invoices/types/invoice.types.ts
 *   src/features/billing/invoices/hooks/use-invoices.ts       → invoiceKeys, useInvoices, useArchiveInvoices
 *   src/features/billing/invoices/components/invoice-*.tsx     → columns, toolbar, bulk bar, skeleton, states
 *   src/features/billing/invoices/index.tsx                   → InvoiceIndexScreen
 * Promote table/toolbar/pagination pieces to src/components/data-table/ only when a 2nd feature needs them.
 *
 * Route wiring (thin):
 *   export const Route = createFileRoute("/_app/invoices/")({
 *     validateSearch: invoiceSearchSchema,
 *     component: InvoiceIndexScreen,
 *   });
 *   Enable `scrollRestoration: true` on the router for AC-INDEX-5.
 */
import * as React from "react";
import { getRouteApi, Link } from "@tanstack/react-router";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type RowSelectionState,
} from "@tanstack/react-table";
import {
  Archive,
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  CircleAlert,
  Download,
  FileText,
  ListFilter,
  LockKeyhole,
  MoreHorizontal,
  Plus,
  Search,
  SearchX,
  TriangleAlert,
  Upload,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { z } from "zod";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
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
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group";
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Command,
  CommandGroup,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ApiError, apiClient } from "@/lib/api-client";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";

// -----------------------------------------------------------------------------
// Types and data contract (Page Composition Spec §3)
// -----------------------------------------------------------------------------

export type InvoiceStatus = "draft" | "open" | "paid" | "overdue" | "void";

export interface Invoice {
  id: string;
  number: string; // identity (Tier 1)
  customerName: string;
  status: InvoiceStatus;
  amountDue: number; // minor units handled by formatCurrency
  currency: string;
  dueDate: string; // ISO date
}

export interface InvoicePage {
  rows: Invoice[];
  total: number; // matches the active filters
  totalUnfiltered: number; // drives S2 vs S3
  capabilities: { canCreate: boolean; canArchive: boolean; canExport: boolean };
  staleSince?: string; // S6: present when results come from a lagging replica
}

// -----------------------------------------------------------------------------
// URL search state (AC-INDEX-1) — pcs-adapter 03 §1
// -----------------------------------------------------------------------------

const invoiceStatuses = ["draft", "open", "paid", "overdue", "void"] as const;

export const invoiceSearchSchema = z.object({
  q: z.string().trim().optional().catch(undefined),
  status: z.array(z.enum(invoiceStatuses)).default([]).catch([]),
  sort: z.enum(["number", "customerName", "amountDue", "dueDate"]).default("dueDate").catch("dueDate"),
  dir: z.enum(["asc", "desc"]).default("desc").catch("desc"),
  page: z.number().int().min(1).default(1).catch(1),
  pageSize: z.union([z.literal(10), z.literal(25), z.literal(50), z.literal(100)]).default(25).catch(25),
  inspect: z.string().optional().catch(undefined), // org-detail-panel sheet deep link
});

export type InvoiceSearch = z.infer<typeof invoiceSearchSchema>;

const routeApi = getRouteApi("/_app/invoices/");

// -----------------------------------------------------------------------------
// Query hooks (hooks/use-invoices.ts) — pcs-adapter 03 §2
// -----------------------------------------------------------------------------

export const invoiceKeys = {
  all: ["invoices"] as const,
  lists: () => [...invoiceKeys.all, "list"] as const,
  list: (search: Omit<InvoiceSearch, "inspect">) => [...invoiceKeys.lists(), search] as const,
};

export function useInvoices(search: InvoiceSearch) {
  const { inspect: _inspect, ...listParams } = search;
  return useQuery({
    queryKey: invoiceKeys.list(listParams),
    queryFn: () => apiClient.get<InvoicePage>("/invoices", { params: listParams }),
    placeholderData: keepPreviousData, // S1 refetch keeps stale rows visible
    retry: (count, error) => !(error instanceof ApiError && error.status === 403) && count < 2,
  });
}

export function useArchiveInvoices() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ids: string[]) => apiClient.post<void>("/invoices/archive", { ids }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: invoiceKeys.lists() }),
  });
}

function useRestoreInvoices() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ids: string[]) => apiClient.post<void>("/invoices/restore", { ids }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: invoiceKeys.lists() }),
  });
}

// -----------------------------------------------------------------------------
// mol-status-badge — one vocabulary per object, text always present
// -----------------------------------------------------------------------------

const invoiceStatusRole: Record<InvoiceStatus, { label: string; className: string }> = {
  draft: { label: "Draft", className: "text-muted-foreground" },
  open: { label: "Open", className: "border-info/30 bg-info/10 text-info" },
  paid: { label: "Paid", className: "border-success/30 bg-success/10 text-success" },
  overdue: { label: "Overdue", className: "border-destructive/30 bg-destructive/10 text-destructive" },
  void: { label: "Void", className: "text-muted-foreground" },
};

export function InvoiceStatusBadge({ status }: { status: InvoiceStatus }) {
  const role = invoiceStatusRole[status];
  return (
    <Badge variant="outline" className={cn("gap-1.5", role.className)}>
      <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
      {role.label}
    </Badge>
  );
}

// -----------------------------------------------------------------------------
// org-data-table columns (components/invoice-columns.tsx) — AC-INDEX-3: 6 default columns
// -----------------------------------------------------------------------------

function buildInvoiceColumns(options: {
  canArchive: boolean;
  onInspect: (invoice: Invoice) => void;
  onArchive: (invoices: Invoice[]) => void;
}): ColumnDef<Invoice>[] {
  return [
    {
      id: "select",
      header: ({ table }) => (
        <Checkbox
          aria-label="Select all invoices on this page"
          checked={
            table.getIsAllPageRowsSelected() ||
            (table.getIsSomePageRowsSelected() && "indeterminate")
          }
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(Boolean(value))}
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          aria-label={`Select invoice ${row.original.number}`}
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(Boolean(value))}
          onClick={(event) => event.stopPropagation()}
        />
      ),
      meta: { className: "w-10" },
    },
    {
      accessorKey: "number",
      header: "Invoice",
      enableSorting: true,
      cell: ({ row }) => (
        // Identity link: keyboard path to the record (row click is the pointer path).
        <button
          type="button"
          className="font-medium tabular-nums text-foreground underline-offset-4 hover:underline focus-visible:underline focus-visible:outline-none"
          onClick={(event) => {
            event.stopPropagation();
            options.onInspect(row.original);
          }}
        >
          {row.original.number}
        </button>
      ),
      meta: { className: "min-w-[200px]" },
    },
    {
      accessorKey: "customerName",
      header: "Customer",
      enableSorting: true,
      cell: ({ row }) => (
        <span className="block max-w-[280px] truncate" title={row.original.customerName}>
          {row.original.customerName}
        </span>
      ),
    },
    {
      accessorKey: "status",
      header: "Status",
      enableSorting: false,
      cell: ({ row }) => <InvoiceStatusBadge status={row.original.status} />,
    },
    {
      accessorKey: "amountDue",
      header: "Amount due",
      enableSorting: true,
      cell: ({ row }) => (
        <span className="block text-right tabular-nums">
          {formatCurrency(row.original.amountDue, row.original.currency)}
        </span>
      ),
      meta: { className: "text-right", numeric: true },
    },
    {
      accessorKey: "dueDate",
      header: "Due date",
      enableSorting: true,
      cell: ({ row }) => <span className="tabular-nums">{formatDate(row.original.dueDate)}</span>,
    },
    {
      id: "actions",
      header: () => <span className="sr-only">Actions</span>,
      cell: ({ row }) => (
        // mol-action-menu: 0 inline row actions here; everything in one overflow menu.
        <div className="flex justify-end" onClick={(event) => event.stopPropagation()}>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="size-8"
                aria-label={`Actions for invoice ${row.original.number}`}
              >
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuGroup>
                <DropdownMenuItem onSelect={() => options.onInspect(row.original)}>
                  View details
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <a href={`/api/invoices/${row.original.id}/pdf`} download>
                    Download PDF
                  </a>
                </DropdownMenuItem>
              </DropdownMenuGroup>
              {options.canArchive ? (
                <>
                  <DropdownMenuSeparator />
                  {/* Archive is reversible → undo toast, not a confirm dialog (pcs-saas 03 §15). */}
                  <DropdownMenuItem
                    variant="destructive"
                    onSelect={() => options.onArchive([row.original])}
                  >
                    Archive
                  </DropdownMenuItem>
                </>
              ) : null}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      ),
      meta: { className: "w-12 text-right" },
    },
  ];
}

type ColumnMeta = { className?: string; numeric?: boolean };

// -----------------------------------------------------------------------------
// S1 skeleton — real headers, 10 rows, geometry-matched (mol-skeleton)
// -----------------------------------------------------------------------------

function InvoiceRowsSkeleton({ columnCount }: { columnCount: number }) {
  return (
    <>
      {Array.from({ length: 10 }).map((_, rowIndex) => (
        <TableRow key={rowIndex} aria-hidden="true" className="h-10 hover:bg-transparent">
          {Array.from({ length: columnCount }).map((__, cellIndex) => (
            <TableCell key={cellIndex}>
              <Skeleton
                className={cn(
                  "h-4",
                  cellIndex === 0 ? "size-4" : cellIndex === columnCount - 1 ? "ml-auto size-6" : "w-full max-w-32",
                )}
              />
            </TableCell>
          ))}
        </TableRow>
      ))}
    </>
  );
}

// -----------------------------------------------------------------------------
// org-filter-bar: mol-search-input + mol-facet-filter + mol-filter-chip
// -----------------------------------------------------------------------------

function useDebouncedValue<T>(value: T, delayMs: number) {
  const [debounced, setDebounced] = React.useState(value);
  React.useEffect(() => {
    const handle = window.setTimeout(() => setDebounced(value), delayMs);
    return () => window.clearTimeout(handle);
  }, [value, delayMs]);
  return debounced;
}

function InvoiceSearchInput({ value, onChange }: { value: string; onChange: (q: string | undefined) => void }) {
  const [draft, setDraft] = React.useState(value);
  const debounced = useDebouncedValue(draft, 300);
  const inputRef = React.useRef<HTMLInputElement>(null);

  React.useEffect(() => {
    const next = debounced.trim();
    if (next.length === 1) return; // min 2 characters for remote search
    if (next !== value) onChange(next || undefined);
  }, [debounced, onChange, value]);

  // "/" focuses page search when no input is focused (mol-search-input).
  React.useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement;
      if (event.key === "/" && !target.closest("input, textarea, [contenteditable=true]")) {
        event.preventDefault();
        inputRef.current?.focus();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  return (
    <form role="search" className="w-full sm:w-72" onSubmit={(event) => event.preventDefault()}>
      <InputGroup>
        <InputGroupAddon>
          <Search className="size-4" aria-hidden="true" />
        </InputGroupAddon>
        <InputGroupInput
          ref={inputRef}
          type="search"
          aria-label="Search invoices"
          placeholder="Search by number or customer…"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Escape") return;
            if (draft) setDraft("");
            else event.currentTarget.blur();
          }}
        />
        {draft ? (
          <InputGroupAddon align="inline-end">
            <InputGroupButton size="icon-xs" aria-label="Clear search" onClick={() => setDraft("")}>
              <X className="size-3.5" />
            </InputGroupButton>
          </InputGroupAddon>
        ) : null}
      </InputGroup>
    </form>
  );
}

function StatusFacetFilter({
  selected,
  onChange,
}: {
  selected: InvoiceStatus[];
  onChange: (next: InvoiceStatus[]) => void;
}) {
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="border-dashed">
          <ListFilter className="size-4" aria-hidden="true" />
          Status
          {selected.length > 0 ? (
            <Badge variant="secondary" className="ml-1 tabular-nums">
              {selected.length === 1 ? invoiceStatusRole[selected[0]].label : `${selected.length} selected`}
            </Badge>
          ) : null}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-56 p-0" align="start">
        {/* <= 8 options → no CommandInput (mol-facet-filter). Multi-select applies immediately. */}
        <Command>
          <CommandList>
            <CommandGroup>
              {invoiceStatuses.map((status) => {
                const isSelected = selected.includes(status);
                return (
                  <CommandItem
                    key={status}
                    onSelect={() =>
                      onChange(isSelected ? selected.filter((s) => s !== status) : [...selected, status])
                    }
                  >
                    <Checkbox checked={isSelected} aria-hidden="true" tabIndex={-1} className="pointer-events-none" />
                    {invoiceStatusRole[status].label}
                  </CommandItem>
                );
              })}
            </CommandGroup>
            {selected.length > 0 ? (
              <CommandGroup>
                <CommandItem onSelect={() => onChange([])} className="justify-center">
                  Clear
                </CommandItem>
              </CommandGroup>
            ) : null}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

// -----------------------------------------------------------------------------
// Screen entry (index.tsx) — tpl-index-table
// -----------------------------------------------------------------------------

export function InvoiceIndexScreen() {
  const search = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const invoices = useInvoices(search);
  const archive = useArchiveInvoices();
  const restore = useRestoreInvoices();
  const [rowSelection, setRowSelection] = React.useState<RowSelectionState>({});
  const tableTopRef = React.useRef<HTMLDivElement>(null);

  // Every filter change resets page to 1 in the same navigation (org-filter-bar).
  const setSearch = React.useCallback(
    (patch: Partial<InvoiceSearch>, options: { resetPage?: boolean } = { resetPage: true }) =>
      navigate({
        search: (prev) => ({ ...prev, ...patch, ...(options.resetPage ? { page: 1 } : {}) }),
        replace: true,
      }),
    [navigate],
  );

  const hasActiveFilters = Boolean(search.q) || search.status.length > 0;
  const page = invoices.data;
  const rows = page?.rows ?? [];
  const capabilities = page?.capabilities ?? { canCreate: false, canArchive: false, canExport: false };

  const archiveWithUndo = React.useCallback(
    (targets: Invoice[]) => {
      const ids = targets.map((invoice) => invoice.id);
      archive.mutate(ids, {
        onSuccess: () => {
          setRowSelection({}); // selection clears after a bulk action completes
          // S7: reversible destructive → toast with Undo (org-feedback).
          toast(
            targets.length === 1 ? `Invoice ${targets[0].number} archived` : `${targets.length} invoices archived`,
            { action: { label: "Undo", onClick: () => restore.mutate(ids) }, duration: 6000 },
          );
        },
        onError: () =>
          toast.error("Couldn't archive invoices", {
            action: { label: "Retry", onClick: () => archiveWithUndo(targets) },
            duration: Infinity,
          }),
      });
    },
    [archive, restore],
  );

  const columns = React.useMemo(
    () =>
      buildInvoiceColumns({
        canArchive: capabilities.canArchive,
        onInspect: (invoice) => setSearch({ inspect: invoice.id }, { resetPage: false }),
        onArchive: archiveWithUndo,
      }).filter((column) => column.id !== "select" || capabilities.canArchive),
    [archiveWithUndo, capabilities.canArchive, setSearch],
  );

  const table = useReactTable({
    data: rows,
    columns,
    getRowId: (row) => row.id,
    getCoreRowModel: getCoreRowModel(),
    manualSorting: true,
    manualPagination: true,
    rowCount: page?.total ?? 0,
    state: { rowSelection },
    onRowSelectionChange: setRowSelection,
  });

  const selectedRows = table.getSelectedRowModel().rows.map((row) => row.original);

  // ---- S5 page-level no-access (tpl-system-page variant no-access inside the shell) ----
  if (invoices.error instanceof ApiError && invoices.error.status === 403) {
    return (
      <Empty className="min-h-[60vh]">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <LockKeyhole aria-hidden="true" />
          </EmptyMedia>
          <EmptyTitle>You don't have access to invoices</EmptyTitle>
          <EmptyDescription>Workspace admins and billing managers can grant access.</EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <Button asChild>
            <Link to="/">Go to dashboard</Link>
          </Button>
        </EmptyContent>
      </Empty>
    );
  }

  const isInitialLoading = invoices.isPending;
  const isZeroData = !isInitialLoading && !invoices.isError && page?.totalUnfiltered === 0 && !hasActiveFilters;

  return (
    <div className="mx-auto flex w-full max-w-[1600px] flex-col gap-6 px-4 py-6 lg:px-8">
      {/* org-page-header (default): 1 primary + secondary actions in overflow */}
      <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1">
          <h1 className="text-xl font-semibold tracking-tight text-foreground">
            Invoices
            {page ? (
              <span className="ml-2 text-base font-normal tabular-nums text-muted-foreground">
                {page.totalUnfiltered.toLocaleString()}
              </span>
            ) : null}
          </h1>
          <p className="text-sm text-muted-foreground">Bill customers and track what is still owed.</p>
        </div>
        <div className="flex items-center gap-2">
          {capabilities.canExport ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" size="icon" aria-label="More actions" disabled={isInitialLoading}>
                  <MoreHorizontal className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem>
                  <Upload className="size-4" aria-hidden="true" /> Import invoices…
                </DropdownMenuItem>
                <DropdownMenuItem>
                  <Download className="size-4" aria-hidden="true" /> Export invoices…
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
          {/* S5 action denied and never grantable → hidden, not disabled. */}
          {capabilities.canCreate ? (
            <Button asChild>
              <Link to="/invoices/new">
                <Plus className="size-4" aria-hidden="true" />
                Create invoice
              </Link>
            </Button>
          ) : null}
        </div>
      </header>

      {/* S6 partial/degraded: org-banner warning, not dismissible while the condition holds */}
      {page?.staleSince ? (
        <Alert role="status" className="border-warning/50 bg-warning/10 [&>svg]:text-warning">
          <TriangleAlert aria-hidden="true" />
          <AlertTitle>Results may be up to 10 minutes old</AlertTitle>
          <AlertDescription>Invoice sync is delayed. Creating and archiving still work.</AlertDescription>
        </Alert>
      ) : null}

      {isZeroData ? (
        // S2 empty first-use: replaces toolbar + table (AC-INDEX-2)
        <Empty className="min-h-[420px] rounded-lg border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <FileText aria-hidden="true" />
            </EmptyMedia>
            <EmptyTitle>No invoices yet</EmptyTitle>
            <EmptyDescription>Invoices let you bill customers and see who has paid at a glance.</EmptyDescription>
          </EmptyHeader>
          {capabilities.canCreate ? (
            <EmptyContent className="flex-row justify-center gap-2">
              <Button asChild>
                <Link to="/invoices/new">Create invoice</Link>
              </Button>
              <Button variant="outline">Import invoices</Button>
            </EmptyContent>
          ) : (
            <EmptyDescription>Ask a billing manager to create the first invoice.</EmptyDescription>
          )}
        </Empty>
      ) : (
        <section aria-labelledby="invoice-table-heading" className="flex flex-col gap-3">
          <h2 id="invoice-table-heading" className="sr-only">
            Invoice list
          </h2>

          {selectedRows.length > 0 ? (
            // org-bulk-action-bar replaces the filter row while >= 1 row is selected (AC-INDEX-4)
            <div role="region" aria-label="Bulk actions" className="flex min-h-9 flex-wrap items-center gap-2">
              <span role="status" className="text-sm font-medium tabular-nums">
                {selectedRows.length} selected
              </span>
              {table.getIsAllPageRowsSelected() && (page?.total ?? 0) > selectedRows.length ? (
                <Button variant="link" size="sm" className="px-1">
                  Select all {page?.total.toLocaleString()} invoices
                </Button>
              ) : null}
              <Button variant="outline" size="sm" onClick={() => archiveWithUndo(selectedRows)} disabled={archive.isPending}>
                {archive.isPending ? <Spinner /> : <Archive className="size-4" aria-hidden="true" />}
                Archive
              </Button>
              <Button variant="outline" size="sm">
                <Download className="size-4" aria-hidden="true" />
                Export
              </Button>
              <Button variant="ghost" size="sm" className="ml-auto" onClick={() => setRowSelection({})}>
                Clear selection
              </Button>
            </div>
          ) : (
            // org-filter-bar
            <div className="flex flex-col gap-2">
              <div className="flex flex-wrap items-center gap-2">
                <InvoiceSearchInput value={search.q ?? ""} onChange={(q) => setSearch({ q })} />
                <StatusFacetFilter selected={search.status} onChange={(status) => setSearch({ status })} />
                {invoices.isFetching && !invoices.isPending ? (
                  <Spinner className="ml-auto size-4 text-muted-foreground" aria-label="Updating results" />
                ) : null}
              </div>
              {hasActiveFilters ? (
                <div className="flex flex-wrap items-center gap-2">
                  {search.q ? (
                    <Badge variant="secondary" className="gap-1 pr-1">
                      Search: {search.q}
                      <button type="button" aria-label={`Remove filter Search: ${search.q}`} onClick={() => setSearch({ q: undefined })}>
                        <X className="size-3" />
                      </button>
                    </Badge>
                  ) : null}
                  {search.status.map((status) => (
                    <Badge key={status} variant="secondary" className="gap-1 pr-1">
                      Status: {invoiceStatusRole[status].label}
                      <button
                        type="button"
                        aria-label={`Remove filter Status: ${invoiceStatusRole[status].label}`}
                        onClick={() => setSearch({ status: search.status.filter((s) => s !== status) })}
                      >
                        <X className="size-3" />
                      </button>
                    </Badge>
                  ))}
                  <Button variant="ghost" size="sm" onClick={() => setSearch({ q: undefined, status: [] })}>
                    Clear filters
                  </Button>
                </div>
              ) : null}
              <p role="status" aria-live="polite" className="sr-only">
                {page ? `${page.total} results` : ""}
              </p>
            </div>
          )}

          {/* org-data-table — desktop/tablet table; mobile stacked list below sm */}
          <div ref={tableTopRef} className="hidden overflow-hidden rounded-lg border border-border bg-card sm:block">
            {invoices.isFetching && !invoices.isPending ? (
              <div aria-hidden="true" className="h-0.5 w-full animate-pulse bg-primary motion-reduce:animate-none" />
            ) : null}
            <Table aria-busy={isInitialLoading} aria-labelledby="invoice-table-heading">
              <TableHeader className="sticky top-0 z-10 bg-card">
                {table.getHeaderGroups().map((headerGroup) => (
                  <TableRow key={headerGroup.id} className="hover:bg-transparent">
                    {headerGroup.headers.map((header) => {
                      const meta = header.column.columnDef.meta as ColumnMeta | undefined;
                      const sortKey = header.column.id as InvoiceSearch["sort"];
                      const isSortable = header.column.columnDef.enableSorting === true;
                      const isActive = search.sort === sortKey;
                      return (
                        <TableHead
                          key={header.id}
                          scope="col"
                          className={meta?.className}
                          aria-sort={isSortable ? (isActive ? (search.dir === "asc" ? "ascending" : "descending") : "none") : undefined}
                        >
                          {isSortable ? (
                            <Button
                              variant="ghost"
                              size="sm"
                              className={cn("-ml-2 h-8", meta?.numeric && "ml-auto -mr-2 flex")}
                              onClick={() =>
                                setSearch({ sort: sortKey, dir: isActive && search.dir === "asc" ? "desc" : "asc" })
                              }
                            >
                              {flexRender(header.column.columnDef.header, header.getContext())}
                              {isActive ? (
                                search.dir === "asc" ? <ArrowUp className="size-3.5" aria-hidden="true" /> : <ArrowDown className="size-3.5" aria-hidden="true" />
                              ) : (
                                <ArrowUpDown className="size-3.5 opacity-40" aria-hidden="true" />
                              )}
                            </Button>
                          ) : (
                            flexRender(header.column.columnDef.header, header.getContext())
                          )}
                        </TableHead>
                      );
                    })}
                  </TableRow>
                ))}
              </TableHeader>
              <TableBody>
                {isInitialLoading ? (
                  <InvoiceRowsSkeleton columnCount={columns.length} />
                ) : invoices.isError ? (
                  // S4: error inside the table body; toolbar and headers stay usable
                  <TableRow className="hover:bg-transparent">
                    <TableCell colSpan={columns.length} className="h-64">
                      <Alert variant="destructive" role="status" className="mx-auto max-w-md">
                        <CircleAlert aria-hidden="true" />
                        <AlertTitle>Couldn't load invoices</AlertTitle>
                        <AlertDescription>
                          <p>Check your connection and try again.</p>
                          {invoices.error instanceof ApiError && invoices.error.referenceCode ? (
                            <p className="font-mono text-xs tabular-nums">Reference {invoices.error.referenceCode}</p>
                          ) : null}
                          <Button variant="outline" size="sm" className="mt-2" onClick={() => invoices.refetch()}>
                            Retry
                          </Button>
                        </AlertDescription>
                      </Alert>
                    </TableCell>
                  </TableRow>
                ) : rows.length === 0 ? (
                  // S3 no-results: names filters, offers Clear filters, never Create (AC-INDEX-2)
                  <TableRow className="hover:bg-transparent">
                    <TableCell colSpan={columns.length} className="h-64">
                      <Empty>
                        <EmptyHeader>
                          <EmptyMedia variant="icon">
                            <SearchX aria-hidden="true" />
                          </EmptyMedia>
                          <EmptyTitle>{search.q ? `No results for '${search.q}'` : "No invoices match your filters"}</EmptyTitle>
                          <EmptyDescription>Try a different search or remove a filter.</EmptyDescription>
                        </EmptyHeader>
                        <EmptyContent>
                          <Button variant="outline" onClick={() => setSearch({ q: undefined, status: [] })}>
                            Clear filters
                          </Button>
                        </EmptyContent>
                      </Empty>
                    </TableCell>
                  </TableRow>
                ) : (
                  table.getRowModel().rows.map((row) => (
                    <TableRow
                      key={row.id}
                      data-state={row.getIsSelected() ? "selected" : undefined}
                      className="h-10 cursor-pointer"
                      onClick={() => setSearch({ inspect: row.original.id }, { resetPage: false })}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <TableCell key={cell.id} className={(cell.column.columnDef.meta as ColumnMeta | undefined)?.className}>
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>

          {/* Mobile (< 640px): rows become stacked list items (identity, status, one metric, menu) */}
          <ul className="flex flex-col divide-y divide-border rounded-lg border border-border bg-card sm:hidden">
            {isInitialLoading
              ? Array.from({ length: 6 }).map((_, index) => (
                  <li key={index} aria-hidden="true" className="flex items-center justify-between gap-3 p-4">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-5 w-16 rounded-full" />
                  </li>
                ))
              : rows.map((invoice) => (
                  <li key={invoice.id} className="flex items-center gap-3 p-4">
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 flex-col items-start gap-1 text-left"
                      onClick={() => setSearch({ inspect: invoice.id }, { resetPage: false })}
                    >
                      <span className="font-medium tabular-nums">{invoice.number}</span>
                      <span className="truncate text-xs text-muted-foreground">{invoice.customerName}</span>
                    </button>
                    <div className="flex flex-col items-end gap-1">
                      <InvoiceStatusBadge status={invoice.status} />
                      <span className="text-sm tabular-nums">{formatCurrency(invoice.amountDue, invoice.currency)}</span>
                    </div>
                  </li>
                ))}
          </ul>

          {/* mol-pagination: page + page size in URL; scroll table top into view on change */}
          {page && page.total > 0 ? (
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <p className="text-sm tabular-nums text-muted-foreground">
                Showing {((search.page - 1) * search.pageSize + 1).toLocaleString()}–
                {Math.min(search.page * search.pageSize, page.total).toLocaleString()} of {page.total.toLocaleString()}
              </p>
              <div className="flex items-center gap-4">
                <Select
                  value={String(search.pageSize)}
                  onValueChange={(value) => setSearch({ pageSize: Number(value) as InvoiceSearch["pageSize"] })}
                >
                  <SelectTrigger size="sm" className="w-[120px]" aria-label="Rows per page">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {[10, 25, 50, 100].map((size) => (
                      <SelectItem key={size} value={String(size)}>
                        {size} per page
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Pagination className="mx-0 w-auto">
                  <PaginationContent>
                    <PaginationItem>
                      <PaginationPrevious
                        href="#"
                        aria-disabled={search.page <= 1}
                        className={cn(search.page <= 1 && "pointer-events-none opacity-50")}
                        onClick={(event) => {
                          event.preventDefault();
                          setSearch({ page: search.page - 1 }, { resetPage: false });
                          tableTopRef.current?.scrollIntoView({ block: "start" });
                        }}
                      />
                    </PaginationItem>
                    <PaginationItem>
                      <PaginationNext
                        href="#"
                        aria-disabled={search.page * search.pageSize >= page.total}
                        className={cn(search.page * search.pageSize >= page.total && "pointer-events-none opacity-50")}
                        onClick={(event) => {
                          event.preventDefault();
                          setSearch({ page: search.page + 1 }, { resetPage: false });
                          tableTopRef.current?.scrollIntoView({ block: "start" });
                        }}
                      />
                    </PaginationItem>
                  </PaginationContent>
                </Pagination>
              </div>
            </div>
          ) : null}
        </section>
      )}

      {/*
       * org-detail-panel (sheet): rendered from `search.inspect`; < 12 attributes → sheet,
       * otherwise navigate to tpl-record-detail. See tpl-list-detail.tsx for the panel body and
       * tpl-form-long.tsx (EditContactSheet) for the Sheet layout contract.
       */}
    </div>
  );
}
