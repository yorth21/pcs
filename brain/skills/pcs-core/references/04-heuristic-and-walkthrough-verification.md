# Verification — Contract Checks, Heuristics, Walkthrough, and Accessibility

This reference defines the verification executed by `pcs-verifier` on the rendered UI after a build (and after every fix round), and the rubric `pcs-auditor` applies to existing UI without a spec. Every check produces an observation from the rendered page, the keyboard, or the code; nothing is accepted because the spec declares it.

---

## 1. The 10 Usability Heuristics Rubric

Score every heuristic on the `0–4` severity scale:
- **`0` — Pass**: full compliance.
- **`1` — Cosmetic**: visual or copy polish; no impact on task speed or accuracy.
- **`2` — Minor friction**: noticeable hesitation; the user recovers unaided.
- **`3` — Major blocker (FAIL)**: missing state, context-destroying container, failed acceptance check, ambiguous error recovery, broken keyboard flow.
- **`4` — Catastrophe (FAIL)**: data loss risk, unguarded destructive action, dead-end flow, inaccessible primary action.

> **Hard Gate**: any `3` or `4` blocks closure and is sent to a builder fix round.

| # | Heuristic | Pass Criteria | Automatic `3–4` Triggers |
|---|-----------|---------------|--------------------------|
| **H1** | Visibility of system status | Feedback within 400 ms on every fetch, filter, and mutation: geometry-matched skeletons, in-button progress while saving, visible active filters, completion feedback. | Save or delete shows no progress and allows double submission; active filters are invisible. |
| **H2** | Match with the real world | Labels, headers, and statuses use the domain vocabulary of the ORCA matrix and inventory glossary. | Raw enum keys, internal IDs, stack traces, or database error codes in the UI. |
| **H3** | User control and freedom | Every overlay closes with Escape, a close control, and "Cancel"; modified forms warn before discarding; reversible actions offer "Undo". | A modal with no exit; an accidental backdrop click wipes typed input. |
| **H4** | Consistency and standards | Same object -> same identity and status representation; same action -> same label, icon, and position across screens; inventory components reused. | Swapped action order between overlays; a second component implementing the same catalog organism without justification. |
| **H5** | Error prevention | Destructive actions confirm naming the target and consequence; invalid input constrained by typed controls. | One-click permanent delete; submit allowed while already submitting. |
| **H6** | Recognition rather than recall | Contextual sheets keep the list visible; search keeps the typed query; filters show active state. | An opaque overlay hides data the user must copy or compare. |
| **H7** | Flexibility and efficiency | Full keyboard operation; list state in the URL; context restored after record edits. | Hover-only critical actions; filters and page reset after each edit. |
| **H8** | Aesthetic and minimalist design | One primary action per region; whitespace floor; `<= 7` default columns; tabular figures on numbers. | Competing primary buttons in one header; clipped wide tables. |
| **H9** | Recognize, diagnose, recover from errors | Inline field errors stating what and how to fix; region-scoped errors with "Retry". | Generic error notification that clears the form; a single failed query breaks the page. |
| **H10** | Help and documentation | First-use empty state explains the purpose and offers the primary action; non-obvious fields have a one-line description. | Blank area or bare header on first use. |

---

## 2. The 4-Question Cognitive Walkthrough

Walk every step of the primary job (and each recovery path) on the rendered UI:

1. **Q1 Intent**: will the user try to achieve the right effect at this step?
2. **Q2 Visibility**: is the control visible without hover-only discovery or deep scrolling?
3. **Q3 Association**: does the label match the intended action ("Create invoice", not "Submit")?
4. **Q4 Feedback**: does the interface confirm progress within 400 ms?

Record each `No` as a Severity `2` or `3` finding with the component and location.

---

## 3. Accessibility & State Checklist (WCAG 2.2 AA)

- [ ] Zero-Data and No-Results are separate branches with distinct copy and actions.
- [ ] Loading skeletons match the final geometry (no layout shift).
- [ ] Failed queries render a region-scoped error with a working retry.
- [ ] Every input has a programmatic label, an invalid flag when in error, and error text linked as its description.
- [ ] Every icon-only button has an accessible name.
- [ ] Visible focus indicator (`>= 3:1` contrast); overlays trap focus; closing returns focus to the trigger.
- [ ] Landmarks present; text contrast `>= 4.5:1`; targets `>= 24px` (`>= 44px` on touch).
- [ ] No horizontal page overflow at `1440x900` and `390x844`.
- [ ] No console errors during the walk.

---

## 4. Page Composition Spec Contract Verification (Build Flows)

