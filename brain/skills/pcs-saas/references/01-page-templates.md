# 01 — SaaS Page Templates (`tpl-*`)

Every SaaS page is exactly one template from this catalog. A template fixes the ordered regions, the organisms that fill them, the primary action position, the density, the state matrix, and the responsive behaviour. The composer picks the template with `04-template-decision-tree.md`; the Stack Adapter maps each ID to concrete components. Organism IDs (`org-*`) are defined in `02-organisms.md`; molecule IDs (`mol-*`) in `03-molecules-and-patterns.md`.

---

## 0. Shared Conventions

### 0.1 State Codes

Every template and every data region declares all seven states. `S2`–`S6` extend the `pcs-core` 5 Mandatory Negative States; inline validation errors are covered by `mol-form-field`.

| Code | State | Trigger | Default Contract |
|------|-------|---------|------------------|
| `S1` | Loading | First fetch or refetch in flight | Skeleton matching final geometry (`mol-skeleton`); page header and filters render immediately; no full-page spinner. Refetch keeps stale data visible with a subtle progress indicator. |
| `S2` | Empty first-use | Collection total is `0` before any filter | `org-empty-state` variant `first-use`: what lives here, why it matters, one primary create action, optional secondary "Import" or "Learn more". Filter bar hidden. |
| `S3` | Empty no-results | Total `> 0`, active query/filters match `0` | `org-empty-state` variant `no-results`: names the query/filters, offers "Clear filters". Filter bar stays visible with active chips. Never offers "Create first". |
| `S4` | Error | Fetch fails (network, 5xx) | `org-section-error` inside the failing region only: plain-language cause, "Retry" action, optional reference code. Shell, header, and sibling regions stay usable. |
| `S5` | No permission | User lacks read or action rights | Read denied: `tpl-system-page` variant `no-access` (whole page) or `org-section-error` variant `no-access` (one region). Action denied: hide the action if the user can never gain it; disable it with a tooltip naming the required role if they can request it. |
| `S6` | Partial / degraded | Some data or a dependency is unavailable, stale, or limited (quota, sync lag, read-only mode) | `org-banner` variant `warning` at page top stating what is degraded, since when if known, and the effect; unaffected regions keep working. |
| `S7` | Success feedback | A mutation completes | Feedback model of `org-feedback`: inline change is the confirmation for in-place edits; toast for out-of-view effects; toast with "Undo" for reversible destructive actions; navigation to the created record for page-level creates. |

### 0.2 Density Codes

| Code | Row / control height | Use for |
|------|----------------------|---------|
| `comfortable` | Rows `48–56px`, controls `40px` | Occasional use, forms, settings, onboarding, auth |
| `compact` | Rows `36–40px`, controls `32–36px` | Daily data-heavy work: tables, queues, logs |

Density rules and spacing scale live in `05-visual-consistency.md`.

### 0.3 Breakpoints

`mobile < 640px`, `tablet 640–1023px`, `desktop >= 1024px`, `wide >= 1440px`. Every template defines desktop and mobile; tablet defaults to mobile rules for overlays and desktop rules for content unless stated.

---

## 1. `tpl-app-shell` — Application Shell (Global Layout)

- **Job**: Give every authenticated page a stable frame for navigation, identity, workspace context, and global tools.
- **Use when**: Always, for every authenticated page except `tpl-auth`, `tpl-onboarding` full-screen steps, and `tpl-system-page` variant `fatal`.
- **Do not use when**: Unauthenticated pages; focused full-screen flows (wizard, checkout) may use the minimal shell variant (top bar with logo and "Exit" only).
- **Anatomy**:
  1. Left navigation → `org-app-sidebar`.
  2. Top bar → `org-top-bar` (workspace switcher, global search trigger, notifications, account menu).
  3. Global banner slot → `org-banner` (trial ending, degraded service, impersonation); max one visible at a time, highest severity wins.
  4. Main content region → one page template.
  5. Overlay layer → `org-command-palette`, `org-notification-center`, toasts of `org-feedback`.
