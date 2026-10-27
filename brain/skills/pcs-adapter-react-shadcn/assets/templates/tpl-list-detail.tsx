/**
 * Blueprint: `tpl-list-detail` — List-Detail Split / Master-Detail (pcs-saas 01 §4)
 *
 * Domain: Support tickets of a workspace (read-heavy, opened in sequence).
 *
 * Catalog nodes composed here:
 *   org-page-header (compact) · org-filter-bar (compact: search + 1 facet) · selectable list
 *   org-detail-panel (inline) · mol-avatar-name · mol-status-badge · mol-key-value-list
 *   org-activity-feed (compact, last 5) · org-empty-state (first-use, no-results, select placeholder)
 *   org-section-error · org-banner (S6) · org-feedback (S7) · mol-skeleton · mol-copyable-value
 *
 * Acceptance checks satisfied (search for the tag):
 *   AC-SPLIT-1 Selected item highlighted, aria-selected="true", and in the URL (?id=).
 *   AC-SPLIT-2 ArrowUp/ArrowDown and j/k move selection and update the detail without reload.
 *   AC-SPLIT-3 Mobile back returns to the list at the same scroll offset (list stays mounted).
 *
 * Split into a project:
 *   src/routes/_app/tickets/index.tsx                   → Route (validateSearch: ticketSearchSchema)
 *   src/features/support/tickets/hooks/use-tickets.ts   → ticketKeys, useTickets, useTicket, useUpdateTicketStatus
 *   src/features/support/tickets/components/ticket-list.tsx, ticket-detail.tsx, ticket-status-badge.tsx
 *   src/features/support/tickets/index.tsx              → TicketWorkbenchScreen
 *
 * Rule: never auto-select the first item — opening a ticket marks it read (side effect).
 */
import * as React from "react";
import { getRouteApi } from "@tanstack/react-router";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  Check,
  CircleAlert,
  Copy,
  Inbox,
  LockKeyhole,
  MoreHorizontal,
  MousePointerClick,
  Plus,
  Search,
  SearchX,
  TriangleAlert,
} from "lucide-react";
import { toast } from "sonner";
import { z } from "zod";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
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
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import { Kbd } from "@/components/ui/kbd";
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "@/components/ui/resizable";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { ApiError, apiClient } from "@/lib/api-client";
import { formatDateTime, formatRelativeTime } from "@/lib/format";
import { cn } from "@/lib/utils";

// -----------------------------------------------------------------------------
// Types and data contract
// -----------------------------------------------------------------------------

export type TicketStatus = "new" | "open" | "waiting" | "resolved";
export type TicketPriority = "low" | "normal" | "high" | "urgent";

export interface TicketSummary {
  id: string;
  subject: string;
  requesterName: string;
  requesterAvatarUrl?: string;
  status: TicketStatus;
  updatedAt: string;
  unread: boolean;
}

export interface TicketDetail extends TicketSummary {
  reference: string;
  priority: TicketPriority;
  assigneeName?: string;
  channel: "email" | "chat" | "phone";
  createdAt: string;
  body: string;
  activity: { id: string; actor: string; sentence: string; at: string }[];
  canEdit: boolean;
}

export interface TicketListResponse {
  rows: TicketSummary[];
  totalUnfiltered: number;
  degradedReason?: string; // S6: e.g. mailbox sync paused
}

// -----------------------------------------------------------------------------
// URL state — selected item lives in ?id= (AC-SPLIT-1)
// -----------------------------------------------------------------------------

const ticketStatuses = ["new", "open", "waiting", "resolved"] as const;

export const ticketSearchSchema = z.object({
  id: z.string().optional().catch(undefined),
  q: z.string().trim().optional().catch(undefined),
  status: z.enum(["all", ...ticketStatuses]).default("all").catch("all"),
});

export type TicketSearch = z.infer<typeof ticketSearchSchema>;

const routeApi = getRouteApi("/_app/tickets/");

// -----------------------------------------------------------------------------
// Query hooks — list and detail load and fail independently (S1/S4 per pane)
// -----------------------------------------------------------------------------

export const ticketKeys = {
  all: ["tickets"] as const,
  list: (params: Omit<TicketSearch, "id">) => [...ticketKeys.all, "list", params] as const,
  detail: (id: string) => [...ticketKeys.all, "detail", id] as const,
};

export function useTickets(search: TicketSearch) {
  const { id: _id, ...params } = search;
  return useQuery({
    queryKey: ticketKeys.list(params),
    queryFn: () => apiClient.get<TicketListResponse>("/tickets", { params }),
    placeholderData: keepPreviousData,
  });
}

