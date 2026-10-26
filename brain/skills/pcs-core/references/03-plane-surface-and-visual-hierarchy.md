# Plane 5 — Surface, Visual Hierarchy and Data Integrity Specification

This reference defines the technology-agnostic engineering rules for **Plane 5 (Surface)** in Product-Centered Development (PCD). It governs visual hierarchy, perceptual layering, semantic color budgets, typography, scanning patterns, quantitative data display, and micro-interaction timing.

---

## 1. Perceptual Layering & Figure-Ground Elevation

Interfaces must communicate depth and containment through subtle surface luminance shifts and hairline borders rather than heavy drop shadows:

| Elevation Layer | Semantic Role | Visual Treatment |
|-----------------|---------------|------------------|
| **Layer 0 — App Canvas** | Background shell behind main content (`background` / `muted/20`) | Lowest luminance contrast; recedes visually so content containers stand out. |
| **Layer 1 — Primary Work Surface** | Cards, DataTables, split workbench panels (`card`) | Distinct surface fill + `1px` hairline border (`border`). Zero or minimal shadow (`shadow-xs`). |
| **Layer 2 — Interactive Controls & Sticky Headers** | Inputs, buttons, active tabs, sticky table headers | Clear border or subtle background tint (`muted/40`) with high-contrast text (`foreground`). |
| **Layer 3 — Floating Overlays** | `Popover`, `DropdownMenu`, `Command`, `Sheet`, `Dialog`, `AlertDialog` | Elevated surface (`popover` / `background`) + crisp border + perceptual backdrop dimming (`bg-black/50` for modal overlays) to establish unmistakable Figure-Ground separation. |

---

## 2. The 60-30-10 Semantic Color Budget

Color in operational software is an **information signal**, never wallpaper. Enforce the **60-30-10 proportion** across every viewport:

1. **60% — Neutral Canvas & Whitespace (`background`, `card`)**:
   - At least **25% of every viewport MUST remain open whitespace** (padding, margins, breathing room between chunks). Never pack cards edge-to-edge without gaps.
2. **30% — Structural Hierarchy & Secondary Surfaces (`muted`, `border`, `secondary`, `muted-foreground`)**:
   - Used for table headers, subtle row hover states, secondary buttons, inactive tabs, and supporting labels.
3. **10% — High-Salience Semantic & Primary Accent (`primary`, `destructive`, `warning`, `success`)**:
   - **Single Primary CTA Rule**: Render **at most ONE solid primary-filled button (`variant="default"`) per viewport level** (plus 1 inside an active modal/sheet footer when open). All sibling actions MUST step down to `outline`, `secondary`, or `ghost`.
   - **Semantic Status Restraint**: Reserve saturated red (`destructive`), amber (`warning`), and green (`success`) strictly for status badges, validation errors, and critical alerts. Never use saturated semantic colors as full card backgrounds.

---

## 3. Three-Level Typographic & Contrast Hierarchy

Limit every component or card region to **at most 3 distinct typographic levels** to prevent visual noise:

| Level | Semantic Role | Size & Weight Range | Contrast Target (WCAG 2.2 AA) |
|-------|---------------|---------------------|-------------------------------|
| **Level 1 — Primary Anchor** | Page Title, Card Title, Primary Metric Value, Dialog Title | `18px–24px` (`text-lg` to `text-2xl`), `font-semibold` (`600`), tight tracking (`tracking-tight`) | `foreground` (`>= 7:1` AAA preferred, `>= 4.5:1` mandatory) |
| **Level 2 — Body & Control Text** | Table cell values, form input values, field labels, button labels | `14px` (`text-sm`), `font-normal` (`400`) or `font-medium` (`500` for labels/primary cells) | `foreground` (`>= 4.5:1` mandatory) |
| **Level 3 — Supporting Metadata** | Section descriptions, helper hints, timestamps, secondary IDs | `12px–13px` (`text-xs`), `font-normal` (`400`) | `muted-foreground` (`>= 4.5:1` against background surface) |

---

## 4. Human Scanning Patterns (F-Pattern, Layer-Cake & Z-Pattern)

Align component geometry with natural eye-tracking paths:

1. **Layer-Cake Scanning (Forms & Settings)**:
   - Use distinct horizontal section headers (`CardHeader` or `FieldSet` legend) with high contrast so operators can scan down the left margin and jump directly to the relevant section.
   - Maintain a **single vertical alignment axis** for form labels and inputs. Prefer 1-column layouts for forms; allow 2-column splits (`grid-cols-12`) only for tightly coupled short pairs (e.g., `First Name + Last Name`, `Code + Category`, `City + Postal Code`).
2. **F-Pattern Scanning (Data Tables & Lists)**:
   - Place the **Primary Object Identifier** (Name, Reference Code, Subject) in the **first (leftmost) data column** with `font-medium`.
   - Keep left-aligned text columns for prose/identifiers, center-aligned cells only for fixed-width badges/icons, and **right-aligned cells with `tabular-nums`** for all quantitative amounts, percentages, counts, and currencies.
3. **Z-Pattern Scanning (Page Headers & Empty States)**:
   - Top-left: Page Title + concise 1-line operational description.
   - Top-right: Single Primary Action button.
   - Center/Bottom: Primary workspace content.

---

## 5. Quantitative Data Display & Chart Integrity

Whenever displaying numbers, metrics, tables, or charts:

1. **Mandatory `tabular-nums` (`font-variant-numeric: tabular-nums`)**:
   - Apply `tabular-nums` to every currency amount, count, percentage, timestamp, date, duration, phone number, and reference code so digits align vertically across rows and never jitter during live updates.
2. **Explicit Units & Formatting**:
   - Format currencies, decimals, and dates consistently using locale-aware formatters (`Intl.NumberFormat` / `Intl.DateTimeFormat`). Never display raw unformatted floats (e.g., `1240.5`) or ambiguous dates.
3. **Zero Chart Junk (Data-Ink Ratio)**:
   - Every KPI card or chart must answer a concrete operational question. Omit decorative sparklines, 3D effects, dual-axis distortions, or unlabelled axes.
   - Pair any summary metric delta (e.g., `+12.4%`) with its explicit comparison baseline (e.g., `"vs. last 30 days"`).

---

## 6. Micro-Interaction Timing & Motion Discipline

- **Hover / Focus / Active Feedback**: Instantaneous (`0–100ms`).
- **Overlay Enter / Exit Transitions (`Dialog`, `Sheet`, `Popover`, `DropdownMenu`)**: Fast and purposeful (`150ms–200ms` using `ease-out` on entry and `ease-in` on exit). Never use slow (`> 300ms`) or bouncy spring animations in productivity software.
- **Reduced Motion Accessibility**: Always respect `@media (prefers-reduced-motion: reduce)` by disabling spatial slide/scale translations when requested by the operating system.