- **Primary action**: None in the shell; page primary actions live in `org-page-header`.
- **Density**: Shell chrome is fixed; never changes with page density.
- **States**: `S1` sidebar and top bar render from cached session, only main region shows skeletons. `S4` main region failure never removes navigation. `S5` nav items the user cannot access are hidden, not disabled. `S6` global banner. `S2`/`S3`/`S7` delegated to the page.
- **Responsive**: Desktop: sidebar expanded `240–280px`, collapsible to icon rail `56–64px`, collapse state persisted. Tablet: icon rail default. Mobile: sidebar becomes an off-canvas drawer opened by a menu button at top-left of the top bar; global search collapses to an icon.
- **Content width**: Main region max width per template (see `05-visual-consistency.md` §4); content is never stretched edge to edge beyond `1600px` except tables and boards.
- **Anti-patterns**: Second vertical sidebar inside a page; navigation that changes order per page; global search placed differently on different pages; more than one global banner stacked.
- **Acceptance checks**:
  - `AC-SHELL-1` Sidebar, top bar, and their item order are identical on every authenticated page.
  - `AC-SHELL-2` Current section is marked active in the sidebar with both a visual indicator and `aria-current="page"`.
  - `AC-SHELL-3` A failing main region leaves sidebar and top bar interactive.
  - `AC-SHELL-4` At `375px` width no horizontal scroll exists and navigation is reachable in 1 tap.
  - `AC-SHELL-5` Skip-to-content link is the first focusable element.

## 2. `tpl-index-table` — Index with Data Table

- **Job**: Find, scan, compare, and act on many records of one object type with `>= 4` comparable attributes.
- **Use when**: Cardinality `> 20` expected, users compare attributes across rows, sort/filter matter, bulk actions exist.
- **Do not use when**: Records are visual (images, previews) → `tpl-index-cards`; users process records one by one with a decision → `tpl-inbox-queue`; users need the detail open while scanning → `tpl-list-detail`.
- **Anatomy**:
  1. Header → `org-page-header` (title, count, description, primary "Create {object}", secondary "Import"/"Export" in overflow).
  2. Optional summary → `org-kpi-row` only if each tile filters the table (max 4 tiles).
  3. Toolbar → `org-filter-bar` (search, max 3 visible facets, "More filters", active chips, "Clear filters", view options).
  4. Content → `org-data-table` (5–7 default columns, sorting, row actions, selection).
  5. Bulk layer → `org-bulk-action-bar` visible only while `>= 1` row is selected.
  6. Footer → `mol-pagination`.
  7. Inspection → row click opens the record per the decision tree: `org-detail-panel` (sheet) for `< 12` attributes and no sub-collections; `tpl-record-detail` page otherwise.
- **Primary action**: Top-right of page header; exactly one solid button.
- **Density**: `compact` default; user toggle to `comfortable` persisted per user.
- **States**: `S1` header + filter bar render, table shows 8–10 skeleton rows with real column headers. `S2` replace toolbar+table with first-use empty state. `S3` keep toolbar and headers, empty state inside the table body. `S4` error inside table body, toolbar stays. `S5` page-level no-access; row actions without permission hidden. `S6` banner (e.g., "Results may be up to 10 minutes old"). `S7` created row appears highlighted for `2s` at top of current sort or toast with "View".
- **Responsive**: Desktop: full table. Tablet: hide Tier 4 columns, keep identity, status, one metric, actions. Mobile (`< 640px`): render each row as a stacked list item (identity, status badge, one metric, action menu); filters move into a "Filters" sheet with a count badge; bulk selection via long-press is forbidden, use an explicit "Select" mode.
- **Anti-patterns**: Bare header over blank body; more than 1 visible inline row action; filters in a left sidebar plus a toolbar; pagination and infinite scroll mixed; "Create" button inside the table footer.
- **Acceptance checks**:
  - `AC-INDEX-1` URL serializes query, filters, sort, page, and page size; reload restores the exact view.
  - `AC-INDEX-2` `S2` and `S3` render different copy and different actions.
  - `AC-INDEX-3` Default visible columns `<= 7`; identity in column 1; numeric columns right-aligned with tabular figures.
  - `AC-INDEX-4` Selecting a row shows the bulk bar with the selected count; clearing selection hides it.
  - `AC-INDEX-5` Returning from a record restores scroll position, filters, and page.

## 3. `tpl-index-cards` — Index with Card Grid

- **Job**: Browse and pick among records recognized visually or by a short summary (projects, templates, workspaces, integrations, files).
- **Use when**: Cardinality `<= 200`, recognition by image/icon/preview, `<= 3` attributes matter for choosing, little cross-record comparison.
- **Do not use when**: Users compare `>= 4` attributes or sort by numeric columns → `tpl-index-table`.
- **Anatomy**:
  1. Header → `org-page-header`.
  2. Toolbar → `org-filter-bar` (search + max 2 facets + sort select + optional grid/table view toggle).
  3. Content → `org-card-grid`.
  4. Footer → "Load more" button or `mol-pagination` when `> 48` items.
