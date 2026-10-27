# 02 — SaaS Organisms (`org-*`)

An organism is a self-contained region built from molecules (`mol-*`, see `03-molecules-and-patterns.md`) with its own behaviour and states. Templates in `01-page-templates.md` are assembled only from these organisms or from project organisms that declare which `org-*` ID they implement (`06-ui-inventory.md`). State codes `S1`–`S7` are defined in `01-page-templates.md` §0.1.

Each entry lists: **Purpose**, **Anatomy**, **Behaviour**, **States**, **Accessibility**, **Anti-patterns**.

---

## Navigation and Frame

### `org-app-sidebar` — Application Sidebar Navigation

- **Purpose**: Level-1 navigation between product areas.
- **Anatomy**: Workspace identity at top (or in `org-top-bar`, never both); grouped nav items (icon + label, optional count badge); group headings (max 4 groups); footer slot (help, settings, collapse toggle).
- **Behaviour**: Max 7 items per group, max 12 items total; items ordered by frequency, "Settings" last; active item matches the current route prefix; collapse to icon rail shows labels as tooltips; collapse state persisted per user.
- **States**: Count badges load lazily and are omitted on failure (never show "0" on error). Items the user cannot access are hidden.
- **Accessibility**: `nav` landmark with label "Main"; active item `aria-current="page"`; collapse button has `aria-expanded`; full keyboard traversal with visible focus.
- **Anti-patterns**: Nested expandable trees deeper than 1 level; icons without labels in expanded mode; badges used for marketing ("New!") on more than 1 item.

### `org-top-bar` — Top Bar

- **Purpose**: Global context and tools available on every page.
- **Anatomy** (left → right): mobile menu button (mobile only); workspace switcher (`mol-avatar-name` + chevron, opens a list with search when `> 5` workspaces, "Create workspace" at bottom); global search trigger (input-looking button showing the shortcut hint, opens `org-command-palette`); spacer; help; notifications bell with unread dot/count (opens `org-notification-center`); account menu (avatar → profile, preferences, theme, sign out).
- **Behaviour**: Height fixed `48–64px`; sticky; switching workspace navigates to the same section in the new workspace or its dashboard if the section does not exist.
- **States**: Unread count failure hides the badge. Workspace list failure shows inline error with retry inside the menu.
- **Accessibility**: `header` landmark; every icon button has an accessible name; menus use menu button semantics and close on `Escape`, returning focus to the trigger.
- **Anti-patterns**: Page-specific actions in the top bar; more than 5 icon buttons; sign out as a top-level button.

### `org-page-header` — Page Header

- **Purpose**: Identify the page and host its primary and secondary actions.
- **Anatomy**: optional `mol-breadcrumbs` (only at depth `>= 2`); title (level-1 type); optional count or `mol-status-badge`; 1-line description (optional, max 120 characters); actions group right-aligned: max 1 primary + 2 secondary visible + `mol-action-menu` overflow.
- **Variants**: `default` (index, settings), `record` (identity + status + 2–4 inline key facts), `compact` (list-detail, inbox: title + primary only).
- **Behaviour**: Title equals the nav label for index pages and the object name for record pages; document title mirrors it ("{Title} · {Product}"). Header is not sticky by default; becomes sticky only on `tpl-record-detail` with long tabs (condensed to identity + primary).
- **States**: Record title skeleton while loading; actions disabled until data loaded.
- **Accessibility**: Title is the single `h1` of the page.
- **Anti-patterns**: Two primary buttons; description repeating the title; actions placed below the title on desktop.

### `org-tabs` — Page Tabs (Level-2 Navigation)

- **Purpose**: Switch between views of the same object or collection.
- **Anatomy**: Tab list (label + optional count), max 6 tabs; content panel.
- **Behaviour**: Active tab in URL (`?tab=`); counts load independently; tab content lazy-loads with its own `S1`/`S4`; overflow scrolls horizontally with fade edges and stays keyboard reachable.
- **Accessibility**: `tablist`/`tab`/`tabpanel` roles; arrow keys move between tabs; `aria-selected` on the active tab.
- **Anti-patterns**: Tabs nested inside tabs; tabs used as a wizard; tabs for fewer than 2 views.

