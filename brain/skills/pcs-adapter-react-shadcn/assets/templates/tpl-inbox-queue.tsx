/**
 * Blueprint: `tpl-inbox-queue` — Inbox / Approval Queue (pcs-saas 01 §11)
 *
 * Domain: Expense report approvals. Each item needs a decision by the current user; throughput matters.
 *
 * Catalog nodes composed here:
 *   org-page-header (compact) + org-tabs (queue counts per status, max 5)
 *   tpl-list-detail split: queue list (explicit sort label) + org-detail-panel (inline) with full context
 *   decision bar pinned in the detail pane (primary = most common decision; reason required to reject)
 *   org-empty-state (done = inbox zero, no-results) · org-banner (S6 "New items available")
 *   org-feedback (toast with Undo) · mol-inline-alert (decision failure) · mol-status-badge · mol-tooltip + Kbd
 *
 * Acceptance checks satisfied (search for the tag):
 *   AC-QUEUE-1 After a decision the next item opens automatically and Undo is offered.
 *   AC-QUEUE-2 Empty pending state is distinct from no-results and has no create action.
 *   AC-QUEUE-3 Every decision is reachable by keyboard (buttons + single-key shortcuts shown in tooltips).
 *
 * Anti-patterns avoided: no confirmation dialog for routine approvals; the list never reorders under the
 * cursor (new items wait behind the S6 banner); decisions are not hidden in row menus.
 *
 * Split into a project:
 *   src/routes/_app/approvals/index.tsx                     → Route (validateSearch: approvalSearchSchema)
 *   src/features/expenses/approvals/hooks/use-approvals.ts  → approvalKeys, useApprovalQueue, useDecide, useUndoDecision
 *   src/features/expenses/approvals/components/approval-list.tsx, approval-detail.tsx, decision-bar.tsx
 *   src/features/expenses/approvals/index.tsx               → ApprovalQueueScreen
 */
import * as React from "react";
import { getRouteApi, Link } from "@tanstack/react-router";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, CircleAlert, CircleCheck, Clock, PartyPopper, SearchX } from "lucide-react";
import { toast } from "sonner";
import { z } from "zod";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { Kbd } from "@/components/ui/kbd";
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "@/components/ui/resizable";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ApiError, apiClient } from "@/lib/api-client";
import { formatCurrency, formatDate, formatDateTime, formatRelativeTime } from "@/lib/format";
import { cn } from "@/lib/utils";

// -----------------------------------------------------------------------------
// Types and data contract
// -----------------------------------------------------------------------------

type QueueStatus = "pending" | "approved" | "rejected";
type Decision = "approve" | "reject";

export interface ExpenseReportSummary {
  id: string;
  title: string;
  submitterName: string;
  total: number;
  currency: string;
  submittedAt: string;
  waitingOnRole?: string; // S5: visible read-only when another role must decide
}

export interface ExpenseReportDetail extends ExpenseReportSummary {
  reference: string;
  costCenter: string;
  policyWarnings: string[];
  lines: { id: string; date: string; category: string; merchant: string; amount: number }[];
}

export interface QueueResponse {
  rows: ExpenseReportSummary[]; // sorted oldest-first (explicit sort label in the UI)
  counts: Record<QueueStatus, number>;
  totalUnfiltered: number;
  lastProcessedAt?: string;
}

// -----------------------------------------------------------------------------
// URL state
// -----------------------------------------------------------------------------

export const approvalSearchSchema = z.object({
  status: z.enum(["pending", "approved", "rejected"]).default("pending").catch("pending"),
  id: z.string().optional().catch(undefined),
  q: z.string().trim().optional().catch(undefined),
});

type ApprovalSearch = z.infer<typeof approvalSearchSchema>;

const routeApi = getRouteApi("/_app/approvals/");

// -----------------------------------------------------------------------------
// Query hooks
// -----------------------------------------------------------------------------