- **Primary action**: Page header top-right. Never a "Create" card as the first grid item when the header already holds "Create".
- **Density**: `comfortable`.
- **States**: `S1` 6–12 skeleton cards with final card dimensions. `S2` first-use empty state centered in content area, optional gallery of starter templates. `S3` no-results inside grid area, toolbar visible. `S4` section error replaces grid. `S5` hide cards the user cannot open; page-level no-access if none. `S6` banner; cards with unavailable data show a badge "Unavailable". `S7` new card inserted first with highlight.
- **Responsive**: Columns: wide 4, desktop 3, tablet 2, mobile 1; card min width `260px`.
- **Anti-patterns**: Cards with different heights in one row; more than 1 primary button per card; card content that is only an image with no text label.
- **Acceptance checks**:
  - `AC-CARDS-1` Every card has a visible text title and is a single focusable target (or has one primary link plus an action menu).
  - `AC-CARDS-2` Cards in a row share equal height; text truncates per `mol-truncation` rules.
  - `AC-CARDS-3` Grid reflows to 1 column at `375px` with no horizontal scroll.

## 4. `tpl-list-detail` — List-Detail Split (Master-Detail)

- **Job**: Move through records sequentially while reading or editing each one in full, without losing list position (tickets, messages, candidates, transactions under review).
- **Use when**: Users open `>= 5` records per session in sequence, detail is read-heavy, list items are identified by `<= 3` attributes.
- **Do not use when**: Comparing many columns → `tpl-index-table`; detail needs full width with tabs and sub-collections → `tpl-record-detail`.
- **Anatomy**:
  1. Header → `org-page-header` (compact variant, title + primary action).
  2. Left pane (`320–400px`, resizable optional) → `org-filter-bar` (compact: search + 1 facet) + selectable list (list items use `mol-avatar-name`, `mol-status-badge`, 1 metadata line).
  3. Right pane → `org-detail-panel` (inline variant) with its own header, actions, and content.
- **Primary action**: Page-level create in header; record actions in the detail pane header.
- **Density**: List `compact`; detail `comfortable`.
- **States**: `S1` list skeleton and detail skeleton independently. `S2` whole split replaced by first-use empty state. `S3` list shows no-results; detail pane shows "Select an item" placeholder. Nothing selected → placeholder with keyboard hint. `S4` list and detail fail independently. `S5` detail pane shows no-access if the record is restricted. `S6` banner. `S7` list item updates in place; status changes move the item only after the user navigates away.
- **Keyboard**: `ArrowUp`/`ArrowDown` or `j`/`k` move selection; `Enter` focuses detail; selected item reflected in URL (`?id=`).
- **Responsive**: Below `1024px` the split becomes two routes: list, then full-screen detail with a "Back" control preserving list scroll.
- **Anti-patterns**: Auto-selecting the first item when it marks it as read or triggers side effects; detail pane narrower than `480px` on desktop.
- **Acceptance checks**:
  - `AC-SPLIT-1` Selected item is highlighted, has `aria-selected="true"`, and is in the URL.
  - `AC-SPLIT-2` Keyboard navigation moves selection and updates the detail without page reload.
  - `AC-SPLIT-3` Mobile back returns to the list at the same scroll offset.

## 5. `tpl-record-detail` — Entity Detail / Record Page with Tabs

- **Job**: Understand one complex record completely and act on it (customer, project, invoice, deployment).
- **Use when**: Record has `>= 12` attributes, or `>= 1` related sub-collection, or its own activity history, or it is a destination users bookmark and share.
- **Do not use when**: Fewer than 12 attributes and no sub-collections → `org-detail-panel` in a sheet from the index.
- **Anatomy**:
  1. Breadcrumbs → `mol-breadcrumbs` (Index › Record).
  2. Record header → `org-page-header` variant `record`: identity (name, ID, avatar/icon), `mol-status-badge`, 2–4 key facts as `mol-key-value-list` inline, primary action, secondary actions (max 2 visible + overflow).
  3. Tabs → `org-tabs` (max 6; first tab "Overview").
  4. Overview tab: two-column layout — main column (`org-sectioned-form` read mode or summary cards, related sub-collection previews via `org-data-table` compact max 5 rows + "View all"), side column (`mol-key-value-list` properties, owners, dates).
  5. Other tabs: one sub-collection per tab (`org-data-table`), "Activity" (`org-activity-feed`), "Settings" (`org-sectioned-form`).