export function useTicket(id: string | undefined) {
  return useQuery({
    queryKey: ticketKeys.detail(id ?? "none"),
    queryFn: () => apiClient.get<TicketDetail>(`/tickets/${id}`),
    enabled: Boolean(id),
    retry: (count, error) => !(error instanceof ApiError && [403, 404].includes(error.status)) && count < 2,
  });
}

export function useUpdateTicketStatus() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: TicketStatus }) =>
      apiClient.patch<TicketDetail>(`/tickets/${id}`, { status }),
    // S7: list item updates in place; the item does not move until the user navigates away.
    onSuccess: (updated) => {
      queryClient.setQueryData(ticketKeys.detail(updated.id), updated);
      queryClient.setQueriesData<TicketListResponse>({ queryKey: [...ticketKeys.all, "list"] }, (old) =>
        old ? { ...old, rows: old.rows.map((row) => (row.id === updated.id ? { ...row, status: updated.status } : row)) } : old,
      );
    },
  });
}

// -----------------------------------------------------------------------------
// mol-status-badge
// -----------------------------------------------------------------------------

const ticketStatusRole: Record<TicketStatus, { label: string; className: string }> = {
  new: { label: "New", className: "border-info/30 bg-info/10 text-info" },
  open: { label: "Open", className: "border-warning/30 bg-warning/10 text-warning" },
  waiting: { label: "Waiting", className: "text-muted-foreground" },
  resolved: { label: "Resolved", className: "border-success/30 bg-success/10 text-success" },
};

function TicketStatusBadge({ status }: { status: TicketStatus }) {
  const role = ticketStatusRole[status];
  return (
    <Badge variant="outline" className={cn("gap-1.5", role.className)}>
      <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
      {role.label}
    </Badge>
  );
}

function initials(name: string) {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");
}

// -----------------------------------------------------------------------------
// Left pane: compact filter bar + selectable list (listbox semantics)
// -----------------------------------------------------------------------------

