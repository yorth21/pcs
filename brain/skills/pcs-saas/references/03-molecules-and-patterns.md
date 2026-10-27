# 03 — Molecules and Micro-Patterns (`mol-*`)

Molecules are the smallest composed units the composer names in a Page Composition Spec. Each rule is checkable on a rendered screen. Formatting and copy rules apply everywhere, including inside organisms.

---

## 1. `mol-form-field` — Form Field Anatomy

Order, top to bottom: **label** → **hint** (optional) → **control** → **error** (conditional).

| Part | Rule |
|------|------|
| Label | Always visible above the control (never placeholder-only); sentence case; `<= 4` words; no trailing colon. |
| Required marking | Mark the minority: if most fields are required, append "(optional)" to optional labels; otherwise mark required with "*" plus a form-level legend "* Required". Never both. |
| Hint | One line below the label (or below the control for format examples): format, constraints, or consequence ("Shown on invoices"). Persistent, never only a tooltip. |
| Placeholder | Example value only ("e.g. Acme Inc."), never instructions, never a substitute for the label. |
| Control width | Matches expected content length: postal code/short codes `~8ch`, dates `~14ch`, names/emails full column width. |
| Error | Below the control, error color + icon + text; text formula in `05-visual-consistency.md` §9.4; replaces nothing (hint stays). |
| Disabled vs read-only | Read-only for values the user can see but not change (selectable text); disabled only for temporarily unavailable inputs, with a reason in the hint. |

Accessibility: label bound to control; hint and error referenced by `aria-describedby`; invalid control has `aria-invalid="true"`; required control has `aria-required` or native `required`.

## 2. Inline Validation Timing

| Event | Rule |
|-------|------|
| While typing (first time) | Do not show errors. Exception: live requirement checklists (password rules) and character counters. |
| On blur | Validate format and required-ness of that field; show error. |
| After an error is shown | Revalidate on every change; remove the error as soon as the value is valid. |
| On submit | Validate all; show all field errors + error summary (forms with `> 5` fields); focus first invalid field (or summary). |
| Async checks (uniqueness, availability) | Run on blur or `500ms` after typing stops; show "Checking…" inline; never block typing. |
| Server errors | Map to fields when attributable; otherwise `mol-inline-alert` at form top; keep all values. |

Never disable the submit button to signal invalid input; let the user submit and show errors. Disable it only while a submit is pending.

## 3. `mol-status-badge` — Status Badge Semantics

- Content: text label (1–2 words, sentence case) + optional leading dot or icon. Text is mandatory; color is never the only signal.
- One status vocabulary per object type; every status maps to exactly one semantic role:

| Role | Meaning | Example labels |
|------|---------|----------------|
| `neutral` | Inactive, draft, not started | Draft, Inactive, Archived |
| `info` | In progress, informational | Processing, Scheduled, In review |
| `success` | Completed, healthy, positive terminal | Active, Paid, Completed |
| `warning` | Needs attention soon, at risk | Pending approval, Expiring, Overdue soon |
| `danger` | Failed, blocked, negative terminal | Failed, Overdue, Suspended |

- Max 7 statuses per object; the same status renders identically in tables, cards, headers, and panels.
- Badges are not buttons; if status is editable, use a select styled distinctly from the badge.

## 4. `mol-avatar-name` — Avatar + Name

- Avatar sizes: `20px` (inline/dense), `24px` (table rows), `32px` (lists, comments), `40px+` (headers).
- Fallback: initials (max 2 letters) on a deterministic background derived from the user/workspace ID; generic icon for system actors.
- Name follows the avatar with `8px` gap; secondary line (email/role) only in lists and pickers.
- Groups: show max 3–5 avatars stacked then "+{n}" with full list in tooltip/popover.
- Avatar images have `alt=""` when the name is adjacent; otherwise `alt="{name}"`.

## 5. `mol-key-value-list` — Key-Value List

