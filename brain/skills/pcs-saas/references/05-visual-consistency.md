# 05 — Visual Consistency Rules (Stack-Agnostic)

These rules make every screen of a SaaS product look like one system. They refine `pcs-core` `references/03-plane-surface-and-visual-hierarchy.md` (60-30-10 color budget, 3-level type, whitespace floor) with concrete scales. The Stack Adapter maps each scale step and role to the project's design tokens; when the project already defines tokens, use the project's tokens and record the mapping in `pcs/design/inventory.md`.

---

## 1. Spacing Scale and Rhythm

- Base unit `4px`. Allowed steps: `4, 8, 12, 16, 20, 24, 32, 40, 48, 64`. No other values (no `10px`, `15px`, `18px`).
- Proximity ladder (each level strictly larger than the previous):

| Relationship | Spacing |
|--------------|---------|
| Icon ↔ its label, label ↔ input | `4–8px` |
| Related controls in a group, field ↔ next field | `12–16px` |
| Card internal padding | `16–24px` (compact `12–16px`) |
| Section ↔ section, card ↔ card | `24–32px` |
| Page header ↔ first content region | `24–32px` |
| Page outer gutter | desktop `24–32px`, mobile `16px` |

- One rhythm per page: sibling cards share padding; sibling sections share gaps.
- Whitespace floor: at least `25%` of the viewport is empty space (from `pcs-core`), measured on the default desktop viewport `1440×900`.

## 2. Type Scale and Hierarchy

| Role | Size / weight | Use |
|------|---------------|-----|
| Page title | `20–24px` / semibold | One per page (`h1`) |
| Section title | `16–18px` / semibold | Card and section headings |
| Body / control | `14px` / regular; labels and identity cells medium | Default text, table cells, inputs, buttons |
| Supporting | `12–13px` / regular, secondary color | Hints, metadata, timestamps, table headers |
| Display (KPI values only) | `24–32px` / semibold, tabular figures | KPI tiles |

- Max 4 distinct sizes per page (including display) and max 3 levels per region (from `pcs-core`).
- Line length for prose `45–80` characters; line height `1.4–1.6` body, `1.2–1.3` headings.
- One font family for UI (a second only for code/monospace values).
- Weights: regular, medium, semibold only; never bold-italic or all-caps body text; all-caps allowed only for overline labels `<= 2` words with `+0.04em` tracking.

## 3. Density Modes

| Property | `comfortable` | `compact` |
|----------|---------------|-----------|
| Table row height | `48–56px` | `36–40px` |
| Control height | `40px` | `32–36px` |
| Card padding | `24px` | `16px` |
| Body text | `14px` | `13–14px` |
| Minimum pointer target | `36×36px` desktop, `44×44px` touch (both modes) | same |

- Density is a property of the page template (`01-page-templates.md`), never mixed within one table.
- Touch devices always use `comfortable` targets regardless of mode.

## 4. Alignment Grid and Content Widths

- 12-column grid, gutter `24px` desktop, `16px` mobile; every region edge snaps to a column edge.
- One left alignment axis per page: page title, filter bar, table first column, and section headings start at the same x.

| Template | Max content width |
|----------|-------------------|
| `tpl-index-table`, `tpl-activity-log`, `tpl-inbox-queue`, `tpl-list-detail` | Fluid to `1600px` |
| `tpl-index-cards`, `tpl-dashboard`, `tpl-search-results` | `1280–1440px` |
| `tpl-record-detail` | `1280px` (main column `<= 840px`, side column `280–360px`) |
| `tpl-form-long`, `tpl-settings`, `tpl-billing`, `tpl-wizard` | Form column `640–720px` |
| `tpl-auth`, `tpl-onboarding`, `tpl-system-page` | `400–560px` centered |

- Overlays: dialog `400–560px` (form) or `<= 720px` (content); sheet `480–640px`.

## 5. Semantic Color Roles

| Role | Use | Never |
|------|-----|-------|
| `neutral` (canvas, surface, border, text primary/secondary) | `~90%` of pixels | — |
| `accent` | Primary buttons, active nav, links, focus ring, selected state | Decoration, large backgrounds |
| `success` | Positive status, successful completion | Primary buttons |
| `warning` | Needs attention, approaching limits | Errors |
| `danger` | Errors, failed status, destructive actions | Non-destructive emphasis |
| `info` | In-progress, informational notices | Links |
| `chart-1…6` | Categorical data series | Status meaning |

- Never color-only meaning: every status color pairs with text and/or icon; every chart series pairs with label or marker.
- Contrast: text `>= 4.5:1`, large text and UI component boundaries/focus indicators `>= 3:1`, in light and dark themes.
- Status surfaces use tinted backgrounds (`10–15%` tint) with full-strength text/icons; never saturated full-card fills.
- One accent color; hover/pressed are darker/lighter steps of the same hue.

## 6. Elevation and Borders

- Levels: `0` canvas, `1` surfaces (cards, tables, panels: `1px` border, no or minimal shadow), `2` sticky elements (subtle shadow on scroll only), `3` overlays (border + shadow + backdrop for modal).
- Use border **or** background shift to separate regions, not both plus shadow.
- Corner radius: one scale of 3 values (e.g., small for inputs/badges, medium for cards/buttons, large for dialogs); same component type always uses the same radius.
- Dividers: `1px` neutral border; never more than one divider between two adjacent regions.

## 7. Iconography

- One icon set, one stroke weight, one style (outline or filled; filled only for active/selected states).
- Sizes: `16px` inline with `14px` text, `20px` in buttons/nav, `24px` standalone; icons align to the text baseline/center.
- Icons accompany labels; icon-only buttons only for universal actions (close, more, search, edit-inline, copy, expand) with accessible name + tooltip.
- Same concept → same icon across the product; an icon never represents two concepts.
- Decorative icons are hidden from assistive technology.