export const approvalKeys = {
  all: ["approvals"] as const,
  queue: (status: QueueStatus, q?: string) => [...approvalKeys.all, "queue", status, q ?? ""] as const,
  pendingCount: () => [...approvalKeys.all, "pending-count"] as const,
  detail: (id: string) => [...approvalKeys.all, "detail", id] as const,
};

function useApprovalQueue(status: QueueStatus, q?: string) {
  return useQuery({
    queryKey: approvalKeys.queue(status, q),
    queryFn: () => apiClient.get<QueueResponse>("/approvals", { params: { status, q } }),
    placeholderData: keepPreviousData,
    refetchOnWindowFocus: false, // never reorder the list under the cursor
  });
}

/** Lightweight poll that only drives the S6 "New items available" banner. */
function usePendingCount() {
  return useQuery({
    queryKey: approvalKeys.pendingCount(),
    queryFn: () => apiClient.get<{ pending: number }>("/approvals/pending-count"),
    refetchInterval: 30_000,
  });
}

function useApprovalDetail(id: string | undefined) {
  return useQuery({
    queryKey: approvalKeys.detail(id ?? "none"),
    queryFn: () => apiClient.get<ExpenseReportDetail>(`/approvals/${id}`),
    enabled: Boolean(id),
  });
}

function useDecide() {
  return useMutation({
    mutationFn: (input: { id: string; decision: Decision; reason?: string }) =>
      apiClient.post<{ decisionId: string }>(`/approvals/${input.id}/decision`, input),
  });
}