### `org-banner` — Page or Global Banner

- **Purpose**: Persistent, non-blocking notice about a condition affecting the page or the account.
- **Anatomy**: Icon by severity + message (1 sentence) + optional action link + dismiss (only for `info`).
- **Variants**: `info`, `warning` (degraded, trial ending), `danger` (payment failed, account suspended).
- **Behaviour**: Max 1 global banner (shell) + 1 page banner; `warning` and `danger` are not dismissible while the condition exists.
- **Accessibility**: `role="status"` for info/warning, `role="alert"` for danger only when it appears after load.
- **Anti-patterns**: Banners for marketing; stacking 3+ banners; banners for field validation.

### `org-command-palette` — Command Palette

- **Purpose**: Keyboard-first navigation, search, and action execution.
- **Anatomy**: Search input; grouped results ("Recent", "Pages", "{Object types}", "Actions"); each item: icon, label, optional shortcut hint; footer with key hints.
- **Behaviour**: Opens with `Ctrl/Cmd+K` and the top-bar search trigger; empty query shows recent items (max 5) and suggested actions; debounce `150–250ms` on remote search; max 5 items per group with "See all results" → `tpl-search-results`; `Enter` executes; actions requiring input open their container.
- **States**: Loading indicator inside the input; no results: "No results for '{query}'"; remote failure shows local results plus "Search unavailable".
- **Accessibility**: Dialog with combobox + listbox semantics; `aria-activedescendant` tracks the highlighted item; `Escape` closes and restores focus.
- **Anti-patterns**: Destructive actions executable without confirmation; palette as the only path to a feature.

### `org-notification-center` — Notification Center

- **Purpose**: Collect asynchronous events addressed to the user (mentions, assignments, completed jobs, approvals).
- **Anatomy**: Popover or sheet; header with "Mark all as read" and settings link; tabs "Unread" / "All" (optional); items: actor avatar, sentence ("{Actor} {verb} {object}"), relative time, unread dot, deep link; "View all" footer when paginated.
- **Behaviour**: Opening does not mark all as read; clicking an item marks it read and navigates; items grouped "Today", "Earlier"; max 50 loaded, then "Load more".
- **States**: `S1` 5 skeleton items; `S2` "You're all caught up"; `S4` inline error with retry.
- **Accessibility**: Bell button name includes unread count ("Notifications, 3 unread"); new notifications announced politely, never with `alert`.
- **Anti-patterns**: Toasts for every notification; notifications without a link to the object.

---

## Collections

### `org-filter-bar` — Filter Bar

- **Purpose**: Narrow a collection by text and facets with full visibility of what is applied.
- **Anatomy** (left → right): `mol-search-input` (width `240–320px`); up to 3 visible `mol-facet-filter` buttons for the most used attributes; "More filters" (opens a sheet/popover with remaining filters) when more exist; spacer; sort select (if not sortable by column headers); view options (density, columns). Second line, only when filters are active: `mol-filter-chip` list + "Clear filters".
- **Behaviour**: Every filter change resets pagination to page 1; every value serializes into the URL; facets show option counts when available; multi-select facets show "{Attribute}: {value}" or "{Attribute}: {n} selected"; date facets offer presets (Today, Last 7 days, Last 30 days, Custom).
- **States**: Facet options loading → spinner inside the popover; failure → "Couldn't load options" + retry inside the popover, other filters stay usable.
- **Accessibility**: Search input has a visible or accessible label; facet buttons expose `aria-expanded`; chips have a remove button labelled "Remove filter {attribute}: {value}"; results count change announced via a polite live region ("24 results").
- **Anti-patterns**: Filter sidebar plus filter bar plus column-header filters on one table; applying filters only after an "Apply" button for single facets; hiding active filters inside a closed popover.

### `org-data-table` — Data Table

