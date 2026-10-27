/**
 * Blueprint: `tpl-dashboard` — Dashboard / Overview (pcs-saas 01 §10)
 *
 * Domain: Account overview for a billing team ("how is the account doing this period").
 * Mirrors pcs-saas 04 worked example 3.4.
 *
 * Catalog nodes composed here:
 *   org-page-header (workspace name + period selector, no create primary) · org-kpi-row (4 tiles)
 *   attention list (compact org-data-table, max 5 rows + View all) · org-chart-panel x2
 *   org-activity-feed (last 10) · tpl-onboarding checklist variant (S2) · org-section-error (per widget)
 *   org-empty-state (compact, per widget) · mol-status-badge · mol-skeleton
 *
 * Ordering rule: top-left = most urgent actionable info (KPIs, then "Needs your action"), charts below.
 *
 * Acceptance checks satisfied (search for the tag):
 *   AC-DASH-1 Every KPI shows unit, period, and comparison baseline, and links to a filtered list.
 *   AC-DASH-2 Each widget renders its own loading, empty, and error state (DashboardWidget).
 *   AC-DASH-3 Widgets: KPI row + attention list + 2 charts + activity feed = 5 (<= 8); charts = 2 (<= 4).
 *
 * Split into a project:
 *   src/routes/_app/index.tsx                       → Route (validateSearch: dashboardSearchSchema)
 *   src/features/overview/hooks/use-overview.ts    → overviewKeys, useKpi, useOverdueInvoices, useRevenueByWeek, ...
 *   src/features/overview/components/kpi-tile.tsx, dashboard-widget.tsx, revenue-chart.tsx, ...
 *   src/features/overview/index.tsx                → OverviewDashboardScreen
 * Promote DashboardWidget to src/components/layout/ when a second feature builds a dashboard.
 */
import * as React from "react";
import { getRouteApi, Link } from "@tanstack/react-router";
import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { CircleCheck, Circle, CircleAlert, RotateCw, TrendingDown, TrendingUp } from "lucide-react";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { z } from "zod";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart";
import { Progress } from "@/components/ui/progress";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ApiError, apiClient } from "@/lib/api-client";
import {
  formatCompactNumber,
  formatCurrency,
  formatDate,
  formatDateTime,
  formatPercent,
  formatRelativeTime,
} from "@/lib/format";
import { cn } from "@/lib/utils";

// -----------------------------------------------------------------------------
// URL state — period selector
// -----------------------------------------------------------------------------

const periods = { "7d": "last 7 days", "30d": "last 30 days", "90d": "last 90 days" } as const;
type Period = keyof typeof periods;

export const dashboardSearchSchema = z.object({
  period: z.enum(["7d", "30d", "90d"]).default("30d").catch("30d"),
});

const routeApi = getRouteApi("/_app/");

// -----------------------------------------------------------------------------
// Data contract — every widget has its own endpoint, so each loads and fails alone
// -----------------------------------------------------------------------------

interface KpiResponse {
  value: number;
  previous: number; // explicit baseline: previous period of equal length
  unit: "currency" | "count";
  currency?: string;
  goodDirection: "up" | "down";
  updatedAt: string;
}

interface OverdueInvoice {
  id: string;
  number: string;
  customerName: string;
  amountDue: number;
  currency: string;
  daysOverdue: number;
}

interface WeeklyRevenuePoint {
  weekStart: string;
  revenue: number;
}

interface StatusCount {
  status: "draft" | "open" | "paid" | "overdue";
  count: number;
}

interface ActivityEntry {
  id: string;
  actor: string;
  sentence: string;
  at: string;
}

interface WorkspaceSetup {
  isNew: boolean; // S2: no data yet → onboarding checklist instead of widgets
  tasks: { id: string; label: string; done: boolean; to: string }[];
  access: { revenue: boolean; invoices: boolean; activity: boolean }; // S5: omit inaccessible widgets
}

interface WidgetEnvelope<T> {
  data: T;
  updatedAt: string; // S6: per-widget staleness
}

export const overviewKeys = {
  all: ["overview"] as const,
  widget: (name: string, period: Period) => [...overviewKeys.all, name, period] as const,
};

function useWidget<T>(name: string, period: Period, enabled = true) {
  return useQuery({
    queryKey: overviewKeys.widget(name, period),
    queryFn: () => apiClient.get<WidgetEnvelope<T>>(`/overview/${name}`, { params: { period } }),
    enabled,
    staleTime: 60_000,
    retry: (count, error) => !(error instanceof ApiError && error.status === 403) && count < 2,
  });
}