## 8. Motion

- Durations: hover/press `0–100ms`; small transitions (dropdowns, tooltips) `100–150ms`; overlays `150–250ms`; never `> 300ms`.
- Easing: ease-out on enter, ease-in on exit; no bounce, no spring overshoot.
- Motion only communicates state change or spatial origin; no decorative looping animation except progress indicators.
- Reduced-motion preference disables translation/scale animations (fades `<= 150ms` allowed).
- No layout shift on load: skeletons reserve final geometry (CLS target `0`, max `0.1`).

## 9. Microcopy Rules

### 9.1 Buttons and Actions

- Verb-first, specific object: "Create project", "Send invoice", "Invite members". Never "Submit", "OK", "Yes", "Click here".
- `<= 3` words for buttons; menu items `<= 4` words.
- The label of the action that started a flow equals the label of its final commit button ("Create project" opens the dialog whose primary is "Create project").

### 9.2 Case and Tone

- Sentence case everywhere (titles, buttons, labels, tabs, column headers); proper nouns capitalized.
- Second person ("your workspace"), present tense, active voice; no exclamation marks except success of a major milestone; no jargon the role profile does not use; no internal codes as user text.

### 9.3 Labels and Terminology

- One term per concept product-wide (a glossary in `pcs/design/inventory.md` §Conventions): never "Delete" in one place and "Remove" in another for the same operation; "Remove" = detach from a container (reversible), "Delete" = destroy, "Archive" = hide but keep.
- Numbers with units; plurals correct ("1 item", "2 items").

### 9.4 Error Message Formula

`{What happened}` + `{Why, if known}` + `{What to do next}` — in plain language, without blame, without codes as the main text.

- Field: "Enter an email address like name@example.com." / "Use at least 8 characters."
- Section: "Couldn't load invoices. Check your connection and try again." + "Retry".
- Permission: "You don't have permission to delete projects. Ask a workspace admin."
- Never: "Invalid input", "Error 500", "Something went wrong" without a next step.

### 9.5 Empty State Formula

- First-use: "No {objects} yet" + "{Objects} help you {benefit}." + primary "Create {object}".
- No-results: "No {objects} match your filters" + "Try different keywords or clear filters." + "Clear filters".
- Done: "You're all caught up" + context ("Last approval 5 minutes ago").

### 9.6 Confirmation and Success

- Confirmation title is a question naming the target: "Delete invoice INV-0042?"; body states the consequence; buttons "Cancel" + exact verb.
- Success toasts in past tense naming the object: "Invoice INV-0042 sent".

## 10. Consistency Rules Across Screens

1. **Same object → same representation**: identity format, status badge vocabulary and roles, avatar size per context, date format, and money format are identical wherever the object appears (index row, card, header, panel, feed, search result).
2. **Same action → same label, icon, and position**: e.g., "Create {object}" always top-right of the page header; row overflow always last column; destructive always last in menus.
3. **Same template → same anatomy**: two index pages place search, facets, chips, table, and pagination in the same order and positions.
4. **Same state → same pattern**: all `S2` use `org-empty-state` first-use formula; all `S4` use `org-section-error`; all success feedback follows `org-feedback`.
5. **Same container for same task type**: if "Create customer" is a dialog, "Create vendor" (same complexity) is a dialog.
6. **Tokens only**: no hard-coded colors, spacing, radii, or font sizes outside the scales.
7. **Deviation requires record**: any deliberate exception is listed under Deviations in `pcs/design/inventory.md` with its reason.

## 11. Screenshot Verification Checklist

The verifier captures each page at `1440×900` and `375×812` in each declared state and checks:

| ID | Check | Fail severity |
|----|-------|---------------|
| `VC-01` | Exactly one primary-filled button per region (page header, open overlay footer, empty state). | 2 |
| `VC-02` | Page title is the largest text (except KPI values); `<= 4` font sizes on the page. | 1 |
| `VC-03` | All spacing visibly follows one rhythm: sibling cards equal padding, sections equal gaps, single left alignment axis. | 1 |
| `VC-04` | No status, error, or chart meaning conveyed by color alone (text or icon present). | 3 |
| `VC-05` | Numeric columns right-aligned with tabular figures; currencies and dates formatted per `03-molecules-and-patterns.md` §6. | 2 |
| `VC-06` | Same object's identity and status look identical on every screen captured. | 2 |
| `VC-07` | Same action has the same label, icon, and position on every screen captured. | 2 |
| `VC-08` | Buttons verb-first, sentence case; no "Submit"/"OK"; error copy follows §9.4. | 1 |
| `VC-09` | Empty first-use and no-results captures differ in copy and action. | 3 |
| `VC-10` | Loading capture shows skeletons matching final layout; no full-page spinner after the shell renders. | 2 |
| `VC-11` | Mobile capture: no horizontal page scroll; targets `>= 44px`; primary action reachable. | 3 |
| `VC-12` | Text contrast `>= 4.5:1`; focus indicator visible on keyboard focus with `>= 3:1` contrast. | 3 |
| `VC-13` | Content width within the template's max width; no stretched forms on wide screens. | 1 |
| `VC-14` | `>= 25%` whitespace on the desktop capture; no edge-to-edge packed cards. | 1 |
| `VC-15` | Icons from one set, one style, consistent sizes; icon-only buttons have tooltips. | 1 |
| `VC-16` | Density matches the template's declared mode; not mixed within one table or list. | 1 |

Severities use the 0–4 scale of `_shared/pcs-phase-common.md` §D; any `3` blocks closure.