- **Primary action**: Record header top-right; it is the most frequent state transition (e.g., "Send invoice", "Approve"), not "Edit" when inline editing exists.
- **Density**: `comfortable`; sub-collection tables `compact`.
- **States**: `S1` header skeleton + active-tab skeleton; tabs load lazily, each with its own skeleton. `S2` per sub-collection empty states ("No invoices yet" + create action scoped to this record). `S3` per tab filter no-results. `S4` record not found → `tpl-system-page` `not-found`; tab-level fetch failure → `org-section-error` in that tab only. `S5` record restricted → `no-access`; tab restricted → tab hidden. `S6` banner (e.g., "This record is archived. Restore to edit."), edit controls disabled. `S7` inline edits confirm in place; state transitions update the badge and add an activity entry.
- **Responsive**: Side column stacks under main column below `1024px`; tabs become horizontally scrollable with visible overflow affordance; header actions collapse into one overflow menu except the primary.
- **Anti-patterns**: Tabs inside tabs; more than 6 tabs; "Edit" opening a page that duplicates the detail layout when inline or sheet editing suffices; status shown only by color.
- **Acceptance checks**:
  - `AC-RECORD-1` Active tab is in the URL (`?tab=`); reload restores it.
  - `AC-RECORD-2` Identity and status badge use the same label, color role, and format as in the index row.
  - `AC-RECORD-3` A failing tab does not break the header or other tabs.
  - `AC-RECORD-4` Archived/read-only state disables every mutation control and explains why.

## 6. `tpl-form-short` — Create/Edit Short Form (Dialog or Sheet)

- **Job**: Create or edit a record quickly without leaving the current context.
- **Use when**: `<= 6` fields (dialog, no background reference needed) or `<= 12` fields (sheet, background reference needed); single step; no file-heavy or rich-text content.
- **Do not use when**: `> 12` fields, more than 3 logical sections, or rich content → `tpl-form-long`; dependent sequential decisions → `tpl-wizard`.
- **Anatomy**:
  1. Container header: title "Create {object}" / "Edit {object name}", optional 1-line description, close control.
  2. Body → `org-sectioned-form` variant `single-section` (fields per `mol-form-field`).
  3. Footer: "Cancel" (secondary, left of primary) + primary verb button ("Create project"), right-aligned.
- **Primary action**: Footer right. `Enter` submits from single-line inputs; `Ctrl/Cmd+Enter` from multiline.
- **Density**: `comfortable`.
- **States**: `S1` edit mode: skeleton fields until data arrives, primary disabled. Submit pending: primary shows inline spinner and is disabled, fields read-only. `S2`/`S3` n/a. `S4` submit failure: `mol-inline-alert` at top of body with cause, values preserved; field-level server errors mapped to fields. `S5` opening is impossible without permission (trigger hidden/disabled). `S6` if a dependency list (e.g., owners) fails, that field shows an inline error with retry; the rest stays usable. `S7` close container, toast "{Object} created" with "View" link, new record highlighted in the list.
- **Dirty guard**: Closing with unsaved changes prompts "Discard changes?" (`Discard` / `Keep editing`); clean close is immediate.
- **Responsive**: Dialog becomes full-screen on mobile; sheet becomes bottom drawer or full-screen; footer sticks to the bottom.
- **Anti-patterns**: Dialog over dialog; scrolling dialog with `> 6` fields; "Save" and "Save and close" and "Save and new" all visible; clearing the form on error.
- **Acceptance checks**:
  - `AC-SHORT-1` First field autofocused on open; focus returns to the trigger on close.
  - `AC-SHORT-2` `Escape` closes a clean form and triggers the dirty guard on a modified form.
  - `AC-SHORT-3` Server error keeps all entered values and focuses the first invalid field or the alert.

## 7. `tpl-form-long` — Create/Edit Long Sectioned Form Page

- **Job**: Author or edit a complex record with many fields organized in logical sections.
- **Use when**: `> 12` fields, or `>= 3` logical sections, or rich content (editor, attachments, nested line items).
- **Do not use when**: Steps depend on earlier answers and must be validated in order → `tpl-wizard`.
- **Anatomy**:
  1. Breadcrumbs + `org-page-header` (title "New {object}" / object name; no primary in header when a sticky footer exists).
  2. Optional section index (anchor list) at left or right on `wide` screens only when `>= 5` sections.
  3. Body → `org-sectioned-form` (sections of 3–7 fields, each with heading and 1-line description).
  4. Sticky footer action bar: "Cancel" + primary "Create {object}" / "Save changes"; shows "Unsaved changes" indicator when dirty.