- **Purpose**: Scan and compare records across attributes and act on one or many.
- **Anatomy**: Optional selection column (checkbox); identity column (Tier 1 attribute, link to record, `font-weight 500`); status column (`mol-status-badge`); 2–4 attribute columns; actions column (`mol-action-menu`, right-most, header label visually hidden "Actions"); sticky header; footer with `mol-pagination`.
- **Column rules**:
  1. Default visible columns `5–7`; additional columns available via column visibility menu.
  2. Text left-aligned; numbers, currency, durations right-aligned with tabular figures; badges and icons left-aligned (center only for fixed-width icon-only columns).
  3. Column header labels in sentence case, `<= 3` words, with unit in header when uniform ("Amount (EUR)").
  4. Minimum column width `80px`; identity column `>= 200px`; long text truncates with tooltip (`mol-truncation`).
  5. Empty cell renders an em dash "—" with accessible text "Not set", never blank.
- **Sorting**: Sortable columns show a sort indicator on hover and the active direction always; one active sort by default; default sort stated (most recent first for time-based objects, alphabetical for named catalogs); sort in URL.
- **Pagination vs infinite**: Paginate (page size 25 default; options 10/25/50/100) when users return to positions, export, or bulk-select; "Load more" for append-only feeds; infinite scroll only for logs and feeds, never for tables with bulk selection.
- **Row selection**: Header checkbox selects the current page; after selecting a full page show "Select all {total} {objects}"; selection survives pagination only when stated in the bulk bar; selection clears after a bulk action completes.
- **Row actions**: Max 1 visible inline action (most frequent, non-destructive); everything else in the row `mol-action-menu`; destructive item last, separated, danger-colored text; row click opens the record (the whole row is the target, inner controls stop propagation).
- **Density**: `compact` default for daily tools, toggle persisted; row height per density code.
- **States**: `S1` 8–10 skeleton rows with real headers; refetch keeps rows with a top progress line. `S2`/`S3` per template. `S4` error in body, headers stay. `S6` stale indicator in footer. `S7` updated row flashes a highlight `<= 2s`.
- **Accessibility**: Native table semantics (`table`, `th` with `scope`); sortable headers are buttons with `aria-sort`; selection checkboxes labelled "Select {identity}"; row action menu labelled "Actions for {identity}"; keyboard focus visible on rows and controls.
- **Anti-patterns**: Horizontal scroll at desktop width with default columns; action icons repeated in every row (> 1); zebra stripes plus borders plus hover color; numeric columns left-aligned.

### `org-bulk-action-bar` — Bulk Action Bar

- **Purpose**: Apply one action to many selected records.
- **Anatomy**: Selected count ("12 selected"), "Select all {total}" link when applicable, max 3 visible actions + overflow, "Clear selection".
- **Behaviour**: Appears only with `>= 1` selected, replacing the filter bar line or floating at the bottom center; destructive bulk actions use `org-destructive-confirm` with count; progress for long operations with partial-failure report.
- **Accessibility**: Appearance announced politely ("12 selected, bulk actions available"); reachable by `Tab` immediately after the table header.
- **Anti-patterns**: Bulk actions always visible but disabled; bulk delete without count in the confirmation.

### `org-card-grid` — Card Grid

- **Purpose**: Present visually recognized records for browsing and picking.
- **Anatomy**: Card = optional media/icon (fixed aspect ratio, `16:9` or `1:1`), title (max 2 lines), 1–2 metadata lines, optional `mol-status-badge`, footer with `mol-avatar-name` or timestamp, `mol-action-menu` top-right.
- **Behaviour**: Whole card is the link to the record; hover raises border contrast, never scales; responsive columns per `tpl-index-cards`.
- **States**: Skeleton cards with identical geometry; missing media → neutral placeholder with object-type icon.
- **Accessibility**: Card exposes one link with the title as its name; action menu separately focusable.
- **Anti-patterns**: Multiple links per card; uneven card heights; information only in images.

### `org-detail-panel` — Detail Panel / Inspector