### 4.1 Procedure

For every page block under `## 3. Page Composition` in `pcs/specs/<feature-name>.md`:

1. **Static checks**: run the project's typecheck, lint, and build. A failure is Severity `3`.
2. **Server**: reuse a running dev server (never stop it); a server you start runs in one shell invocation with an exit trap that kills it.
3. **Anatomy**: the rendered page shows every region of the region tree in order; omitted regions match the spec's reasons. A missing region is Severity `3`.
4. **Acceptance checks**: execute every `AC-*` line exactly as written (action + expected observation). A failed check is Severity `3` unless the check itself is cosmetic (`1–2`).
5. **State matrix**: force every non-`n/a` cell (§4.2) and compare the rendering with the cell. A missing or wrong state is Severity `3`; wrong copy only is `1–2`.
6. **Visual checklist**: evaluate `VC-01`..`VC-16` of `pcs-saas` `05-visual-consistency.md` §11 on the captures, with the severity listed there.
7. **Inventory**: each organism's decision (`reuse`, `extend`, `new`, `deviate`) matches the code; a new component where the inventory lists a reusable one, or a label or status representation that differs from the glossary, is Severity `2` (`pcs-saas` `06` §4).
8. **Keyboard & accessibility**: walk the primary job with Tab, Shift+Tab, Enter, Escape, and arrow keys; apply §3.
9. **Captures**: desktop `1440x900` and mobile `390x844` for the default state and each forced state, saved under `.pcs/playwright/<page>-<viewport>-<state>.png`.
10. **Re-runs**: after a fix round, re-check the previous blocking findings first and mark each `resolved` or `still open`, then run the full procedure again.

### 4.2 Forcing States Live

Use the first technique available, in order: browser automation code that intercepts network requests; the project's own end-to-end tooling run from a temporary directory outside the repository; the typed mock's empty and error switches; otherwise a static code check, reported as `unverified live` with the reason.

| State | Technique |
|-------|-----------|
| `S1` Loading | Delay the data request (for example 3 s) and capture before it resolves. |
| `S2` Empty first-use | Return an empty collection for the request, or use the mock's empty switch. |
| `S3` No-results | Type a query or select filters that match nothing on real data. |
| `S4` Error | Abort the request or return a 500 response; then click "Retry" and confirm recovery. |
| `S5` No permission | Return 403, or use a role without rights when credentials for it are provided. |
| `S6` Degraded | Fail one dependency request while the main one succeeds. |
| `S7` Success | Perform the mutation and capture the feedback named in the matrix (inline change, notification with "Undo", navigation). |
| Validation | Submit empty and invalid values; confirm inline messages, preserved input, focus on the first invalid field. |

### 4.3 Fix-Ready Finding Format

Every finding is actionable by a builder that has not seen the page:

| Field | Content |
|-------|---------|
| **ID** | `F-<n>`, stable across re-runs of the same feature |
| **Severity** | `0–4` per §1 |
| **Check** | The failed rule: `AC-*`, state code, `VC-*`, heuristic, law, or WCAG criterion |
| **Evidence** | Screenshot path or DOM selector, plus the observed text or value |
| **Location** | `file:line` when known, else component path |
| **Expected vs actual** | The spec contract versus what rendered |
| **Fix** | One concrete instruction (what to change, where, to what) |

Blocking findings (Severity `3–4`) are listed first in the Verification Report. The Return Envelope is `success` only with zero blocking findings; otherwise `partial`.

---

## 5. Cognitive Laws Checked on the Rendered UI

Laws are verified by measurement or observation, never declared:

| Law | Observation on the Rendered UI | Typical Severity |
|-----|--------------------------------|------------------|
| **Fitts** | Interactive targets `>= 36px` on desktop and `>= 44px` at `390px`; primary action at the expected anchor of its container; destructive actions separated from confirm. | 2 (3 if the primary action is unreachable on mobile) |
| **Hick** | `<= 5` top-level actions per toolbar or header; row actions in one overflow menu with `<= 1` inline quick action. | 2 |
| **Miller** | `<= 7` default visible columns; form sections of 3–7 fields. | 2 |
| **Jakob** | Search top-left of the filter bar, create action top-right of the header, "Cancel" before the confirming action, Escape closes overlays. | 2 |
| **Tesler** | Defaults pre-filled where the system knows the value (date, organization, page size). | 1–2 |
| **Doherty** | Visible feedback within 400 ms after each interaction (measured with delayed requests in §4.2). | 2–3 |
| **Gestalt** | Label-to-field gap < field-to-field gap < section gap; related controls share one container. | 1 |