- **Primary action**: Sticky footer right; always visible.
- **Density**: `comfortable`; form column max width `720px` (single column).
- **States**: `S1` skeleton sections. Submit pending: primary spinner, fields read-only. `S4` submit error: summary `mol-inline-alert` at top listing errors as links to fields + inline field errors; focus moves to summary. `S5` read-only rendering if user can view but not edit, with explanation banner. `S6` autosave failure banner "Changes not saved. Retrying…" with manual "Save" fallback. `S7` navigate to the record page with toast "{Object} created"; on edit, stay and show "Saved" inline near the footer.
- **Dirty guard**: Route change and browser unload with unsaved changes prompt.
- **Responsive**: Single column at all sizes; section index hidden below `1440px`; footer stays sticky.
- **Anti-patterns**: Two-column field grids for unrelated fields; section headings without descriptions on technical forms; primary action only at the bottom of a long scroll.
- **Acceptance checks**:
  - `AC-LONG-1` Every section holds 3–7 fields; no section exceeds 7.
  - `AC-LONG-2` Submitting with errors moves focus to an error summary whose items link to fields.
  - `AC-LONG-3` Navigating away while dirty prompts; while clean does not.

## 8. `tpl-wizard` — Multi-Step Wizard

- **Job**: Complete a process whose later inputs depend on earlier answers, or that a novice performs rarely (setup, import mapping, provisioning).
- **Use when**: `3–5` steps where step N's options depend on step N-1, or a validation/preview must happen before commit.
- **Do not use when**: Steps are independent → `tpl-form-long` with sections; `<= 2` steps → single form with a preview.
- **Anatomy**:
  1. Minimal shell variant (logo + "Exit" with dirty guard) or page within shell.
  2. Stepper → `org-wizard-stepper` (step names, current, completed, upcoming).
  3. Step body → `org-sectioned-form` (one step = one section group, max 7 fields).
  4. Final step: review summary (`mol-key-value-list` per earlier step with "Edit" links).
  5. Footer: "Back" (left), "Continue" (primary, right); last step primary is the commit verb ("Create workspace").
- **Primary action**: Footer right.
- **Limits**: Max 5 steps (7 only for one-time setup with explicit progress); each step completable in `< 2 minutes`.
- **States**: `S1` step-level skeleton when options load. Commit pending: progress state with what is happening; never a blank screen. `S4` step validation errors inline; commit failure returns to the review step with alert and preserved inputs. `S5` permission check before step 1. `S6` partial commit (some items failed) → result screen listing successes and failures with retry for failures. `S7` completion screen with outcome summary and next action.
- **Persistence**: Progress saved per step (draft) when the wizard exceeds 3 steps; returning resumes at the last incomplete step.
- **Responsive**: Stepper collapses to "Step 2 of 4 — {name}" on mobile.
- **Anti-patterns**: Validating all steps only at the end; losing data on "Back"; disabling "Continue" without saying why; a wizard for a single form.
- **Acceptance checks**:
  - `AC-WIZ-1` Step count `<= 5` and visible progress shows current step and total.
  - `AC-WIZ-2` "Back" preserves every entered value.
  - `AC-WIZ-3` Review step lists all answers with edit links to their steps.

## 9. `tpl-settings` — Sectioned Settings

- **Job**: Configure workspace, account, or feature preferences that change rarely.
- **Use when**: Any configuration surface; group by audience (Personal, Workspace, Billing, Developers) then by topic.
- **Do not use when**: Configuration is part of creating an object → the object's form.
- **Anatomy**:
  1. Settings navigation: a secondary vertical list of setting groups on the left of the content (this replaces, not nests within, page tabs; the app sidebar shows "Settings" as active) — max 2 levels total.
  2. `org-page-header` per settings page (title + description).
  3. Body → `org-sectioned-form` with cards per topic; each card has heading, description, fields, and its own save model.
  4. Danger zone card last → destructive actions using `org-destructive-confirm`.
- **Save model** (pick one per page, never mix in one card):
  - `instant`: toggles and selects that apply on change; confirm with inline "Saved" text near the control for `2s`; failure reverts the control and shows inline error.
  - `per-card`: card footer with "Save" enabled only when the card is dirty; "Cancel" restores values.
  - Never a single page-bottom "Save" for multiple independent cards.
- **Density**: `comfortable`; content max width `720px`.
- **States**: `S1` card skeletons. `S4` per-card load failure. `S5` read-only fields with "Only admins can change this" hint; groups the user cannot see are hidden. `S6` banner for pending changes awaiting propagation. `S7` per save model above.
- **Responsive**: Settings navigation becomes a select or a list page that drills into each group on mobile.
- **Anti-patterns**: Mixed instant and explicit save in one card; destructive actions mixed into regular cards; settings split across unrelated pages.
- **Acceptance checks**:
  - `AC-SET-1` Each card declares one save model; instant controls show "Saved" confirmation; explicit cards disable "Save" when clean.
  - `AC-SET-2` Danger zone is the last section and every destructive action requires confirmation naming the target.
  - `AC-SET-3` Leaving a dirty per-card form prompts.