- **Purpose**: Show and edit one record next to the collection it came from.
- **Variants**: `sheet` (overlay from the right, width `480–640px`) and `inline` (right pane of `tpl-list-detail`).
- **Anatomy**: Header (identity, status badge, close, "Open full page" when a `tpl-record-detail` exists, prev/next arrows); action row (primary + overflow); body: `mol-key-value-list` grouped in sections; optional compact `org-activity-feed` (last 5 + "View all").
- **Behaviour**: Opening sets `?inspect={id}` in the URL; background list keeps scroll and filters; prev/next follow the list order; editing inline per field (click to edit, `Enter` save, `Escape` cancel) or switches the body to a form with sticky footer.
- **States**: Header skeleton + body skeleton; not found → panel-level message with "Close"; field save failure reverts and shows inline error.
- **Accessibility**: Sheet variant is a modal dialog with focus trap and labelled by the identity; `Escape` closes (dirty guard applies).
- **Anti-patterns**: Full data tables inside the panel; panel over panel; panel width `< 400px`.

---

## Forms and Flows

### `org-sectioned-form` — Sectioned Form

- **Purpose**: Collect or edit structured data with low cognitive load.
- **Anatomy**: Sections, each with heading, optional 1-line description, and 3–7 `mol-form-field` items; optional error summary at top; actions per owning template (dialog footer, sticky footer, or card footer).
- **Rules**:
  1. Single column; two fields share a row only when tightly coupled and short (first/last name, city/postal code, start/end date).
  2. Fields ordered by the user's mental sequence, required fields before optional within a section.
  3. More than 7 fields in a group → split into sections; more than 4 sections on a page → add section index; more than 30 fields → reconsider scope or use `tpl-wizard`.
  4. Defaults prefilled where the system can infer them; optional fields labelled "(optional)" when they are the minority, required marked when they are the minority — never both markers.
  5. Validation timing per `03-molecules-and-patterns.md` §2.
- **Read mode**: Same section structure rendered as `mol-key-value-list`, so view and edit layouts match.
- **States**: Section skeletons; submit pending (fields read-only, primary spinner); error summary.
- **Accessibility**: Sections use `fieldset` + `legend` or headings; error summary `role="alert"` with links to fields; focus moves to the first invalid field or the summary on submit.
- **Anti-patterns**: Placeholder-only labels; multi-column grids of unrelated fields; disabled submit with no explanation.

### `org-wizard-stepper` — Wizard Stepper

- **Purpose**: Show position and progress in a multi-step process.
- **Anatomy**: Ordered steps (number, name, state: completed/current/upcoming/error); compact variant "Step {n} of {total} — {name}".
- **Behaviour**: Completed steps are clickable to go back; upcoming steps are not clickable; a step with errors shows an error state; max 5 steps.
- **Accessibility**: Ordered list; current step `aria-current="step"`; state conveyed in text, not only by icon color.
- **Anti-patterns**: Steps named "Step 1", "Step 2" without names; jumping forward over unvalidated steps.

### `org-auth-form` — Authentication Form

- **Purpose**: Credential entry for `tpl-auth` pages.
- **Anatomy**: Optional identity-provider buttons (full width, max 3, provider name in label), divider "or", fields (email, password with show/hide), inline error area above fields, primary full-width submit, secondary link row.
- **Rules**: Submit label matches the page verb ("Sign in", "Create account", "Send reset link", "Verify"); one-time-code inputs accept paste of the full code and auto-submit on completion.
- **Accessibility**: Proper `autocomplete` tokens; password toggle labelled "Show password"/"Hide password" with `aria-pressed`.

### `org-file-dropzone` — File Dropzone

- **Purpose**: Select files for upload or import.
- **Anatomy**: Dashed-border area with icon, "Drag a file here or browse", accepted formats and max size line, template download link (import); selected file row with name, size, remove, progress.
- **Behaviour**: Validate type and size on selection, before upload; drag-over state raises border contrast; multiple files listed with per-file progress and errors.
- **Accessibility**: Keyboard-operable "Browse" button; drag-and-drop is never the only path; errors announced.