function TicketList({
  search,
  query,
  onSelect,
  onSearchChange,
}: {
  search: TicketSearch;
  query: ReturnType<typeof useTickets>;
  onSelect: (id: string) => void;
  onSearchChange: (patch: Partial<TicketSearch>) => void;
}) {
  const rows = query.data?.rows ?? [];
  const listRef = React.useRef<HTMLUListElement>(null);

  // AC-SPLIT-2: Arrow keys and j/k move selection; Enter focuses the detail pane.
  const onKeyDown = (event: React.KeyboardEvent<HTMLUListElement>) => {
    const index = rows.findIndex((row) => row.id === search.id);
    const move = (delta: number) => {
      event.preventDefault();
      const next = rows[Math.min(Math.max(index + delta, 0), rows.length - 1)];
      if (next) {
        onSelect(next.id);
        listRef.current?.querySelector<HTMLElement>(`[data-id="${next.id}"]`)?.scrollIntoView({ block: "nearest" });
      }
    };
    if (event.key === "ArrowDown" || event.key === "j") move(index === -1 ? 0 : 1);
    if (event.key === "ArrowUp" || event.key === "k") move(index === -1 ? 0 : -1);
    if (event.key === "Enter" && search.id) {
      event.preventDefault();
      document.getElementById("ticket-detail-heading")?.focus();
    }
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* org-filter-bar (compact) */}
      <div role="search" className="flex items-center gap-2 border-b border-border p-3">
        <InputGroup className="h-8 flex-1">
          <InputGroupAddon>
            <Search className="size-4" aria-hidden="true" />
          </InputGroupAddon>
          <InputGroupInput
            type="search"
            aria-label="Search tickets"
            placeholder="Search tickets…"
            // Debounce 300ms in production (see useDebouncedValue in tpl-index-table.tsx).
            defaultValue={search.q ?? ""}
            onChange={(event) => {
              const value = event.target.value.trim();
              if (value.length !== 1) onSearchChange({ q: value || undefined });
            }}
          />
        </InputGroup>
        <Select value={search.status} onValueChange={(status) => onSearchChange({ status: status as TicketSearch["status"] })}>
          <SelectTrigger size="sm" className="w-[120px]" aria-label="Filter by status">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            {ticketStatuses.map((status) => (
              <SelectItem key={status} value={status}>
                {ticketStatusRole[status].label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <ScrollArea className="min-h-0 flex-1">
        {query.isPending ? (
          // S1: list skeleton independent from the detail skeleton
          <div aria-hidden="true" className="divide-y divide-border">
            {Array.from({ length: 10 }).map((_, index) => (
              <div key={index} className="flex gap-3 p-3">
                <Skeleton className="size-8 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-3/4" />
                  <Skeleton className="h-3 w-1/2" />
                </div>
              </div>
            ))}
            <span className="sr-only" role="status">
              Loading tickets
            </span>
          </div>
        ) : query.isError ? (
          // S4: list fails independently
          <div className="p-3">
            <Alert variant="destructive" role="status">
              <CircleAlert aria-hidden="true" />
              <AlertTitle>Couldn't load tickets</AlertTitle>
              <AlertDescription>
                <Button variant="outline" size="sm" className="mt-2" onClick={() => query.refetch()}>
                  Retry
                </Button>
              </AlertDescription>
            </Alert>
          </div>
        ) : rows.length === 0 ? (
          // S3: list no-results (detail pane keeps its placeholder)
          <Empty className="py-12">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <SearchX aria-hidden="true" />
              </EmptyMedia>
              <EmptyTitle>{search.q ? `No results for '${search.q}'` : "No tickets match your filters"}</EmptyTitle>
            </EmptyHeader>
            <EmptyContent>
              <Button variant="outline" size="sm" onClick={() => onSearchChange({ q: undefined, status: "all" })}>
                Clear filters
              </Button>
            </EmptyContent>
          </Empty>
        ) : (
          <ul
            ref={listRef}
            role="listbox"
            aria-label="Tickets"
            tabIndex={0}
            aria-activedescendant={search.id ? `ticket-option-${search.id}` : undefined}
            onKeyDown={onKeyDown}
            className="divide-y divide-border outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            {rows.map((ticket) => {
              const isSelected = ticket.id === search.id;
              return (
                <li
                  key={ticket.id}
                  id={`ticket-option-${ticket.id}`}
                  data-id={ticket.id}
                  role="option"
                  aria-selected={isSelected} // AC-SPLIT-1
                  onClick={() => onSelect(ticket.id)}
                  className={cn(
                    "flex cursor-pointer gap-3 p-3 transition-colors hover:bg-muted/50",
                    isSelected && "bg-accent text-accent-foreground hover:bg-accent",
                  )}
                >
                  {/* mol-avatar-name: 32px in lists, alt="" because the name is adjacent */}
                  <Avatar className="size-8">
                    <AvatarImage src={ticket.requesterAvatarUrl} alt="" />
                    <AvatarFallback className="text-xs">{initials(ticket.requesterName)}</AvatarFallback>
                  </Avatar>
                  <div className="min-w-0 flex-1 space-y-1">
                    <div className="flex items-center gap-2">
                      {ticket.unread ? (
                        <span className="size-2 shrink-0 rounded-full bg-primary">
                          <span className="sr-only">Unread</span>
                        </span>
                      ) : null}
                      <p className={cn("truncate text-sm", ticket.unread && "font-medium")} title={ticket.subject}>
                        {ticket.subject}
                      </p>
                    </div>
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate text-xs text-muted-foreground">{ticket.requesterName}</span>
                      <time dateTime={ticket.updatedAt} title={formatDateTime(ticket.updatedAt)} className="shrink-0 text-xs tabular-nums text-muted-foreground">
                        {formatRelativeTime(ticket.updatedAt)}
                      </time>
                    </div>
                    <TicketStatusBadge status={ticket.status} />
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </ScrollArea>
    </div>
  );
}

// -----------------------------------------------------------------------------
// Right pane: org-detail-panel (inline variant)
// -----------------------------------------------------------------------------

function CopyableValue({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = React.useState(false);
  return (
    <span className="inline-flex items-center gap-1">
      <code className="font-mono text-sm tabular-nums">{value}</code>
      <Button
        variant="ghost"
        size="icon"
        className="size-7"
        aria-label={`Copy ${label}`}
        onClick={async () => {
          await navigator.clipboard.writeText(value);
          setCopied(true);
          window.setTimeout(() => setCopied(false), 2000);
        }}
      >
        {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      </Button>
      <span role="status" className="sr-only">
        {copied ? "Copied" : ""}
      </span>
    </span>
  );
}

function TicketDetailPane({ id, onBack }: { id: string | undefined; onBack?: () => void }) {
  const ticket = useTicket(id);
  const updateStatus = useUpdateTicketStatus();

  if (!id) {
    // Nothing selected → placeholder with keyboard hint
    return (
      <Empty className="h-full">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <MousePointerClick aria-hidden="true" />
          </EmptyMedia>
          <EmptyTitle>Select a ticket</EmptyTitle>
          <EmptyDescription>
            Use <Kbd>J</Kbd> and <Kbd>K</Kbd> or the arrow keys to move through the list.
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    );
  }

  if (ticket.isPending) {
    return (
      <div aria-hidden="true" className="space-y-6 p-6">
        <div className="space-y-2">
          <Skeleton className="h-6 w-2/3" />
          <Skeleton className="h-5 w-20 rounded-full" />
        </div>
        <Skeleton className="h-24 w-full" />
        <div className="grid grid-cols-[35%_1fr] gap-3">
          {Array.from({ length: 8 }).map((_, index) => (
            <Skeleton key={index} className="h-4 w-full" />
          ))}
        </div>
      </div>
    );
  }

  if (ticket.error instanceof ApiError && ticket.error.status === 403) {
    // S5: detail pane no-access for a restricted record
    return (
      <Empty className="h-full">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <LockKeyhole aria-hidden="true" />
          </EmptyMedia>
          <EmptyTitle>You don't have access to this ticket</EmptyTitle>
          <EmptyDescription>It belongs to a restricted queue. A support lead can grant access.</EmptyDescription>
        </EmptyHeader>
      </Empty>
    );
  }

  if (ticket.isError) {
    return (
      <div className="p-6">
        <Alert variant="destructive" role="status">
          <CircleAlert aria-hidden="true" />
          <AlertTitle>
            {ticket.error instanceof ApiError && ticket.error.status === 404 ? "This ticket doesn't exist or was deleted" : "Couldn't load this ticket"}
          </AlertTitle>
          <AlertDescription>
            <Button variant="outline" size="sm" className="mt-2" onClick={() => ticket.refetch()}>
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  const data = ticket.data;

  return (
    <section aria-labelledby="ticket-detail-heading" className="flex h-full min-h-0 flex-col">
      {/* Panel header: identity, status, 1 primary + overflow (mol-action-menu panel budget 1+1) */}
      <header className="flex flex-col gap-3 border-b border-border p-4 lg:px-6">
        {onBack ? (
          <Button variant="ghost" size="sm" className="-ml-2 w-fit" onClick={onBack}>
            <ArrowLeft className="size-4" aria-hidden="true" />
            Back to tickets
          </Button>
        ) : null}
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 space-y-1">
            <h2 id="ticket-detail-heading" tabIndex={-1} className="text-lg font-semibold tracking-tight outline-none">
              {data.subject}
            </h2>
            <div className="flex flex-wrap items-center gap-2">
              <TicketStatusBadge status={data.status} />
              <CopyableValue label="ticket reference" value={data.reference} />
            </div>
          </div>
          {data.canEdit ? (
            <div className="flex shrink-0 items-center gap-2">
              <Button
                disabled={updateStatus.isPending || data.status === "resolved"}
                onClick={() =>
                  updateStatus.mutate(
                    { id: data.id, status: "resolved" },
                    // S7: inline change is the confirmation (badge updates in place); no toast needed.
                    { onError: () => toast.error("Couldn't resolve ticket", { action: { label: "Retry", onClick: () => updateStatus.mutate({ id: data.id, status: "resolved" }) } }) },
                  )
                }
              >
                {updateStatus.isPending ? <Spinner /> : null}
                {updateStatus.isPending ? "Resolving…" : "Resolve ticket"}
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" size="icon" aria-label="More actions">
                    <MoreHorizontal className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onSelect={() => updateStatus.mutate({ id: data.id, status: "waiting" })}>
                    Mark as waiting
                  </DropdownMenuItem>
                  <DropdownMenuItem>Assign to…</DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          ) : null}
        </div>
      </header>

      <ScrollArea className="min-h-0 flex-1">
        <div className="space-y-6 p-4 lg:p-6 @container">
          <p className="max-w-prose whitespace-pre-line text-sm leading-relaxed">{data.body}</p>

          <Separator />

          {/* mol-key-value-list: two columns when the panel is >= 480px wide (container query) */}
          <section aria-labelledby="ticket-properties">
            <h3 id="ticket-properties" className="mb-3 text-sm font-semibold">
              Properties
            </h3>
            <dl className="grid grid-cols-1 gap-x-4 gap-y-3 @min-[480px]:grid-cols-[minmax(0,35%)_1fr]">
              <dt className="text-xs text-muted-foreground">Requester</dt>
              <dd className="text-sm">{data.requesterName}</dd>
              <dt className="text-xs text-muted-foreground">Priority</dt>
              <dd className="text-sm capitalize">{data.priority}</dd>
              <dt className="text-xs text-muted-foreground">Assignee</dt>
              <dd className="text-sm">{data.assigneeName ?? <span aria-label="Not set">—</span>}</dd>
              <dt className="text-xs text-muted-foreground">Channel</dt>
              <dd className="text-sm capitalize">{data.channel}</dd>
              <dt className="text-xs text-muted-foreground">Created</dt>
              <dd className="text-sm tabular-nums">
                <time dateTime={data.createdAt}>{formatDateTime(data.createdAt)}</time>
              </dd>
            </dl>
          </section>

          {/* org-activity-feed (compact): last 5 + View all */}
          <section aria-labelledby="ticket-activity">
            <h3 id="ticket-activity" className="mb-3 text-sm font-semibold">
              Activity
            </h3>
            {data.activity.length === 0 ? (
              <p className="text-sm text-muted-foreground">No activity yet.</p>
            ) : (
              <ol className="space-y-3">
                {data.activity.slice(0, 5).map((entry) => (
                  <li key={entry.id} className="flex items-baseline justify-between gap-3 text-sm">
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
          </section>
        </div>
      </ScrollArea>
    </section>
  );
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

// -----------------------------------------------------------------------------
// Screen entry — tpl-list-detail
// -----------------------------------------------------------------------------

export function TicketWorkbenchScreen() {
  const search = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const isDesktop = useIsDesktop(); // below 1024px the split becomes two "routes"
  const tickets = useTickets(search);

  const select = React.useCallback(
    (id: string | undefined) => navigate({ search: (prev) => ({ ...prev, id }), replace: isDesktop }),
    [isDesktop, navigate],
  );
  const setFilters = React.useCallback(
    (patch: Partial<TicketSearch>) => navigate({ search: (prev) => ({ ...prev, ...patch }), replace: true }),
    [navigate],
  );

  const isZeroData =
    !tickets.isPending && !tickets.isError && tickets.data?.totalUnfiltered === 0 && !search.q && search.status === "all";
  const showDetailOnly = !isDesktop && Boolean(search.id);

  return (
    <div className="flex h-[calc(100dvh-3.5rem)] flex-col gap-4 px-4 py-4 lg:px-8">
      {/* org-page-header (compact): title + primary only */}
      <header className={cn("flex items-center justify-between gap-4", showDetailOnly && "hidden")}>
        <h1 className="text-lg font-semibold tracking-tight">Tickets</h1>
        <Button size="sm">
          <Plus className="size-4" aria-hidden="true" />
          Create ticket
        </Button>
      </header>

      {tickets.data?.degradedReason ? (
        <Alert role="status" className="border-warning/50 bg-warning/10 [&>svg]:text-warning">
          <TriangleAlert aria-hidden="true" />
          <AlertTitle>New emails aren't arriving</AlertTitle>
          <AlertDescription>{tickets.data.degradedReason} Existing tickets can still be answered.</AlertDescription>
        </Alert>
      ) : null}

      {isZeroData ? (
        // S2: whole split replaced by the first-use empty state
        <Empty className="flex-1 rounded-lg border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Inbox aria-hidden="true" />
            </EmptyMedia>
            <EmptyTitle>No tickets yet</EmptyTitle>
            <EmptyDescription>Tickets collect customer requests from email and chat in one place.</EmptyDescription>
          </EmptyHeader>
          <EmptyContent className="flex-row justify-center gap-2">
            <Button>Create ticket</Button>
            <Button variant="outline">Connect a mailbox</Button>
          </EmptyContent>
        </Empty>
      ) : (
        isDesktop ? (
          /* Desktop (>= 1024px): resizable split; list 320–400px, detail >= 480px */
          <ResizablePanelGroup direction="horizontal" className="min-h-0 flex-1 rounded-lg border border-border bg-card">
            <ResizablePanel defaultSize={30} minSize={24} maxSize={40}>
              <TicketList search={search} query={tickets} onSelect={select} onSearchChange={setFilters} />
            </ResizablePanel>
            <ResizableHandle withHandle />
            <ResizablePanel defaultSize={70} minSize={45}>
              <TicketDetailPane id={search.id} />
            </ResizablePanel>
          </ResizablePanelGroup>
        ) : (
          /* Mobile/tablet: list stays mounted and hidden while the detail shows, so Back keeps scroll (AC-SPLIT-3) */
          <div className="flex min-h-0 flex-1 flex-col rounded-lg border border-border bg-card">
            <div className={cn("min-h-0 flex-1", showDetailOnly && "hidden")}>
              <TicketList search={search} query={tickets} onSelect={select} onSearchChange={setFilters} />
            </div>
            {showDetailOnly ? (
              <div className="min-h-0 flex-1">
                <TicketDetailPane id={search.id} onBack={() => select(undefined)} />
              </div>
            ) : null}
          </div>
        )
      )}
    </div>
  );
}