## 10. `tpl-dashboard` — Dashboard / Overview

- **Job**: Answer "what needs my attention now and how are we doing" within 5 seconds of landing.
- **Use when**: Users start their session here or monitor health/progress across objects.
- **Do not use when**: A single object list answers the question → `tpl-index-table` with a KPI row.
- **Anatomy**:
  1. `org-page-header` (greeting or workspace name, period selector `mol-date-range` at right; no create primary unless one dominant action exists).
  2. `org-kpi-row` (3–5 tiles, each with value, delta vs explicit baseline, link to filtered detail).
  3. Attention list: compact `org-data-table` or list "Needs your action" (max 5 rows + "View all").
  4. `org-chart-panel` × 1–4 in a 12-column grid (each answers one named question).
  5. Optional `org-activity-feed` (recent, max 10 items).
- **Ordering rule**: Top-left = most urgent actionable info; charts below KPIs; decorative content forbidden.
- **Density**: `comfortable` cards; lists inside `compact`.
- **States**: `S1` every widget skeletons independently, matching final height. `S2` new workspace: replace widgets with `tpl-onboarding` checklist variant or per-widget "No data yet" with the action that produces data. `S3` period with no data: "No data for {period}" inside each widget, period selector visible. `S4` per-widget error with retry; others render. `S5` widgets the user cannot access are omitted. `S6` stale data indicator "Updated 2h ago" per widget. `S7` n/a (read-only).
- **Responsive**: 12-col desktop (KPIs 3–4 per row, charts 6 or 12 cols), tablet 2 per row, mobile 1 per row; KPI tiles become a 2-col grid on mobile.
- **Anti-patterns**: More than 8 widgets; charts without units, baselines, or axis labels; KPI tiles not clickable; one widget failure blanking the page.
- **Acceptance checks**:
  - `AC-DASH-1` Every KPI shows unit, period, and comparison baseline, and links to a filtered list.
  - `AC-DASH-2` Each widget renders its own loading, empty, and error state.
  - `AC-DASH-3` Widget count `<= 8`; chart count `<= 4`.

## 11. `tpl-inbox-queue` — Inbox / Approval Queue

- **Job**: Process incoming items to a decision (approve, reject, assign, resolve) as fast as possible.
- **Use when**: Each item requires a decision by the current user; throughput matters; items arrive continuously.
- **Do not use when**: Users only browse → index templates.
- **Anatomy**:
  1. `org-page-header` with queue counts per status as `org-tabs` (e.g., "Pending 12", "Approved", "Rejected"), max 5.
  2. Layout = `tpl-list-detail` split: queue list (oldest-first or priority-first, explicit sort label) + `org-detail-panel` with full context.
  3. Decision bar pinned in the detail pane: decision buttons (primary = most common decision), optional reason field required for rejections.
  4. Optional `org-bulk-action-bar` for batch decisions when items are homogeneous.
- **Primary action**: Decision bar; after a decision, auto-advance to the next item (setting persisted, default on).
- **Keyboard**: Single-key shortcuts for decisions shown in button tooltips, plus `j`/`k` navigation.
- **Density**: `compact` list.
- **States**: `S1` list + detail skeletons. `S2` "You're all caught up" (inbox zero) with last-processed time and link to processed items — positive tone, no create action. `S3` no-results for filters. `S4` decision failure: item stays, inline alert, decision retried manually. `S5` items needing a different role are visible read-only with "Waiting on {role}". `S6` banner "New items available" (never auto-insert above the selection). `S7` toast "{Decision} — {item}" with "Undo" for `5–8s`; item leaves the pending list.
- **Responsive**: Mobile: list route → item route with sticky decision bar at the bottom.
- **Anti-patterns**: Confirmation dialog for routine approvals; reordering the list under the cursor; decisions only via row menus.
- **Acceptance checks**:
  - `AC-QUEUE-1` After a decision, the next item opens automatically and undo is offered.
  - `AC-QUEUE-2` Empty pending state is distinct from no-results and has no create action.
  - `AC-QUEUE-3` Every decision is reachable by keyboard.

## 12. `tpl-onboarding` — Onboarding / First Run

- **Job**: Bring a new user or workspace from signup to the first meaningful outcome ("activation") with the fewest required inputs.
- **Use when**: First session of a user or workspace; a feature's first use requiring setup.
- **Variants**: `welcome-steps` (full-screen, 2–4 steps: profile, workspace, invite, first object) and `checklist` (in-app card listing 3–6 setup tasks with completion state, dismissible after completion).
- **Anatomy**:
  1. `welcome-steps`: minimal shell, `org-wizard-stepper` (compact dots or "Step 1 of 3"), one question per step, "Skip" for optional steps.
  2. `checklist`: card at the top of `tpl-dashboard` with progress (`mol-progress-bar` "2 of 5 done"), each task links to the place where it is completed.