### `org-destructive-confirm` — Destructive Action Confirmation

- **Purpose**: Prevent irreversible mistakes (Tier C of `pcs-core` friction matrix).
- **Anatomy**: Modal dialog: title "{Verb} {object name}?" ("Delete project 'Atlas'?"); body: concrete consequence (what is lost, who is affected, counts); optional type-to-confirm `mol-confirm-input` (required when blast radius `>= 10` records, deletes a workspace/account, or removes data for other users); footer: "Cancel" (default focus) + danger button with the exact verb ("Delete project").
- **Behaviour**: Danger button disabled until the confirm input matches exactly; pending state spinner; failure keeps the dialog open with inline error.
- **When not to use**: Reversible actions (archive, remove from list, status change) use undo toast instead (`org-feedback`).
- **Accessibility**: `alertdialog` role; initial focus on "Cancel"; consequence text referenced by `aria-describedby`.
- **Anti-patterns**: "Are you sure?" without naming the target; "OK"/"Yes" buttons; confirmation for non-destructive saves.

### `org-feedback` — Toast and Inline Feedback Model

- **Purpose**: Tell the user the result of every action through the right channel.
- **Channel rules**:

| Situation | Channel | Duration |
|-----------|---------|----------|
| In-place edit succeeded (toggle, inline field) | The changed value itself + optional inline "Saved" | `2s` inline |
| Action effect not visible on screen (sent email, created in background) | Toast success with link to the result | `4–6s` |
| Reversible destructive action (archive, remove, delete with soft-delete) | Toast with "Undo" | `5–8s`, paused on hover/focus |
| Form/field validation error | Inline at field + summary; never toast only | until fixed |
| Section load failure | `org-section-error` | until retried |
| Mutation failure from a toast-triggered action | Toast error with "Retry" | persistent until dismissed |
| Long-running job | Inline progress, then notification on completion | — |

- **Rules**: Max 3 toasts stacked; newest on top; toast text = past-tense outcome ("Invoice sent"); never toast on page load; toasts never contain forms.
- **Accessibility**: Toast region is a polite live region (`role="status"`); error toasts use assertive announcements only when the user triggered the action; toasts with actions do not auto-dismiss while focused.

### `org-section-error` — Section Error

- **Purpose**: Contain a failure to one region (`S4`, region-level `S5`).
- **Anatomy**: Icon, title ("Couldn't load {region}"), 1-line cause in plain language, "Retry" button, optional reference code; `no-access` variant: "You don't have access to {region}" + who can grant it.
- **Behaviour**: Retry re-runs only the failing request; occupies the region's normal height to avoid layout shift.
- **Accessibility**: Announced via `role="alert"` only after a user-triggered retry fails; initial render uses `status`.

### `org-empty-state` — Empty State Block

- **Purpose**: Turn an empty region into guidance (`S2`, `S3`, inbox zero).
- **Variants and formula**:
  - `first-use`: icon/illustration (optional, `<= 120px`), title "No {objects} yet", 1–2 sentence value statement ("{Objects} let you {benefit}."), primary "Create {object}", optional secondary "Import {objects}" or "Learn more".
  - `no-results`: title "No {objects} match your filters" / "No results for '{query}'", hint listing active filters, secondary "Clear filters" (never primary-filled, never "Create").
  - `done`: title "You're all caught up", last activity time, link to processed items.
  - `restricted`: shown when the user cannot create: explains who can, no create button.
- **Rules**: Centered in the region with the region's normal min height; in small regions (widgets, tabs) use a compact one-line variant with the action as a link.
- **Accessibility**: Title is a heading at the right level; illustration is decorative (`alt=""`).

---

## Data Display

### `org-kpi-row` — KPI / Stat Row