// -----------------------------------------------------------------------------
// DashboardWidget — owns S1 / S3 / S4 / S6 for one widget (AC-DASH-2)
// This is behaviour, not a pass-through wrapper: it centralizes the per-widget state contract.
// -----------------------------------------------------------------------------

function DashboardWidget<T>({
  title,
  description,
  query,
  isEmpty,
  emptyText,
  skeleton,
  action,
  className,
  children,
}: {
  title: string;
  description?: string;
  query: UseQueryResult<WidgetEnvelope<T>>;
  isEmpty: (data: T) => boolean;
  emptyText: string;
  skeleton: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
  children: (data: T) => React.ReactNode;
}) {
  const headingId = React.useId();
  return (
    <Card className={cn("flex flex-col", className)} aria-labelledby={headingId} role="region">
      <CardHeader>
        <CardTitle id={headingId} className="text-base">
          {title}
        </CardTitle>
        {description ? <CardDescription>{description}</CardDescription> : null}
        {action ? <CardAction>{action}</CardAction> : null}
      </CardHeader>
      <CardContent className="flex-1">
        {query.isPending ? (
          <div aria-hidden="true">{skeleton}</div>
        ) : query.isError ? (
          // S4: per-widget error with retry; siblings keep rendering
          <div role="status" className="flex h-full min-h-32 flex-col items-center justify-center gap-2 text-center">
            <CircleAlert className="size-5 text-destructive" aria-hidden="true" />
            <p className="text-sm">Couldn't load {title.toLowerCase()}</p>
            <Button variant="outline" size="sm" onClick={() => query.refetch()}>
              Retry
            </Button>
          </div>
        ) : isEmpty(query.data.data) ? (
          // S3: period with no data — compact one-line empty state
          <p className="flex h-full min-h-32 items-center justify-center text-sm text-muted-foreground">{emptyText}</p>
        ) : (
          children(query.data.data)
        )}
      </CardContent>
      {query.data ? (
        <CardFooter>
          <p className="text-xs text-muted-foreground">
            Updated <time dateTime={query.data.updatedAt} title={formatDateTime(query.data.updatedAt)}>{formatRelativeTime(query.data.updatedAt)}</time>
          </p>
        </CardFooter>
      ) : null}
    </Card>
  );
}

// -----------------------------------------------------------------------------
// org-kpi-row tile — AC-DASH-1: unit, period, baseline, link to filtered list
// -----------------------------------------------------------------------------

function KpiTile({ label, period, endpoint, to, search }: { label: string; period: Period; endpoint: string; to: string; search: Record<string, string> }) {
  const kpi = useWidget<KpiResponse>(endpoint, period);
  const periodLabel = periods[period];

  if (kpi.isPending) {
    return (
      <Card aria-hidden="true">
        <CardHeader className="gap-2">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-8 w-24" />
          <Skeleton className="h-3 w-40" />
        </CardHeader>
      </Card>
    );
  }

  if (kpi.isError) {
    return (
      <Card>
        <CardHeader>
          <CardDescription>
            {label} · {periodLabel}
          </CardDescription>
          <div role="status" className="flex items-center gap-2 text-sm">
            <CircleAlert className="size-4 text-destructive" aria-hidden="true" />
            Couldn't load
            <Button variant="ghost" size="icon" className="size-7" aria-label={`Retry loading ${label}`} onClick={() => kpi.refetch()}>
              <RotateCw className="size-3.5" />
            </Button>
          </div>
        </CardHeader>
      </Card>
    );
  }

  const { value, previous, unit, currency, goodDirection } = kpi.data.data;
  const delta = previous === 0 ? 0 : (value - previous) / Math.abs(previous);
  const isUp = delta >= 0;
  const isGood = (isUp && goodDirection === "up") || (!isUp && goodDirection === "down");
  const full = unit === "currency" ? formatCurrency(value, currency!) : value.toLocaleString();
  const display = unit === "currency" ? formatCurrency(value, currency!, { compact: value >= 1_000_000 }) : formatCompactNumber(value);
  const deltaText = `${isUp ? "up" : "down"} ${formatPercent(Math.abs(delta), { maximumFractionDigits: 1 })} vs previous ${periodLabel.replace("last ", "")}`;

  return (
    <Card className="relative transition-colors hover:border-foreground/20">
      <CardHeader className="gap-1">
        <CardDescription>
          {label} · {periodLabel}
        </CardDescription>
        <Tooltip>
          <TooltipTrigger asChild>
            <p className="text-2xl font-semibold tracking-tight tabular-nums">{display}</p>
          </TooltipTrigger>
          <TooltipContent>{full}</TooltipContent>
        </Tooltip>
        {/* Direction in text + icon + sign + semantic color — never color alone */}
        <p className={cn("flex items-center gap-1 text-xs tabular-nums", isGood ? "text-success" : "text-destructive")}>
          {isUp ? <TrendingUp className="size-3.5" aria-hidden="true" /> : <TrendingDown className="size-3.5" aria-hidden="true" />}
          {isUp ? "+" : "−"}
          {formatPercent(Math.abs(delta), { maximumFractionDigits: 1 })}
          <span className="text-muted-foreground">vs previous {periodLabel.replace("last ", "")}</span>
        </p>
        {/* Whole tile is the link; accessible name carries metric, value, and delta */}
        <Link to={to} search={search} className="absolute inset-0 rounded-xl focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none" aria-label={`${label}: ${full}, ${deltaText}`} />
      </CardHeader>
    </Card>
  );
}