- **Primary action**: Continue/the task's verb; skipping never blocks access to the product.
- **Rules**: Ask only what changes the first experience (max 4 questions); prefill from signup data; never require invites to proceed; sample data, if offered, is labelled "Sample" and removable in 1 action.
- **States**: `S1` per step. `S4` step failure keeps input, retry. `S6` if an integration step fails, allow "Do this later". `S7` completion lands on the populated product with a toast or celebratory inline message (no modal blocking).
- **Responsive**: Single column, max width `480–560px`.
- **Anti-patterns**: Product tours with `> 5` tooltips; mandatory video; onboarding modal over an empty dashboard; checklist that cannot be dismissed.
- **Acceptance checks**:
  - `AC-ONB-1` Required steps `<= 4`; every optional step has "Skip".
  - `AC-ONB-2` Each checklist item links to where it is completed and reflects completion state.

## 13. `tpl-auth` — Authentication Pages

- **Job**: Sign in, sign up, recover access, verify identity with minimal friction and maximal clarity.
- **Pages**: `sign-in`, `sign-up`, `forgot-password`, `reset-password`, `verify-email`, `mfa-challenge`, `accept-invite`, `sso-select`.
- **Anatomy**:
  1. Centered card `400–440px` on a neutral canvas; product logo above.
  2. Title (verb phrase: "Sign in to {product}") + 1-line context.
  3. `org-auth-form`: identity provider buttons first if offered (max 3), divider "or", email, password, primary submit.
  4. Secondary links below the card: "Forgot password?", "Create account" / "Sign in".
  5. Footer: terms and privacy links.
- **Rules**: Single column; email field `type=email` with `autocomplete`; password field with show/hide toggle; sign-up shows password requirements before typing and live-checks them; never ask to confirm the password twice when show/hide exists.
- **Security copy**: Errors never reveal whether an account exists ("Email or password is incorrect"); recovery always says "If an account exists, we sent a link".
- **States**: `S1` submit pending spinner in button. `S4` credential error inline above the form; network error with retry. `S6` rate-limited: state wait time. `S7` redirect to the originally requested URL (`returnTo`).
- **Responsive**: Card becomes full-width with `16px` gutters on mobile.
- **Anti-patterns**: Clearing the email on error; CAPTCHA before any failed attempt; disabling paste in password fields.
- **Acceptance checks**:
  - `AC-AUTH-1` Inputs have `autocomplete` (`email`, `current-password`, `new-password`, `one-time-code`).
  - `AC-AUTH-2` Failed sign-in keeps the email value and focuses the error.
  - `AC-AUTH-3` After sign-in the user lands on the URL they originally requested.

## 14. `tpl-billing` — Billing, Plan and Usage

- **Job**: Understand current plan, cost, and consumption; change plan; manage payment methods and invoices.
- **Anatomy** (settings sub-page within `tpl-settings` navigation):
  1. `org-page-header` "Billing".
  2. Current plan card: plan name, price with period, renewal date, seat count, primary "Change plan".
  3. `org-usage-meter` × N (one per metered resource), each with limit and reset date.
  4. Payment method card: brand + last 4 digits + expiry, "Update".
  5. Invoices → `org-data-table` compact (date, number, amount, status badge, download action).
  6. Plan change → `org-plan-comparison` (page or full-screen dialog), then confirmation summary showing prorated amount and effective date.
- **Primary action**: "Change plan" on current plan card; "Upgrade" in usage meters at `>= 80%`.
- **Friction**: Downgrade/cancel = Tier C confirmation naming lost features and effective date; upgrade = summary with exact charge before commit.
- **States**: `S1` skeletons. `S2` no invoices yet. `S4` payment provider failure → section error per card. `S5` non-billing-admins see plan read-only and "Contact a billing admin". `S6` failed payment banner (danger) with "Update payment method"; trial banner with days left. `S7` plan change confirmed inline + email notice copy.
- **Responsive**: Cards stack; invoices table becomes list.
- **Anti-patterns**: Hidden cancel path; prices without period and currency; usage without limit.
- **Acceptance checks**:
  - `AC-BILL-1` Every price shows currency and period; every meter shows used, limit, and reset date.
  - `AC-BILL-2` Plan change shows the exact amount charged and effective date before commit.

## 15. `tpl-activity-log` — Activity / Audit Log

