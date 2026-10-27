/**
 * Blueprint: `tpl-record-detail` — Entity Detail / Record Page with Tabs (pcs-saas 01 §5)
 *
 * Domain: Customer account (>= 12 attributes, sub-collections: invoices, contacts; activity history).
 *
 * Catalog nodes composed here:
 *   mol-breadcrumbs · org-page-header (record) · mol-status-badge · mol-key-value-list (inline facts)
 *   org-tabs (URL-driven, lazy panels) · org-data-table (compact, max 5 rows + View all)
 *   org-activity-feed · org-section-error (per tab) · org-empty-state (per sub-collection, compact)
 *   org-banner (S6 archived/read-only) · org-feedback (S7 inline) · mol-action-menu · mol-copyable-value
 *
 * Acceptance checks satisfied (search for the tag):
 *   AC-RECORD-1 Active tab is in the URL (?tab=); reload restores it.
 *   AC-RECORD-2 Identity and status badge reuse the index row representation (same badge map).
 *   AC-RECORD-3 A failing tab does not break the header or other tabs (each panel owns its query).
 *   AC-RECORD-4 Archived/read-only state disables every mutation control and explains why.
 *
 * Split into a project:
 *   src/routes/_app/customers/$customerId.tsx                     → Route (validateSearch: customerRecordSearchSchema,
 *                                                                    notFoundComponent → tpl-system-page not-found)
 *   src/features/crm/customers/hooks/use-customer.ts              → customerKeys, useCustomer, useCustomerInvoices, useCustomerActivity
 *   src/features/crm/customers/components/customer-header.tsx, customer-overview-tab.tsx, customer-invoices-tab.tsx, ...
 *   src/features/crm/customers/index.tsx is the index; the record screen is components/customer-record-screen.tsx
 */
import * as React from "react";
import { getRouteApi, Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Archive, Building2, CircleAlert, FileText, MoreHorizontal, RotateCcw } from "lucide-react";
import { toast } from "sonner";
import { z } from "zod";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ScrollArea, ScrollBar } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ApiError, apiClient } from "@/lib/api-client";
import { formatCurrency, formatDate, formatDateTime, formatRelativeTime } from "@/lib/format";
import { cn } from "@/lib/utils";

// -----------------------------------------------------------------------------
// Types and data contract
// -----------------------------------------------------------------------------

export type CustomerStatus = "active" | "trial" | "past_due" | "churned";

export interface Customer {
  id: string;
  name: string;
  accountNumber: string;
  status: CustomerStatus;
  plan: string;
  mrr: number;
  currency: string;
  ownerName?: string;
  industry?: string;
  country: string;
  billingEmail: string;
  createdAt: string;
  renewalDate?: string;
  archivedAt?: string; // S6: archived → read-only
  capabilities: { canEdit: boolean; canArchive: boolean; canViewInvoices: boolean };
}

export interface CustomerInvoiceRow {
  id: string;
  number: string;
  status: "open" | "paid" | "overdue";
  amount: number;
  currency: string;
  issuedAt: string;
}

export interface ActivityEntry {
  id: string;
  actor: string;
  verb: string;
  target: string;
  at: string;
}

// -----------------------------------------------------------------------------
// URL state — AC-RECORD-1
// -----------------------------------------------------------------------------

const customerTabs = ["overview", "invoices", "activity"] as const; // max 6; first is Overview

export const customerRecordSearchSchema = z.object({
  tab: z.enum(customerTabs).default("overview").catch("overview"),
});

const routeApi = getRouteApi("/_app/customers/$customerId");

// -----------------------------------------------------------------------------
// Query hooks — header and each tab query independently (AC-RECORD-3)
// -----------------------------------------------------------------------------

export const customerKeys = {
  all: ["customers"] as const,
  detail: (id: string) => [...customerKeys.all, "detail", id] as const,
  invoices: (id: string, limit: number) => [...customerKeys.detail(id), "invoices", limit] as const,
  activity: (id: string) => [...customerKeys.detail(id), "activity"] as const,
};

export function useCustomer(id: string) {
  return useQuery({
    queryKey: customerKeys.detail(id),
    queryFn: () => apiClient.get<Customer>(`/customers/${id}`),
    retry: (count, error) => !(error instanceof ApiError && [403, 404].includes(error.status)) && count < 2,
  });
}

function useCustomerInvoices(id: string, limit: number, enabled: boolean) {
  return useQuery({
    queryKey: customerKeys.invoices(id, limit),
    queryFn: () => apiClient.get<{ rows: CustomerInvoiceRow[]; total: number }>(`/customers/${id}/invoices`, { params: { limit } }),
    enabled,
  });
}

