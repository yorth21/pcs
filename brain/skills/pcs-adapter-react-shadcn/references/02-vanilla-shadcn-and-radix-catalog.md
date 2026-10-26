# Vanilla `shadcn/ui` Catalog and `radix-ui` Escape Hatch Rules

This reference maps Plane 3 (Structure) and Plane 4 (Skeleton) UX decisions directly to **vanilla `shadcn/ui` primitives** (`src/components/ui/*`) and defines the strict KISS boundary for `radix-ui` usage.

---

## 1. KISS & Vanilla `shadcn/ui` First Rule

**Hard Rule:** Compose `shadcn/ui` primitives (`Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter`, `Dialog`, `Sheet`, `AlertDialog`, `Table`, `Empty`, `Alert`, `Field`, `Button`, `Badge`, `DropdownMenu`, `Tabs`, `Skeleton`) **directly** in your feature components.

### Strictly Forbidden Wrapper Anti-Patterns
- **NO `<CustomModal title="..." footer="...">`**: Compose `<Dialog>`, `<DialogContent>`, `<DialogHeader>`, `<DialogTitle>`, `<DialogDescription>`, and `<DialogFooter>` directly so sub-elements retain full slot flexibility and accessibility attributes.
- **NO `<SettingsCard title="...">`**: Compose `<Card>`, `<CardHeader>`, `<CardTitle>`, `<CardDescription>`, `<CardContent>`, and `<CardFooter>` directly.
- **NO `<PrimaryButton>` / `<DangerButton>`**: Use `<Button variant="default">` and `<Button variant="destructive">` directly.

---

## 2. Plane 4 Skeleton-to-`shadcn/ui` Component Mapping

| UX / Skeleton Need (Plane 3 & 4) | Canonical `shadcn/ui` Primitive (`@/components/ui/*`) | Composition & Accessibility Contract |
|----------------------------------|-------------------------------------------------------|--------------------------------------|
| **Atomic Task Overlay** (`Zero` background context dependency, 1–4 fields) | `Dialog`, `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogDescription`, `DialogFooter` | Always include both `DialogTitle` and `DialogDescription` (never omit `DialogDescription` or Radix will warn on missing `aria-describedby`). Constrain width with `sm:max-w-md`. |
| **Contextual Record Inspection / Edit** (`High` background context dependency, 4–12 fields) | `Sheet`, `SheetContent` (`side="right"`), `SheetHeader`, `SheetTitle`, `SheetDescription`, `SheetFooter` | Use a flex column layout (`flex flex-col p-0 sm:max-w-lg`) with a scrollable body (`flex-1 overflow-y-auto px-6 py-4`) and sticky header/footer (`border-b` / `border-t bg-muted/30 px-6 py-4`). |
| **Mobile Contextual Overlay** | `Drawer`, `DrawerContent`, `DrawerHeader`, `DrawerTitle`, `DrawerDescription`, `DrawerFooter` (`vaul`) | Optional responsive pairing with `Sheet` on mobile viewports (`< 768px`) for thumb-friendly bottom sheet interaction. |
| **High-Risk Destructive Confirmation** (Risk Tier C) | `AlertDialog`, `AlertDialogContent`, `AlertDialogHeader`, `AlertDialogTitle`, `AlertDialogDescription`, `AlertDialogFooter`, `AlertDialogCancel`, `AlertDialogAction` | Mandatory for irreversible deletes/mutations. Style `AlertDialogAction` with destructive semantics (`bg-destructive text-destructive-foreground hover:bg-destructive/90`) and verb-specific label (`"Delete invoice #1042"`). |
| **Low-Risk Reversible Feedback** (Risk Tier A) | `Toaster` + `toast()` from `sonner` | Call `toast.success("Record archived", { action: { label: "Undo", onClick: () => restoreRecord(id) } })`. |
| **Split Master-Detail Workbench** | `ResizablePanelGroup`, `ResizablePanel`, `ResizableHandle` | Use `direction="horizontal"` on desktop with sensible `defaultSize` (`35` / `65` or `40` / `60`) and `minSize={25}`. |
| **Zero-Data First-Use State** (Negative State #1) | `Empty`, `EmptyHeader`, `EmptyMedia`, `EmptyTitle`, `EmptyDescription`, `EmptyContent` (or dashed semantic empty container) | Pair a muted icon (`size-10 rounded-full bg-muted`) with a clear domain title, 1-sentence value explanation, and 1 primary creation `<Button>`. |
| **Contextual Loading State** (Negative State #3) | `Skeleton` | Mirror exact loaded heights and widths (`h-8 w-56`, `h-9 w-72`, table row strips) inside the real page shell so `CLS = 0`. |
| **Partial / Section Error State** (Negative State #4) | `Alert`, `AlertTitle`, `AlertDescription` (`variant="destructive"`) + retry `Button` | Render inline inside the failed section/card without unmounting unaffected sibling sections. |
| **Accessible Form Fields** (Negative State #5) | `Field`, `FieldLabel`, `FieldDescription`, `FieldError` (or `src/components/form/*` adapters) + `Input`, `Textarea`, `Select`, `Checkbox`, `Switch` | Set `aria-invalid={!!fieldState.error}` on the input control and render `<FieldError errors={[fieldState.error]} />` directly underneath. |
| **Row-Level Secondary Actions** (Hick's Law) | `DropdownMenu`, `DropdownMenuTrigger`, `DropdownMenuContent`, `DropdownMenuItem`, `DropdownMenuSeparator` | Trigger via `<Button variant="ghost" size="icon" className="size-8" aria-label="Open row actions">`. Place destructive action below `<DropdownMenuSeparator />` with `className="text-destructive focus:text-destructive"`. |
| **Searchable Picker / Combobox** (`> 8` options) | `Popover` + `Command` (`CommandInput`, `CommandList`, `CommandEmpty`, `CommandGroup`, `CommandItem`) | Use standard `<Select>` for `3–8` fixed options; upgrade to `<Popover>` + `<Command>` (`cmdk`) whenever options exceed `8` or require keyboard filtering. |
| **Navigation & Hierarchy** (`<= 2` levels) | `Sidebar`, `SidebarProvider`, `SidebarInset`, `Breadcrumb`, `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent` | Never nest `Tabs` inside `Tabs`. Keep `TabsList` to `2–5` triggers maximum. |

---

## 3. The `radix-ui` Escape Hatch Rule

1. **Always check `src/components/ui/` first.** If a `shadcn/ui` primitive exists for the pattern (or can be composed from existing `src/components/ui/*` exports), use it.
2. **When `radix-ui` direct import IS permitted**:
   - You need a headless accessibility primitive that is not wrapped in `src/components/ui/` (for example, `Toolbar` for keyboard-roving button groups, or `VisuallyHidden` when `sr-only` is insufficient).
   - When importing from `radix-ui` (or `@radix-ui/react-*`), style it with the exact same Tailwind v4 semantic tokens (`bg-popover`, `text-popover-foreground`, `border-border`, `focus-visible:ring-ring/50`) and promote it to `src/components/ui/` if used across `>= 2` features.