- Label column (secondary color, level-3 type) and value column (primary color, level-2 type).
- Layout: two columns (label left `~35%`, value right) in panels `>= 480px` wide; stacked (label above value) below that.
- Group into sections of `<= 8` pairs with headings.
- Empty value renders "—" (accessible text "Not set"); copyable values (IDs, keys) render with `mol-copyable-value`.
- Labels identical to the form field labels for the same attribute.

## 6. Formatting Rules (Money, Dates, Numbers)

All formatting is locale-aware through the platform's internationalization formatters, using the user's locale and timezone (workspace defaults as fallback).

| Data | Rule |
|------|------|
| Currency | Always symbol or ISO code + amount with the currency's minor units ("€1,240.50", "USD 1,240.50" when multiple currencies appear). Negative amounts with minus sign (or parentheses in accounting contexts, consistently). Right-aligned, tabular figures. |
| Large numbers | Thousands separators always. Abbreviate (`12.4K`, `1.2M`) only in KPI tiles and charts, with the full value in a tooltip. |
| Percentages | 0–1 decimal places consistently per metric; sign for deltas (`+4.2%`). |
| Dates | Relative ("5 min ago", "Yesterday") for events `< 7 days` old in feeds and lists; absolute ("Mar 4, 2026") otherwise. Absolute value with time and timezone always available on hover/inspection. Never ambiguous numeric-only formats like `03/04/26`. |
| Date ranges | "Mar 4 – Mar 10, 2026" (en dash, year once). |
| Durations | `1h 24m`, `3d`; never raw seconds. |
| Times | User's clock preference; timezone label when actors span timezones. |
| IDs / codes | Monospace or tabular figures, copyable; truncate the middle for long hashes (`a1b2…9f0e`). |
| Unknown / not applicable | "—" (not set) vs "N/A" (not applicable) vs "0" (true zero); never blank. |

## 7. `mol-truncation` and `mol-tooltip`

- Single-line truncation with ellipsis for table cells and card metadata; card titles may clamp at 2 lines.
- Truncated text exposes the full value in a tooltip on hover and keyboard focus, and in the detail view.
- Never truncate: status labels, monetary amounts, dates, button labels, error messages.
- Tooltips: max 1 sentence (`<= 80` characters), appear after `300–500ms` hover or immediately on focus, dismiss on `Escape`; never contain interactive content or essential information unavailable elsewhere; every icon-only button has a tooltip equal to its accessible name.

## 8. `mol-action-menu` — Action Menus and Overflow

- Visible action budget per container: page header 1 primary + 2 secondary; table row 1 inline; card 0–1 inline; panel header 1 primary + 1 secondary; bulk bar 3.
- Everything beyond the budget goes into one overflow menu (icon button "More actions"/"Actions for {identity}").
- Menu order: most frequent first; group with separators (view/edit, organize, share/export, destructive); destructive items last with danger text color.
- Max 9 items per menu; never nested submenus deeper than 1 level.
- Menu item labels: verb-first ("Duplicate", "Move to…" with ellipsis when it opens another step).
- Unavailable items: hidden if the user can never perform them; disabled with a reason (tooltip or secondary text) if they are temporarily unavailable.

## 9. `mol-button-group` — Button Hierarchy

| Level | Use | Max per region |
|-------|-----|----------------|
| Primary (solid accent) | The single most important action of the region | 1 per region (page header, dialog/sheet footer, card footer, empty state) |
| Secondary (outlined/subtle fill) | Alternative actions, "Cancel" | 2 visible |
| Tertiary (text/ghost) | Low-emphasis actions, links-like actions | 3 visible |
| Danger (solid danger) | Only inside `org-destructive-confirm` | 1 |
| Danger subtle (danger text) | Destructive entry points (menus, danger zone) | — |

- Order in footers: secondary/cancel left of primary, both right-aligned; destructive entry points left-aligned and separated.
- Labels verb-first and specific ("Create invoice", not "Submit"/"OK"); same action = same label everywhere.
- Icon + label for primary actions is optional; icon-only only for universally understood actions (close, more, search, edit inline) and always with accessible name and tooltip.
- Pending state: spinner inside the button, label kept or changed to progressive verb ("Saving…"), button disabled, width unchanged.

## 10. `mol-search-input` — Search Input Behaviour