- **Job**: Answer "who did what, when, to which object" for compliance and troubleshooting.
- **Anatomy**:
  1. `org-page-header` (title, export action in secondary).
  2. `org-filter-bar`: actor, action type, object type, date range (required default "Last 7 days").
  3. Content: `org-activity-feed` variant `log` (grouped by day) or `org-data-table` (timestamp, actor, action, target, IP/source) when users filter and export.
  4. Entry inspection → `org-detail-panel` with before/after diff.
  5. `mol-pagination` (cursor-based "Load older").
- **Density**: `compact`; timestamps absolute with timezone on hover/inspection, relative in list.
- **States**: `S2` "No activity recorded yet". `S3` no events for filters/date range with "Expand date range". `S4` section error. `S5` admin-only page; others get `no-access`. `S6` "Logs older than {retention} are not available on your plan".
- **Rules**: Read-only; entries immutable; actor links to the user; target links to the object (or "deleted object" label).
- **Acceptance checks**:
  - `AC-LOG-1` Every entry shows actor, action verb, target, and timestamp with timezone available.
  - `AC-LOG-2` Date range filter is always present and in the URL.

## 16. `tpl-search-results` — Global Search Results

- **Job**: Find any object across types when the user knows a name, ID, or keyword.
- **Anatomy**:
  1. Search input prefilled with query (`mol-search-input`), result count.
  2. Type facets as `org-tabs` ("All 42", "Customers 12", "Invoices 30"), max 6.
  3. Results grouped by type (All tab: max 5 per group + "See all {n}"), each result: icon by type, title with matched text highlighted, 1 metadata line, status badge.
  4. `mol-pagination` per type tab.
- **Rules**: Exact ID match ranks first; recent items shown before typing in the palette variant (`org-command-palette`); query in URL.
- **States**: `S1` skeleton results. Empty query: recent searches and recently viewed. `S3` "No results for '{query}'" with spelling-tolerant suggestion and tips (check spelling, try fewer words). `S4` error with retry. `S5` results the user cannot open are excluded, never shown locked.
- **Acceptance checks**:
  - `AC-SRCH-1` Query and type facet live in the URL.
  - `AC-SRCH-2` Matched text is highlighted with a non-color indicator (weight or underline).

## 17. `tpl-import-export` — Import / Export Flow

- **Job**: Move data in or out in bulk safely.
- **Import anatomy** (`tpl-wizard` specialization, 4 steps): 1 Upload (`org-file-dropzone`, accepted formats, size limit, template download link) → 2 Map columns (source column → field, auto-matched, required fields marked) → 3 Review (counts: will create / will update / will skip with reasons, first 10 rows preview, row-level errors downloadable) → 4 Import (progress with counts; can leave the page, notification on completion).
- **Export anatomy**: short form in a dialog: scope (current filters vs all, with counts), format, columns (default visible columns); large exports run async with notification and a download link valid for a stated period.
- **Friction**: Import is Tier B (review step is the confirmation); updates overwriting existing data state the count before commit.
- **States**: `S4` file parse errors at upload step with line numbers. `S6` partial import: result lists succeeded/failed counts and a downloadable error file. `S7` "Imported {n} {objects}" with "View".
- **Acceptance checks**:
  - `AC-IMP-1` Review step shows create/update/skip counts before commit.
  - `AC-IMP-2` Partial failures are downloadable with row numbers and reasons.
  - `AC-EXP-1` Export dialog states which records (filtered vs all) and how many.

## 18. `tpl-system-page` — Error, Not Found, No Access

- **Variants**: `not-found` (404, deleted or mistyped), `no-access` (403), `server-error` (500 or unrecoverable), `offline`, `maintenance`, `fatal` (shell itself failed).
- **Anatomy**: Inside the app shell (except `fatal`): centered block with icon, title, 1–2 sentence explanation, primary recovery action, secondary action, and support reference code for `server-error`.
- **Copy per variant**:
  - `not-found`: "This {object} doesn't exist or was deleted." Primary: "Go to {index}".
  - `no-access`: "You don't have access to this {object}." Explain who can grant it; primary "Request access" if supported, else "Go to dashboard".
  - `server-error`: "Something went wrong on our side." Primary "Try again"; reference code copyable.
  - `offline`: "You're offline. Changes will sync when you reconnect." (if supported).
- **Acceptance checks**:
  - `AC-SYS-1` Every variant offers at least one navigation action out; no dead ends.
  - `AC-SYS-2` Shell navigation remains available except in `fatal`.
  - `AC-SYS-3` Copy never blames the user and never shows raw stack traces or status codes alone.