// -----------------------------------------------------------------------------
// Charts — org-chart-panel (question title, units, baseline at zero, text summary, View as table)
// -----------------------------------------------------------------------------

const revenueChartConfig = { revenue: { label: "Revenue", color: "var(--chart-1)" } } satisfies ChartConfig;
const statusChartConfig = { count: { label: "Invoices", color: "var(--chart-2)" } } satisfies ChartConfig;

function RevenueByWeekChart({ points, currency }: { points: WeeklyRevenuePoint[]; currency: string }) {
  const [asTable, setAsTable] = React.useState(false);
  const best = points.reduce((max, point) => (point.revenue > max.revenue ? point : max), points[0]);
  return (
    <figure className="space-y-3">
      <figcaption className="text-sm text-muted-foreground">
        Highest week: {formatDate(best.weekStart)} with {formatCurrency(best.revenue, currency)}.
      </figcaption>
      {asTable ? (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead scope="col">Week of</TableHead>
              <TableHead scope="col" className="text-right">
                Revenue ({currency})
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {points.map((point) => (
              <TableRow key={point.weekStart}>
                <TableCell className="tabular-nums">{formatDate(point.weekStart)}</TableCell>
                <TableCell className="text-right tabular-nums">{formatCurrency(point.revenue, currency)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      ) : (
        <ChartContainer config={revenueChartConfig} className="h-64 w-full">
          <BarChart data={points} accessibilityLayer>
            <CartesianGrid vertical={false} />
            <XAxis dataKey="weekStart" tickLine={false} axisLine={false} tickFormatter={(value: string) => formatDate(value, { month: "short", day: "numeric" })} />
            {/* Bars start at zero; axis labelled with unit */}
            <YAxis domain={[0, "auto"]} tickLine={false} axisLine={false} width={56} tickFormatter={(value: number) => formatCompactNumber(value)} label={{ value: currency, angle: -90, position: "insideLeft" }} />
            <ChartTooltip content={<ChartTooltipContent formatter={(value) => formatCurrency(Number(value), currency)} />} />
            <Bar dataKey="revenue" fill="var(--color-revenue)" radius={4} />
          </BarChart>
        </ChartContainer>
      )}
      <Button variant="link" size="sm" className="px-0" aria-pressed={asTable} onClick={() => setAsTable((value) => !value)}>
        {asTable ? "View as chart" : "View as table"}
      </Button>
    </figure>
  );
}

function InvoicesByStatusChart({ counts }: { counts: StatusCount[] }) {
  const sorted = [...counts].sort((a, b) => b.count - a.count); // ranking → horizontal bars sorted descending
  return (
    <figure className="space-y-3">
      <figcaption className="sr-only">
        {sorted.map((entry) => `${entry.status}: ${entry.count}`).join(", ")}
      </figcaption>
      <ChartContainer config={statusChartConfig} className="h-64 w-full">
        <BarChart data={sorted} layout="vertical" accessibilityLayer>
          <CartesianGrid horizontal={false} />
          <XAxis type="number" domain={[0, "auto"]} allowDecimals={false} tickLine={false} axisLine={false} />
          <YAxis type="category" dataKey="status" tickLine={false} axisLine={false} width={72} className="capitalize" />
          <ChartTooltip content={<ChartTooltipContent />} />
          {/* Category values get labels; chart palette, never status colors for non-status meaning */}
          <Bar dataKey="count" fill="var(--color-count)" radius={4} label={{ position: "right", className: "fill-foreground text-xs tabular-nums" }} />
        </BarChart>
      </ChartContainer>
    </figure>
  );
}

// -----------------------------------------------------------------------------
// Screen entry — tpl-dashboard
// -----------------------------------------------------------------------------

export function OverviewDashboardScreen({ workspaceName, currency }: { workspaceName: string; currency: string }) {
  const { period } = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const setup = useQuery({ queryKey: [...overviewKeys.all, "setup"], queryFn: () => apiClient.get<WorkspaceSetup>("/overview/setup") });
  const access = setup.data?.access;

  const overdue = useWidget<OverdueInvoice[]>("overdue-invoices", period, Boolean(access?.invoices));
  const revenue = useWidget<WeeklyRevenuePoint[]>("revenue-by-week", period, Boolean(access?.revenue));
  const statuses = useWidget<StatusCount[]>("invoices-by-status", period, Boolean(access?.invoices));
  const activity = useWidget<ActivityEntry[]>("activity", period, Boolean(access?.activity));

  return (
    <div className="mx-auto flex w-full max-w-[1440px] flex-col gap-6 px-4 py-6 lg:px-8">
      {/* org-page-header: workspace name + period selector at right; no create primary */}
      <header className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-1">
          <h1 className="text-xl font-semibold tracking-tight">{workspaceName}</h1>
          <p className="text-sm text-muted-foreground">What needs attention and how billing is trending.</p>
        </div>
        <Select value={period} onValueChange={(next) => navigate({ search: { period: next as Period }, replace: true })}>
          <SelectTrigger className="w-[180px]" aria-label="Period">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {(Object.keys(periods) as Period[]).map((key) => (
              <SelectItem key={key} value={key} className="capitalize">
                {periods[key]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </header>

      {setup.isPending ? (
        // S1 before access is known: KPI geometry only; header and period selector already rendered
        <div aria-hidden="true" className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-[116px] w-full rounded-xl" />
          ))}
        </div>
      ) : setup.isError ? (
        <Card role="status" className="items-center gap-2 py-10 text-center">
          <CircleAlert className="size-5 text-destructive" aria-hidden="true" />
          <p className="text-sm">Couldn't load your overview</p>
          <Button variant="outline" size="sm" onClick={() => setup.refetch()}>
            Retry
          </Button>
        </Card>
      ) : setup.data.isNew ? (
        // S2: new workspace → tpl-onboarding checklist variant replaces widgets
        <Card className="max-w-2xl">
          <CardHeader>
            <CardTitle>Get your workspace ready</CardTitle>
            <CardDescription>
              <span className="tabular-nums">{setup.data.tasks.filter((task) => task.done).length}</span> of{" "}
              <span className="tabular-nums">{setup.data.tasks.length}</span> done
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <Progress
              value={(setup.data.tasks.filter((task) => task.done).length / setup.data.tasks.length) * 100}
              aria-label="Setup progress"
              aria-valuetext={`${setup.data.tasks.filter((task) => task.done).length} of ${setup.data.tasks.length} done`}
            />
            <ul className="space-y-2">
              {setup.data.tasks.map((task) => (
                <li key={task.id}>
                  <Link to={task.to} className="flex items-center gap-3 rounded-md p-2 text-sm hover:bg-muted">
                    {task.done ? <CircleCheck className="size-4 text-success" aria-hidden="true" /> : <Circle className="size-4 text-muted-foreground" aria-hidden="true" />}
                    <span className={cn(task.done && "text-muted-foreground line-through")}>{task.label}</span>
                    <span className="sr-only">{task.done ? "(done)" : "(not done)"}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      ) : (
        <>
          {/* org-kpi-row: 4 tiles, 2-col on mobile, 4-col on desktop; omitted when not accessible (S5) */}
          {access?.revenue || access?.invoices ? (
            <section aria-label="Key metrics" className="grid grid-cols-2 gap-4 lg:grid-cols-4">
              {access.revenue ? <KpiTile label="Revenue" period={period} endpoint="kpi/revenue" to="/invoices" search={{ status: "paid" }} /> : null}
              {access.invoices ? <KpiTile label="Open invoices" period={period} endpoint="kpi/open-invoices" to="/invoices" search={{ status: "open" }} /> : null}
              {access.invoices ? <KpiTile label="Overdue amount" period={period} endpoint="kpi/overdue-amount" to="/invoices" search={{ status: "overdue" }} /> : null}
              {access.revenue ? <KpiTile label="Active customers" period={period} endpoint="kpi/active-customers" to="/customers" search={{ status: "active" }} /> : null}
            </section>
          ) : null}

          <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
            {access?.invoices ? (
              // Attention list: "Needs your action", compact, max 5 rows + View all
              <DashboardWidget
                className="lg:col-span-12"
                title="Overdue invoices"
                description="Oldest first. Follow up to get paid."
                query={overdue}
                isEmpty={(rows) => rows.length === 0}
                emptyText="No overdue invoices. Nice work."
                action={
                  <Button variant="link" size="sm" asChild>
                    <Link to="/invoices" search={{ status: ["overdue"] }}>
                      View all
                    </Link>
                  </Button>
                }
                skeleton={
                  <div className="space-y-2">
                    {Array.from({ length: 5 }).map((_, index) => (
                      <Skeleton key={index} className="h-9 w-full" />
                    ))}
                  </div>
                }
              >
                {(rows) => (
                  <Table>
                    <TableHeader>
                      <TableRow className="hover:bg-transparent">
                        <TableHead scope="col">Invoice</TableHead>
                        <TableHead scope="col">Customer</TableHead>
                        <TableHead scope="col" className="text-right">
                          Days overdue
                        </TableHead>
                        <TableHead scope="col" className="text-right">
                          Amount due
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {rows.slice(0, 5).map((invoice) => (
                        <TableRow key={invoice.id} className="h-10">
                          <TableCell className="font-medium tabular-nums">
                            <Link to="/invoices/$invoiceId" params={{ invoiceId: invoice.id }} className="hover:underline">
                              {invoice.number}
                            </Link>
                          </TableCell>
                          <TableCell className="max-w-[240px] truncate">{invoice.customerName}</TableCell>
                          <TableCell className="text-right">
                            <Badge variant="outline" className="border-destructive/30 bg-destructive/10 tabular-nums text-destructive">
                              {invoice.daysOverdue} days
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right tabular-nums">{formatCurrency(invoice.amountDue, invoice.currency)}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </DashboardWidget>
            ) : null}

            {access?.revenue ? (
              <DashboardWidget
                className="lg:col-span-6"
                title="Is revenue growing week over week?"
                description={`Paid invoices, ${periods[period]}`}
                query={revenue}
                isEmpty={(points) => points.length === 0}
                emptyText={`No data for ${periods[period]}`}
                skeleton={<Skeleton className="h-64 w-full" />}
              >
                {(points) => <RevenueByWeekChart points={points} currency={currency} />}
              </DashboardWidget>
            ) : null}

            {access?.invoices ? (
              <DashboardWidget
                className="lg:col-span-6"
                title="Where are invoices stuck?"
                description={`Invoices by status, ${periods[period]}`}
                query={statuses}
                isEmpty={(counts) => counts.every((entry) => entry.count === 0)}
                emptyText={`No data for ${periods[period]}`}
                skeleton={<Skeleton className="h-64 w-full" />}
              >
                {(counts) => <InvoicesByStatusChart counts={counts} />}
              </DashboardWidget>
            ) : null}

            {access?.activity ? (
              // org-activity-feed: recent, max 10
              <DashboardWidget
                className="lg:col-span-12"
                title="Recent activity"
                query={activity}
                isEmpty={(entries) => entries.length === 0}
                emptyText="No activity yet"
                skeleton={
                  <div className="space-y-3">
                    {Array.from({ length: 5 }).map((_, index) => (
                      <Skeleton key={index} className="h-5 w-full max-w-xl" />
                    ))}
                  </div>
                }
              >
                {(entries) => (
                  <ol className="space-y-3">
                    {entries.slice(0, 10).map((entry) => (
                      <li key={entry.id} className="flex items-baseline justify-between gap-4 text-sm">
                        <p>
                          <span className="font-medium">{entry.actor}</span> {entry.sentence}
                        </p>
                        <time dateTime={entry.at} title={formatDateTime(entry.at)} className="shrink-0 text-xs tabular-nums text-muted-foreground">
                          {formatRelativeTime(entry.at)}
                        </time>
                      </li>
                    ))}
                  </ol>
                )}
              </DashboardWidget>
            ) : null}
          </div>
        </>
      )}
    </div>
  );
}