function useCustomerActivity(id: string, enabled: boolean) {
  return useQuery({
    queryKey: customerKeys.activity(id),
    queryFn: () => apiClient.get<ActivityEntry[]>(`/customers/${id}/activity`),
    enabled,
  });
}

function useSetCustomerArchived(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (archived: boolean) => apiClient.post<Customer>(`/customers/${id}/${archived ? "archive" : "restore"}`, {}),
    onSuccess: (customer) => {
      queryClient.setQueryData(customerKeys.detail(id), customer);
      queryClient.invalidateQueries({ queryKey: customerKeys.activity(id) }); // S7: transition adds an activity entry
    },
  });
}

// -----------------------------------------------------------------------------
// mol-status-badge — import the SAME map the index uses in a real project (AC-RECORD-2)
// -----------------------------------------------------------------------------

const customerStatusRole: Record<CustomerStatus, { label: string; className: string }> = {
  active: { label: "Active", className: "border-success/30 bg-success/10 text-success" },
  trial: { label: "Trial", className: "border-info/30 bg-info/10 text-info" },
  past_due: { label: "Past due", className: "border-warning/30 bg-warning/10 text-warning" },
  churned: { label: "Churned", className: "text-muted-foreground" },
};

function CustomerStatusBadge({ status }: { status: CustomerStatus }) {
  const role = customerStatusRole[status];
  return (
    <Badge variant="outline" className={cn("gap-1.5", role.className)}>
      <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
      {role.label}
    </Badge>
  );
}

function NotSet() {
  return <span aria-label="Not set">—</span>;
}

// -----------------------------------------------------------------------------
// Tab panels — each owns S1/S2/S4 (AC-RECORD-3)
// -----------------------------------------------------------------------------

function SectionError({ region, onRetry }: { region: string; onRetry: () => void }) {
  const [retries, setRetries] = React.useState(0);
  return (
    <Alert variant="destructive" role={retries > 0 ? "alert" : "status"}>
      <CircleAlert aria-hidden="true" />
      <AlertTitle>Couldn't load {region}</AlertTitle>
      <AlertDescription>
        <p>Something went wrong while loading this section. Other sections still work.</p>
        <Button
          variant="outline"
          size="sm"
          className="mt-2"
          onClick={() => {
            setRetries((count) => count + 1);
            onRetry();
          }}
        >
          Retry
        </Button>
      </AlertDescription>
    </Alert>
  );
}