- Leading search icon; placeholder "Search {objects}…" naming searchable fields when not obvious ("Search by name or email").
- Debounce `250–300ms` for remote search; immediate for local filtering of `< 500` items.
- Clear button ("×", labelled "Clear search") appears when non-empty; `Escape` clears the value when focused, a second `Escape` blurs.
- Minimum query length 2 characters for remote search (show nothing new before that).
- Query persists in the URL (`?q=`); pressing `Enter` never submits a page form.
- Keyboard shortcut `/` focuses the page search when no input is focused (shown in placeholder hint).

## 11. `mol-pagination`

- Content: "Showing {from}–{to} of {total}" (or "{n} results" with cursor pagination), page size select (10/25/50/100, default 25), previous/next buttons, page numbers with ellipsis (max 7 slots) when total is known.
- Page size and page in URL; changing page scrolls the table top into view.
- Cursor ("Load more") when total is unknown or `> 10,000`.
- Disabled previous/next at bounds remain visible.

## 12. `mol-skeleton` vs Spinner

| Situation | Indicator |
|-----------|-----------|
| Initial load of a region with known layout | Skeleton matching final geometry (same heights, column widths, number of rows ~ page size up to 10) |
| Refetch with data present | Keep data; thin progress bar at region top or subtle spinner in toolbar |
| Button-triggered mutation | Spinner inside the button |
| Unknown-duration background job | Progress bar with counts or determinate percentage; indeterminate only when no progress info |
| Region load `< 300ms` | Delay showing any indicator by `200–300ms` to avoid flicker |
| Full-page centered spinner | Forbidden after the shell has rendered |

Skeletons have reduced-motion-safe shimmer (static when reduced motion is requested) and are hidden from assistive technology, with one polite "Loading {region}" announcement.

## 13. `mol-filter-chip` and `mol-facet-filter`

- Facet filter: button "{Attribute}" → popover with searchable option list (search shown when `> 8` options), checkboxes for multi-select, radio for single-select, counts per option, "Clear" inside.
- Chip: "{Attribute}: {value}" or "{Attribute}: {n} selected", remove "×"; clicking the chip reopens the facet.
- Chips appear in the order filters were applied; "Clear filters" follows the last chip.

## 14. Other Molecules

- `mol-breadcrumbs`: Shown only at depth `>= 2`; current page last and not a link; max 4 items, middle items collapse into "…" menu; separator "›" or "/".
- `mol-inline-alert`: Icon + title (optional) + message + optional action; severity roles `info`, `success`, `warning`, `danger`; placed at the top of the region it concerns; not dismissible when it describes a blocking condition.
- `mol-date-range`: Presets first (Today, Last 7 days, Last 30 days, This month, Custom); custom opens a two-month calendar with typed input fallback; selected range shown as text in the trigger.
- `mol-progress-bar`: Determinate whenever progress is known; value text adjacent ("62%" or "3 of 5"); threshold colors only via semantic roles.
- `mol-confirm-input`: Instruction "Type {exact name} to confirm"; case-sensitive exact match; paste allowed.
- `mol-copyable-value`: Value + copy icon button; on copy, the icon changes to a check and announces "Copied" for `2s`; secrets masked by default with "Reveal" toggle.

## 15. Optimistic Updates, Undo, and Confirm

| Action type | Pattern | Rule |
|-------------|---------|------|
| Reversible, low impact (toggle, rename, status change, reorder, archive) | Optimistic update | Apply immediately; on failure revert and show error with retry. |
| Removal that the system can restore (archive, soft delete, remove from list) | Undo toast | Remove immediately; toast with "Undo" for `5–8s`; commit after the window or on navigation. |
| Irreversible or affects others (hard delete, revoke access, send to external party, charge money) | Confirmation (`org-destructive-confirm`) | Name the target and consequence; type-to-confirm per blast-radius rule. |
| Creation of significant records | Explicit submit with pending state | No optimistic insert unless the server rarely rejects and the record can be shown as "Saving…". |

Never combine confirm and undo for the same action; never use confirmation for reversible actions; never show optimistic state for money movement.
