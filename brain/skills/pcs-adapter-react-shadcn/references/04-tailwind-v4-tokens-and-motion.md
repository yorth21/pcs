# Tailwind CSS v4 Tokens, Spacing Rhythm, Typography and Motion

This reference defines the styling, semantic token, numeric alignment, focus ring, and motion rules for **Tailwind CSS v4** in `pcs-adapter-react-shadcn`.

---

## 1. Semantic Token Architecture (`@theme inline` & OKLCH Variables)

Never hardcode raw hex/RGB/Tailwind palette colors (such as `bg-slate-900`, `text-gray-500`, `border-zinc-200`, or `bg-blue-600`) for structural surfaces or text. Always use the project's semantic CSS custom properties mapped via `@theme inline` so light and dark modes stay in lockstep:

| Surface / Element Role | Mandatory Tailwind v4 Semantic Classes | Forbidden Raw Classes |
|------------------------|----------------------------------------|-----------------------|
| **App Background (60%)** | `bg-background text-foreground` | `bg-white`, `bg-gray-50`, `text-black` |
| **Card / Table Container** | `bg-card text-card-foreground border border-border rounded-lg` | `bg-slate-100`, `shadow-xl` |
| **Muted / Secondary Surface (30%)** | `bg-muted/30` or `bg-muted text-muted-foreground` | `bg-gray-100 text-gray-400` |
| **Floating Overlay (`Dialog`/`Sheet`/`Popover`)** | `bg-popover text-popover-foreground border border-border` | `bg-white dark:bg-zinc-900` |
| **Primary Accent CTA (10%)** | `bg-primary text-primary-foreground hover:bg-primary/90` (`<Button>`) | Multiple competing colored buttons |
| **Destructive Action / Error** | `text-destructive`, `bg-destructive text-destructive-foreground`, `border-destructive/50` | `text-red-500`, `bg-red-700` |
| **Interactive Focus Ring** | `focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] outline-none` | `focus:outline-none` without a visible replacement ring |

### Canonical `@theme inline` & OKLCH CSS Token Wiring (`src/index.css`)

```css
@import "tailwindcss";
@import "tw-animate-css";

:root {
  --background: oklch(1 0 0);
  --foreground: oklch(0.145 0 0);
  --card: oklch(1 0 0);
  --card-foreground: oklch(0.145 0 0);
  --popover: oklch(1 0 0);
  --popover-foreground: oklch(0.145 0 0);
  --primary: oklch(0.205 0 0);
  --primary-foreground: oklch(0.985 0 0);
  --muted: oklch(0.97 0 0);
  --muted-foreground: oklch(0.556 0 0);
  --destructive: oklch(0.577 0.245 27.325);
  --border: oklch(0.922 0 0);
  --ring: oklch(0.708 0 0);
}

@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-destructive: var(--destructive);
  --color-border: var(--border);
  --color-ring: var(--ring);
}
```

---

## 2. Gestalt Spacing Rhythm & 12-Column Grid Discipline

Enforce a predictable mathematical spacing hierarchy so visual grouping (Gestalt Proximity) is unmistakable without adding extra borders:

| Relationship Level | Tailwind Spacing Utility | Pixel Equivalent | Usage |
|--------------------|--------------------------|------------------|-------|
| **Level 1 — Intra-Field (Label to Input / Error)** | `gap-1.5` or `space-y-1.5` | `6px` | Inside a single `<Field>` connecting `<FieldLabel>`, `<Input>`, and `<FieldError>`. |
| **Level 2 — Inter-Control (Button Group / Toolbar Pills)** | `gap-2` | `8px` | Between adjacent toolbar filters, badges, or footer `"Cancel"` and `"Save"` buttons (Fitts's Law safe separation). |
| **Level 3 — Inter-Field (Form Fields / Card Grid)** | `gap-4` or `space-y-4` | `16px` | Between sibling inputs inside a form or between metric cards in a strip. |
| **Level 4 — Inter-Section (Page Header to Content / Card Sections)** | `gap-6` or `pb-6` | `24px` | Between `PageHeader`, `MetricStrip`, `DataTableToolbar`, and `DataTable`. |

### Responsive Form Grid Pattern
- Default to a single vertical column (`grid-cols-1 gap-4`) inside `Dialog` (`sm:max-w-md`).
- Inside `Sheet` (`sm:max-w-lg`) or `CardContent`, use `grid grid-cols-1 gap-4 md:grid-cols-12` where short paired attributes use `md:col-span-4` or `md:col-span-6` and full-width text inputs use `md:col-span-12`.

---

## 3. Typography & `tabular-nums` Numeric Discipline

1. **Three-Level Type Scale**:
   - **Heading / Title**: `text-lg font-semibold tracking-tight text-foreground` (or `text-xl` / `text-2xl` for top-level page title).
   - **Body / Table Cell / Input**: `text-sm text-foreground` (`font-medium` for primary identifier columns and field labels).
   - **Caption / Helper / Timestamp**: `text-xs text-muted-foreground`.
2. **Mandatory `tabular-nums` Class**:
   - Every element rendering a number, currency, percentage, count, date, time, phone number, or reference code MUST include the `tabular-nums` utility class:
     ```tsx
     <span className="font-medium tabular-nums text-foreground">
       {formatCurrency(row.amount)}
     </span>
     ```
   - InKPI / Metric Strip cards, style the primary value with `text-2xl font-semibold tracking-tight tabular-nums text-foreground`.

---

## 4. Motion, Transitions & `prefers-reduced-motion`

- Rely on standard `shadcn/ui` / `tw-animate-css` data-state transitions (`data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0`) running at **`150ms–200ms`**.
- Never apply layout-shifting hover transforms (such as `hover:scale-105`) to cards or table rows in operational interfaces; use subtle background luminance shifts (`hover:bg-muted/50 transition-colors`).
- Respect reduced motion preferences using `motion-safe:` or `motion-reduce:transition-none motion-reduce:animate-none`.