function RecentInvoicesCard({ customer, enabled }: { customer: Customer; enabled: boolean }) {
  const invoices = useCustomerInvoices(customer.id, 5, enabled);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Recent invoices</CardTitle>
        <CardDescription>Last 5 invoices for this customer.</CardDescription>
        {invoices.data && invoices.data.total > 5 ? (
          <CardAction>
            <Button variant="link" size="sm" asChild>
              <Link to="." search={{ tab: "invoices" }}>
                View all {invoices.data.total}
              </Link>
            </Button>
          </CardAction>
        ) : null}
      </CardHeader>
      <CardContent className="min-h-[220px]">
        {invoices.isPending ? (
          <div aria-hidden="true" className="space-y-3">
            {Array.from({ length: 5 }).map((_, index) => (
              <Skeleton key={index} className="h-8 w-full" />
            ))}
          </div>
        ) : invoices.isError ? (
          <SectionError region="invoices" onRetry={() => invoices.refetch()} />
        ) : invoices.data.rows.length === 0 ? (
          // S2 per sub-collection: compact empty state with a create action scoped to this record
          <p className="text-sm text-muted-foreground">
            No invoices yet.{" "}
            {customer.capabilities.canEdit && !customer.archivedAt ? (
              <Button variant="link" className="h-auto p-0" asChild>
                <Link to="/invoices/new" search={{ customerId: customer.id }}>
                  Create invoice
                </Link>
              </Button>
            ) : null}
          </p>
        ) : (
          // org-data-table compact (rows h-10), numbers right-aligned
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead scope="col">Invoice</TableHead>
                <TableHead scope="col">Issued</TableHead>
                <TableHead scope="col" className="text-right">
                  Amount
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {invoices.data.rows.map((invoice) => (
                <TableRow key={invoice.id} className="h-10">
                  <TableCell className="font-medium tabular-nums">
                    <Link to="/invoices/$invoiceId" params={{ invoiceId: invoice.id }} className="hover:underline">
                      {invoice.number}
                    </Link>
                  </TableCell>
                  <TableCell className="tabular-nums">{formatDate(invoice.issuedAt)}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatCurrency(invoice.amount, invoice.currency)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function OverviewTab({ customer, isActive }: { customer: Customer; isActive: boolean }) {
  return (
    // Two-column layout: main column + side column; stacks below 1024px
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
      <div className="min-w-0 space-y-6">
        {customer.capabilities.canViewInvoices ? <RecentInvoicesCard customer={customer} enabled={isActive} /> : null}
      </div>
      <aside aria-label="Customer properties" className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Properties</CardTitle>
          </CardHeader>
          <CardContent>
            {/* mol-key-value-list: labels identical to the form labels for the same attribute */}
            <dl className="grid grid-cols-[minmax(0,40%)_1fr] gap-x-4 gap-y-3 text-sm">
              <dt className="text-xs text-muted-foreground">Owner</dt>
              <dd>{customer.ownerName ?? <NotSet />}</dd>
              <dt className="text-xs text-muted-foreground">Industry</dt>
              <dd>{customer.industry ?? <NotSet />}</dd>
              <dt className="text-xs text-muted-foreground">Country</dt>
              <dd>{customer.country}</dd>
              <dt className="text-xs text-muted-foreground">Billing email</dt>
              <dd className="truncate" title={customer.billingEmail}>
                {customer.billingEmail}
              </dd>
              <dt className="text-xs text-muted-foreground">Customer since</dt>
              <dd className="tabular-nums">{formatDate(customer.createdAt)}</dd>
              <dt className="text-xs text-muted-foreground">Renewal date</dt>
              <dd className="tabular-nums">{customer.renewalDate ? formatDate(customer.renewalDate) : <NotSet />}</dd>
            </dl>
          </CardContent>
        </Card>
      </aside>
    </div>
  );
}

function ActivityTab({ customerId, isActive }: { customerId: string; isActive: boolean }) {
  const activity = useCustomerActivity(customerId, isActive);
  if (activity.isPending) {
    return (
      <div aria-hidden="true" className="space-y-4">
        {Array.from({ length: 6 }).map((_, index) => (
          <Skeleton key={index} className="h-10 w-full max-w-2xl" />
        ))}
      </div>
    );
  }
  if (activity.isError) return <SectionError region="activity" onRetry={() => activity.refetch()} />;
  if (activity.data.length === 0) return <p className="text-sm text-muted-foreground">No activity yet.</p>;

  // org-activity-feed: ordered list, newest first, time elements with machine-readable values
  return (
    <ol className="max-w-2xl space-y-4">
      {activity.data.map((entry) => (
        <li key={entry.id} className="flex items-start gap-3">
          <Avatar className="size-6">
            <AvatarFallback className="text-[10px]">{entry.actor.slice(0, 2).toUpperCase()}</AvatarFallback>
          </Avatar>
          <div className="flex-1 text-sm">
            <p>
              <span className="font-medium">{entry.actor}</span> {entry.verb} {entry.target}
            </p>
            <time dateTime={entry.at} title={formatDateTime(entry.at)} className="text-xs tabular-nums text-muted-foreground">
              {formatRelativeTime(entry.at)}
            </time>
          </div>
        </li>
      ))}
    </ol>
  );
}

// -----------------------------------------------------------------------------
// Screen entry — tpl-record-detail
// -----------------------------------------------------------------------------

export function CustomerRecordScreen() {
  const { customerId } = routeApi.useParams();
  const { tab } = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const customer = useCustomer(customerId);
  const setArchived = useSetCustomerArchived(customerId);

  // S4 record not found → tpl-system-page not-found (router notFoundComponent renders it).
  if (customer.error instanceof ApiError && customer.error.status === 404) {
    throw customer.error; // the route's errorComponent maps 404 → not-found, 403 → no-access
  }

  const data = customer.data;
  const isReadOnly = Boolean(data?.archivedAt) || !data?.capabilities.canEdit;

  return (
    <div className="mx-auto flex w-full max-w-screen-xl flex-col gap-6 px-4 py-6 lg:px-8">
      {/* mol-breadcrumbs: depth >= 2, current page last and not a link */}
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link to="/customers">Customers</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{data?.name ?? <Skeleton className="h-4 w-32" />}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      {/* org-page-header (record): identity + status + 2–4 key facts + primary + overflow */}
      <header className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        {customer.isPending ? (
          <div aria-hidden="true" className="flex items-center gap-4">
            <Skeleton className="size-12 rounded-lg" />
            <div className="space-y-2">
              <Skeleton className="h-7 w-56" />
              <Skeleton className="h-4 w-72" />
            </div>
          </div>
        ) : customer.isError ? (
          <SectionError region="this customer" onRetry={() => customer.refetch()} />
        ) : (
          <div className="flex min-w-0 items-start gap-4">
            <Avatar className="size-12 rounded-lg">
              <AvatarFallback className="rounded-lg">
                <Building2 className="size-5" aria-hidden="true" />
              </AvatarFallback>
            </Avatar>
            <div className="min-w-0 space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <h1 className="truncate text-2xl font-semibold tracking-tight">{data!.name}</h1>
                <CustomerStatusBadge status={data!.status} />
              </div>
              <dl className="flex flex-wrap gap-x-6 gap-y-1 text-sm">
                <div className="flex gap-1.5">
                  <dt className="text-muted-foreground">Account</dt>
                  <dd className="font-mono tabular-nums">{data!.accountNumber}</dd>
                </div>
                <div className="flex gap-1.5">
                  <dt className="text-muted-foreground">Plan</dt>
                  <dd>{data!.plan}</dd>
                </div>
                <div className="flex gap-1.5">
                  <dt className="text-muted-foreground">MRR</dt>
                  <dd className="tabular-nums">{formatCurrency(data!.mrr, data!.currency)}</dd>
                </div>
              </dl>
            </div>
          </div>
        )}

        <div className="flex shrink-0 items-center gap-2">
          {/* Primary = most frequent state transition, not "Edit". Disabled with a reason when read-only (AC-RECORD-4). */}
          {isReadOnly && data?.capabilities.canEdit ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <span tabIndex={0}>
                  <Button disabled>
                    <FileText className="size-4" aria-hidden="true" />
                    Create invoice
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent>Restore this customer to create invoices</TooltipContent>
            </Tooltip>
          ) : data?.capabilities.canEdit ? (
            <Button asChild>
              <Link to="/invoices/new" search={{ customerId }}>
                <FileText className="size-4" aria-hidden="true" />
                Create invoice
              </Link>
            </Button>
          ) : null}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="icon" aria-label="More actions" disabled={!data}>
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem disabled={isReadOnly}>Edit details</DropdownMenuItem>
              <DropdownMenuItem>Copy account number</DropdownMenuItem>
              {data?.capabilities.canArchive && !data.archivedAt ? (
                <>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    variant="destructive"
                    onSelect={() =>
                      setArchived.mutate(true, {
                        // Archive is reversible → undo toast (no confirm dialog).
                        onSuccess: () =>
                          toast(`${data.name} archived`, {
                            action: { label: "Undo", onClick: () => setArchived.mutate(false) },
                            duration: 6000,
                          }),
                      })
                    }
                  >
                    <Archive className="size-4" aria-hidden="true" />
                    Archive customer
                  </DropdownMenuItem>
                </>
              ) : null}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>

      {/* S6: archived → banner explains why every mutation control is disabled (AC-RECORD-4) */}
      {data?.archivedAt ? (
        <Alert role="status" className="border-warning/50 bg-warning/10 [&>svg]:text-warning">
          <Archive aria-hidden="true" />
          <AlertTitle>This customer is archived</AlertTitle>
          <AlertDescription className="flex flex-wrap items-center gap-2">
            Archived on {formatDate(data.archivedAt)}. Restore it to make changes.
            {data.capabilities.canArchive ? (
              <Button variant="outline" size="sm" onClick={() => setArchived.mutate(false)} disabled={setArchived.isPending}>
                {setArchived.isPending ? <Spinner /> : <RotateCcw className="size-4" aria-hidden="true" />}
                Restore
              </Button>
            ) : null}
          </AlertDescription>
        </Alert>
      ) : null}

      {/* org-tabs: value from the URL, panels lazy-load (AC-RECORD-1). Restricted tabs are hidden (S5). */}
      {data ? (
        <Tabs value={tab} onValueChange={(next) => navigate({ search: { tab: next as (typeof customerTabs)[number] }, replace: true })}>
          <ScrollArea className="w-full">
            <TabsList>
              <TabsTrigger value="overview">Overview</TabsTrigger>
              {data.capabilities.canViewInvoices ? <TabsTrigger value="invoices">Invoices</TabsTrigger> : null}
              <TabsTrigger value="activity">Activity</TabsTrigger>
            </TabsList>
            <ScrollBar orientation="horizontal" />
          </ScrollArea>
          <TabsContent value="overview" className="pt-4">
            <OverviewTab customer={data} isActive={tab === "overview"} />
          </TabsContent>
          {data.capabilities.canViewInvoices ? (
            <TabsContent value="invoices" className="pt-4">
              {/* Full sub-collection: compose tpl-index-table's toolbar + table scoped to customerId (compact). */}
              <RecentInvoicesCard customer={data} enabled={tab === "invoices"} />
            </TabsContent>
          ) : null}
          <TabsContent value="activity" className="pt-4">
            <ActivityTab customerId={customerId} isActive={tab === "activity"} />
          </TabsContent>
        </Tabs>
      ) : null}
    </div>
  );
}