function useUndoDecision() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (decisionId: string) => apiClient.delete<void>(`/decisions/${decisionId}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: approvalKeys.all }),
  });
}

/** Auto-advance preference: default on, persisted per user (browser storage as a convenience). */
function useAutoAdvance() {
  const [value, setValue] = React.useState(() => {
    try {
      return window.localStorage.getItem("approvals:auto-advance") !== "off";
    } catch {
      return true;
    }
  });
  const update = (next: boolean) => {
    setValue(next);
    try {
      window.localStorage.setItem("approvals:auto-advance", next ? "on" : "off");
    } catch {
      /* storage unavailable: keep in-memory value */
    }
  };
  return [value, update] as const;
}

/** Split layout applies at the catalog "desktop" breakpoint (>= 1024px). Render ONE layout so
 *  keyboard shortcuts and focus targets are registered once. Promote to src/hooks/ when shared. */
function useIsDesktop() {
  const query = "(min-width: 1024px)";
  return React.useSyncExternalStore(
    (onChange) => {
      const media = window.matchMedia(query);
      media.addEventListener("change", onChange);
      return () => media.removeEventListener("change", onChange);
    },
    () => window.matchMedia(query).matches,
    () => true,
  );
}

function isTypingTarget(target: EventTarget | null) {
  return target instanceof HTMLElement && Boolean(target.closest("input, textarea, select, [contenteditable=true]"));
}

// -----------------------------------------------------------------------------
// Detail pane with pinned decision bar
// -----------------------------------------------------------------------------

function ApprovalDetail({
  id,
  isPendingTab,
  onDecided,
  onBack,
}: {
  id: string;
  isPendingTab: boolean;
  onDecided: (report: ExpenseReportDetail, decision: Decision, decisionId: string) => void;
  onBack?: () => void;
}) {
  const detail = useApprovalDetail(id);
  const decide = useDecide();
  const [rejecting, setRejecting] = React.useState(false);
  const [reason, setReason] = React.useState("");
  const [reasonError, setReasonError] = React.useState<string>();
  const [decisionError, setDecisionError] = React.useState<string>();
  const reasonRef = React.useRef<HTMLTextAreaElement>(null);

  React.useEffect(() => {
    // Reset per item
    setRejecting(false);
    setReason("");
    setReasonError(undefined);
    setDecisionError(undefined);
  }, [id]);

  const report = detail.data;
  const canDecide = isPendingTab && report && !report.waitingOnRole;

  const submit = React.useCallback(
    (decision: Decision) => {
      if (!report) return;
      if (decision === "reject") {
        if (!rejecting) {
          setRejecting(true);
          requestAnimationFrame(() => reasonRef.current?.focus());
          return;
        }
        if (reason.trim().length < 5) {
          setReasonError("Enter a reason of at least 5 characters so the submitter knows what to fix.");
          reasonRef.current?.focus();
          return;
        }
      }
      setDecisionError(undefined);
      decide.mutate(
        { id: report.id, decision, reason: decision === "reject" ? reason.trim() : undefined },
        {
          onSuccess: ({ decisionId }) => onDecided(report, decision, decisionId),
          // S4: decision failure keeps the item, shows an inline alert, retry is manual
          onError: (error) => setDecisionError(error instanceof ApiError ? error.message : "The decision wasn't saved. Try again."),
        },
      );
    },
    [decide, onDecided, reason, rejecting, report],
  );

  // AC-QUEUE-3: single-key shortcuts (A approve, R reject) when focus is not in a text field
  React.useEffect(() => {
    if (!canDecide) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (isTypingTarget(event.target) || event.metaKey || event.ctrlKey || event.altKey) return;
      if (event.key === "a") {
        event.preventDefault();
        submit("approve");
      }
      if (event.key === "r") {
        event.preventDefault();
        submit("reject");
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [canDecide, submit]);

  if (detail.isPending) {
    return (
      <div aria-hidden="true" className="space-y-4 p-6">
        <Skeleton className="h-6 w-2/3" />
        <Skeleton className="h-4 w-1/3" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }

  if (detail.isError || !report) {
    return (
      <div className="p-6">
        <Alert variant="destructive" role="status">
          <CircleAlert aria-hidden="true" />
          <AlertTitle>Couldn't load this report</AlertTitle>
          <AlertDescription>
            <Button variant="outline" size="sm" className="mt-2" onClick={() => detail.refetch()}>
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  return (
    <section aria-labelledby="approval-heading" className="flex h-full min-h-0 flex-col">
      <header className="space-y-2 border-b border-border p-4 lg:px-6">
        {onBack ? (
          <Button variant="ghost" size="sm" className="-ml-2" onClick={onBack}>
            <ArrowLeft className="size-4" aria-hidden="true" />
            Back to queue
          </Button>
        ) : null}
        <h2 id="approval-heading" tabIndex={-1} className="text-lg font-semibold tracking-tight outline-none">
          {report.title}
        </h2>
        <p className="text-sm text-muted-foreground">
          {report.submitterName} · <span className="font-mono tabular-nums">{report.reference}</span> · {report.costCenter}
        </p>
        <p className="text-2xl font-semibold tabular-nums">{formatCurrency(report.total, report.currency)}</p>
      </header>

      <ScrollArea className="min-h-0 flex-1">
        <div className="space-y-4 p-4 lg:p-6">
          {report.policyWarnings.length > 0 ? (
            <Alert role="status" className="border-warning/50 bg-warning/10 [&>svg]:text-warning">
              <CircleAlert aria-hidden="true" />
              <AlertTitle>Policy warnings</AlertTitle>
              <AlertDescription>
                <ul className="list-disc pl-4">
                  {report.policyWarnings.map((warning) => (
                    <li key={warning}>{warning}</li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          ) : null}

          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead scope="col">Date</TableHead>
                <TableHead scope="col">Category</TableHead>
                <TableHead scope="col">Merchant</TableHead>
                <TableHead scope="col" className="text-right">
                  Amount
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {report.lines.map((line) => (
                <TableRow key={line.id} className="h-10">
                  <TableCell className="tabular-nums">{formatDate(line.date)}</TableCell>
                  <TableCell>{line.category}</TableCell>
                  <TableCell className="max-w-[200px] truncate">{line.merchant}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatCurrency(line.amount, report.currency)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </ScrollArea>

      {/* Decision bar pinned to the bottom of the detail pane (sticky on mobile) */}
      {report.waitingOnRole ? (
        <div className="border-t border-border bg-muted/30 p-4 text-sm text-muted-foreground">
          <Clock className="mr-1.5 inline size-4" aria-hidden="true" />
          Waiting on {report.waitingOnRole}. You can view this report but not decide it.
        </div>
      ) : canDecide ? (
        <div className="sticky bottom-0 space-y-3 border-t border-border bg-background p-4 lg:px-6">
          {decisionError ? (
            <Alert variant="destructive" role="alert">
              <CircleAlert aria-hidden="true" />
              <AlertDescription>{decisionError}</AlertDescription>
            </Alert>
          ) : null}
          {rejecting ? (
            <Field data-invalid={Boolean(reasonError)}>
              <FieldLabel htmlFor="reject-reason">Reason for rejection</FieldLabel>
              <Textarea
                ref={reasonRef}
                id="reject-reason"
                rows={2}
                value={reason}
                aria-invalid={Boolean(reasonError)}
                onChange={(event) => {
                  setReason(event.target.value);
                  if (reasonError && event.target.value.trim().length >= 5) setReasonError(undefined);
                }}
                onKeyDown={(event) => {
                  if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) submit("reject");
                  if (event.key === "Escape") setRejecting(false);
                }}
              />
              <FieldError>{reasonError}</FieldError>
            </Field>
          ) : null}
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="outline" onClick={() => submit("reject")} disabled={decide.isPending}>
                  {rejecting ? "Confirm rejection" : "Reject"}
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                Reject <Kbd>R</Kbd>
              </TooltipContent>
            </Tooltip>
            {/* Primary = most common decision */}
            <Tooltip>
              <TooltipTrigger asChild>
                <Button onClick={() => submit("approve")} disabled={decide.isPending} className="min-w-28">
                  {decide.isPending ? <Spinner /> : null}
                  Approve
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                Approve <Kbd>A</Kbd>
              </TooltipContent>
            </Tooltip>
          </div>
        </div>
      ) : null}
    </section>
  );
}

// -----------------------------------------------------------------------------
// Screen entry — tpl-inbox-queue
// -----------------------------------------------------------------------------

export function ApprovalQueueScreen() {
  const search = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const queryClient = useQueryClient();
  const queue = useApprovalQueue(search.status, search.q);
  const pendingCount = usePendingCount();
  const undo = useUndoDecision();
  const [autoAdvance, setAutoAdvance] = useAutoAdvance();
  const isDesktop = useIsDesktop();
  // Items decided in this session leave the pending list immediately, without reordering others.
  const [decidedIds, setDecidedIds] = React.useState<Set<string>>(new Set());

  const rows = (queue.data?.rows ?? []).filter((row) => !decidedIds.has(row.id));
  const setSearch = React.useCallback(
    (patch: Partial<ApprovalSearch>) => navigate({ search: (prev) => ({ ...prev, ...patch }), replace: true }),
    [navigate],
  );

  const onDecided = React.useCallback(
    (report: ExpenseReportDetail, decision: Decision, decisionId: string) => {
      const index = rows.findIndex((row) => row.id === report.id);
      const next = rows[index + 1] ?? rows[index - 1];
      setDecidedIds((prev) => new Set(prev).add(report.id));
      // AC-QUEUE-1: open the next item automatically (setting persisted, default on)
      setSearch({ id: autoAdvance ? next?.id : undefined });
      requestAnimationFrame(() => document.getElementById("approval-heading")?.focus());
      // S7: toast "{Decision} — {item}" with Undo for 5–8s
      toast(`${decision === "approve" ? "Approved" : "Rejected"} — ${report.title}`, {
        duration: 7000,
        action: {
          label: "Undo",
          onClick: () =>
            undo.mutate(decisionId, {
              onSuccess: () => {
                setDecidedIds((prev) => {
                  const copy = new Set(prev);
                  copy.delete(report.id);
                  return copy;
                });
                setSearch({ id: report.id });
              },
            }),
        },
      });
    },
    [autoAdvance, rows, setSearch, undo],
  );

  // j/k navigation across the queue (AC-QUEUE-3)
  React.useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (isTypingTarget(event.target) || event.metaKey || event.ctrlKey || event.altKey) return;
      if (event.key !== "j" && event.key !== "k") return;
      const index = rows.findIndex((row) => row.id === search.id);
      const target = rows[event.key === "j" ? Math.min(index + 1, rows.length - 1) : Math.max(index - 1, 0)];
      if (target) {
        event.preventDefault();
        setSearch({ id: target.id });
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [rows, search.id, setSearch]);

  const hasNewItems =
    search.status === "pending" && queue.data && pendingCount.data && pendingCount.data.pending > queue.data.counts.pending - decidedIds.size;
  const isPendingTab = search.status === "pending";
  const showDetailOnly = Boolean(search.id);

  const list = (
    <div className="flex h-full min-h-0 flex-col">
      <p className="border-b border-border px-3 py-2 text-xs text-muted-foreground">Sorted by oldest first</p>
      <ScrollArea className="min-h-0 flex-1">
        {queue.isPending ? (
          <div aria-hidden="true" className="space-y-1 p-2">
            {Array.from({ length: 8 }).map((_, index) => (
              <Skeleton key={index} className="h-14 w-full" />
            ))}
          </div>
        ) : queue.isError ? (
          <div className="p-3">
            <Alert variant="destructive" role="status">
              <CircleAlert aria-hidden="true" />
              <AlertTitle>Couldn't load the queue</AlertTitle>
              <AlertDescription>
                <Button variant="outline" size="sm" className="mt-2" onClick={() => queue.refetch()}>
                  Retry
                </Button>
              </AlertDescription>
            </Alert>
          </div>
        ) : rows.length === 0 && search.q ? (
          // S3: no-results for filters — distinct from inbox zero (AC-QUEUE-2)
          <Empty className="py-10">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <SearchX aria-hidden="true" />
              </EmptyMedia>
              <EmptyTitle>No results for '{search.q}'</EmptyTitle>
            </EmptyHeader>
            <EmptyContent>
              <Button variant="outline" size="sm" onClick={() => setSearch({ q: undefined })}>
                Clear filters
              </Button>
            </EmptyContent>
          </Empty>
        ) : rows.length === 0 ? (
          // S2: inbox zero — positive tone, last processed time, link to processed items, NO create action
          <Empty className="py-10">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <PartyPopper aria-hidden="true" />
              </EmptyMedia>
              <EmptyTitle>{isPendingTab ? "You're all caught up" : "Nothing here yet"}</EmptyTitle>
              {isPendingTab && queue.data?.lastProcessedAt ? (
                <EmptyDescription>
                  Last report processed <time dateTime={queue.data.lastProcessedAt}>{formatRelativeTime(queue.data.lastProcessedAt)}</time>.
                </EmptyDescription>
              ) : null}
            </EmptyHeader>
            {isPendingTab ? (
              <EmptyContent>
                <Button variant="link" asChild>
                  <Link to="." search={{ status: "approved" }}>
                    View approved reports
                  </Link>
                </Button>
              </EmptyContent>
            ) : null}
          </Empty>
        ) : (
          <ul role="listbox" aria-label={`${search.status} expense reports`} className="divide-y divide-border">
            {rows.map((row) => {
              const isSelected = row.id === search.id;
              return (
                <li
                  key={row.id}
                  role="option"
                  aria-selected={isSelected}
                  tabIndex={isSelected ? 0 : -1}
                  onClick={() => setSearch({ id: row.id })}
                  onKeyDown={(event) => event.key === "Enter" && setSearch({ id: row.id })}
                  className={cn("flex cursor-pointer gap-3 p-3 hover:bg-muted/50", isSelected && "bg-accent hover:bg-accent")}
                >
                  <Avatar className="size-8">
                    <AvatarFallback className="text-xs">{row.submitterName.slice(0, 2).toUpperCase()}</AvatarFallback>
                  </Avatar>
                  <div className="min-w-0 flex-1 space-y-1">
                    <p className="truncate text-sm font-medium">{row.title}</p>
                    <p className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
                      <span className="truncate">{row.submitterName}</span>
                      <span className="shrink-0 tabular-nums">{formatCurrency(row.total, row.currency)}</span>
                    </p>
                    {row.waitingOnRole ? (
                      <Badge variant="outline" className="text-muted-foreground">
                        Waiting on {row.waitingOnRole}
                      </Badge>
                    ) : (
                      <time dateTime={row.submittedAt} title={formatDateTime(row.submittedAt)} className="text-xs tabular-nums text-muted-foreground">
                        Submitted {formatRelativeTime(row.submittedAt)}
                      </time>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </ScrollArea>
    </div>
  );

  const detail = search.id ? (
    <ApprovalDetail id={search.id} isPendingTab={isPendingTab} onDecided={onDecided} />
  ) : (
    <Empty className="h-full">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <CircleCheck aria-hidden="true" />
        </EmptyMedia>
        <EmptyTitle>Select a report to review</EmptyTitle>
        <EmptyDescription>
          Move with <Kbd>J</Kbd> <Kbd>K</Kbd>, approve with <Kbd>A</Kbd>, reject with <Kbd>R</Kbd>.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  );

  return (
    <div className="flex h-[calc(100dvh-3.5rem)] flex-col gap-4 px-4 py-4 lg:px-8">
      {/* org-page-header (compact) with status tabs carrying counts */}
      <header className={cn("flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between", showDetailOnly && !isDesktop && "hidden")}>
        <div className="flex flex-wrap items-center gap-4">
          <h1 className="text-lg font-semibold tracking-tight">Approvals</h1>
          <Tabs value={search.status} onValueChange={(status) => setSearch({ status: status as QueueStatus, id: undefined })}>
            <TabsList>
              {(["pending", "approved", "rejected"] as const).map((status) => (
                <TabsTrigger key={status} value={status} className="capitalize">
                  {status}
                  {queue.data && status === "pending" ? (
                    <Badge variant="secondary" className="ml-1.5 tabular-nums">
                      {Math.max(queue.data.counts.pending - decidedIds.size, 0)}
                    </Badge>
                  ) : null}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
        <label className="flex items-center gap-2 text-sm">
          <Switch checked={autoAdvance} onCheckedChange={setAutoAdvance} aria-describedby="auto-advance-hint" />
          Auto-advance
          <span id="auto-advance-hint" className="sr-only">
            Open the next report after each decision
          </span>
        </label>
      </header>

      {/* S6: new items wait behind a banner; never auto-inserted above the selection */}
      {hasNewItems ? (
        <Alert role="status">
          <Clock aria-hidden="true" />
          <AlertDescription className="flex flex-wrap items-center gap-2">
            New reports are waiting for review.
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setDecidedIds(new Set());
                queryClient.invalidateQueries({ queryKey: approvalKeys.queue(search.status, search.q) });
              }}
            >
              Show new items
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      {isDesktop ? (
        <ResizablePanelGroup direction="horizontal" className="min-h-0 flex-1 rounded-lg border border-border bg-card">
          <ResizablePanel defaultSize={32} minSize={24} maxSize={40}>
            {list}
          </ResizablePanel>
          <ResizableHandle withHandle />
          <ResizablePanel defaultSize={68} minSize={45}>
            {detail}
          </ResizablePanel>
        </ResizablePanelGroup>
      ) : (
        // Mobile/tablet: list route → item route with a sticky decision bar at the bottom.
        // The list stays mounted (hidden) so Back restores its scroll offset.
        <div className="flex min-h-0 flex-1 flex-col rounded-lg border border-border bg-card">
          <div className={cn("min-h-0 flex-1", showDetailOnly && "hidden")}>{list}</div>
          {search.id ? (
            <div className="min-h-0 flex-1">
              <ApprovalDetail id={search.id} isPendingTab={isPendingTab} onDecided={onDecided} onBack={() => setSearch({ id: undefined })} />
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}