- **Purpose**: Summarize the few numbers that drive action.
- **Anatomy**: 3–5 tiles; tile = label (metric + period, "Revenue · last 30 days"), value (level-1 type, tabular figures, unit), delta (`+12.4% vs previous 30 days`, with arrow icon + sign + semantic color, where "good" direction is defined per metric), link/click to filtered detail.
- **Behaviour**: Each tile loads and fails independently; values abbreviate above 6 digits (`1.2M`) with full value in tooltip.
- **Accessibility**: Delta direction expressed in text ("up 12.4%"), not only arrow or color; tile is a link with a name including metric and value.
- **Anti-patterns**: More than 5 tiles; vanity metrics with no action; deltas without baseline.

### `org-chart-panel` — Chart Panel

- **Purpose**: Show a trend, distribution, or composition that answers a stated question.
- **Anatomy**: Card header (question-style or metric title, period, optional segmented control for granularity), chart, legend (only for `>= 2` series), footer with source/updated time and link to underlying data.
- **Chart choice**: trend over time → line (`<= 5` series) or bar for discrete periods; part-to-whole `<= 5` categories → stacked bar (pie only for `<= 4` slices); ranking → horizontal bar sorted descending; single value vs target → `org-usage-meter`-style progress.
- **Rules**: Axes labelled with units; y-axis starts at zero for bars; max 5 series; categorical colors from the chart palette, never status colors for non-status data; tooltip shows exact values; no 3D, no dual axes.
- **States**: Skeleton with chart geometry; `S3` "No data for {period}"; `S4` section error.
- **Accessibility**: Text summary of the key insight and a "View as table" toggle; series distinguishable without color (labels, markers, or patterns).

### `org-activity-feed` — Timeline / Activity Feed

- **Purpose**: Chronological history of events on an object or account.
- **Anatomy**: Items grouped by day; item = actor `mol-avatar-name`, sentence "{Actor} {verb past tense} {target}", relative time (absolute on hover), optional detail (diff `from → to`, comment excerpt); variant `log` adds IP/source and is denser.
- **Behaviour**: Newest first; "Load older" pagination; system actors labelled "System" or the integration name; changes to the same field within 5 minutes by the same actor may collapse into one entry.
- **States**: `S2` "No activity yet"; `S4` section error.
- **Accessibility**: Ordered list; timestamps in `time` elements with machine-readable value.

### `org-comment-thread` — Comment Thread

- **Purpose**: Discussion attached to a record.
- **Anatomy**: List of comments (avatar, name, time, body, edited marker, action menu: edit/delete for own comments); composer at the bottom (multiline input, mention support, "Comment" primary, `Ctrl/Cmd+Enter` submit).
- **Behaviour**: Optimistic post with pending indicator; failure keeps text in composer with retry; delete own comment → undo toast; mentions notify and render as links.
- **States**: `S2` "No comments yet. Start the discussion."; `S5` composer hidden with "You can view but not comment" when read-only.
- **Accessibility**: Composer labelled; new comments from others announced politely.

### `org-usage-meter` — Usage Meter

- **Purpose**: Show consumption against a limit.
- **Anatomy**: Label (resource), `mol-progress-bar`, text "{used} of {limit} {unit}" + percentage, reset date ("Resets on {date}"), action link ("Upgrade" / "Manage").
- **Thresholds**: `< 80%` neutral; `80–99%` warning color + "Approaching limit"; `>= 100%` danger + consequence text ("New uploads are paused").
- **Accessibility**: Progress role with `aria-valuenow/min/max` and value text; threshold state stated in text.

### `org-plan-comparison` — Plan Comparison

- **Purpose**: Choose a plan.
- **Anatomy**: Billing period toggle (monthly/annual with savings stated); 2–4 plan columns: name, 1-line audience, price with currency and period, primary CTA ("Switch to {plan}" / "Current plan" disabled), top 5 differentiating features; full comparison table below grouped by category with check/dash plus text values.
- **Rules**: Current plan marked; recommended plan highlighted by border and label, not by color alone; downgrade CTAs secondary style; prices exclude/include tax stated.
- **Accessibility**: Comparison table uses table semantics; check marks have text alternatives ("Included"/"Not included").
