# PCS Phase — Common Protocol (`pcs-phase-common.md`)

Shared runtime contract across all Product Composition System (`pcs-*`) native subagents. Every `pcs-*` subagent MUST load and obey this file alongside its injected skills (`pcs-core` and the active Stack Adapter).

**Executor Boundary (MANDATORY):** Every `pcs-*` agent is a bounded **EXECUTOR**, not an orchestrator. Execute your assigned phase directly in your context window. Do NOT call delegation/subagent tools, do NOT spawn child agents, and do NOT bounce work back unless a hard blocker (such as missing Allowed Edit Surfaces or an unresolvable product contradiction) requires stopping.

---

## A. Skill Loading & Feature Locator Protocol

1. **Feature Locator (`## Feature locator`)**: Read the feature slug (`{feature-name}`), workspace spec path (`pcs/specs/{feature-name}.md`), task tracker path (`pcs/tasks/{feature-name}.md`), and Engram namespace (`pcs/{feature-name}/*`) from the `## Feature locator` section of your prompt.
2. **Primary Skill Route (`paths-injected`)**: Check your launch prompt for a `## Skills to load before work` section. If present, read every listed `SKILL.md` file (and `pcs-phase-common.md`) immediately before performing discovery, strategy, architecture, code composition, or verification.
3. **Fallback Route 1 (`fallback-registry`)**: If `## Skills to load before work` was omitted, read `AGENTS.md` or `.atl/skill-registry.md` in the workspace root, match `pcs-core` and the active Stack Adapter (`pcs-adapter-react-shadcn`), and load their exact `SKILL.md` paths (e.g., `brain/skills/...` or `.claude/skills/...`).
4. **Fallback Route 2 (`fallback-path`)**: If no registry exists in the workspace, check `~/.claude-pcs/skills/` and `~/.claude/skills/` for `pcs-core/SKILL.md`, `pcs-adapter-react-shadcn/SKILL.md`, and `_shared/pcs-phase-common.md`.
5. Record how skills were loaded in your Return Envelope `skill_resolution` field (`paths-injected`, `fallback-registry`, `fallback-path`, or `none`).

---

## B. Artifact Retrieval Protocol

`pcs` uses a deterministic **Dual Persistence** model combining repository Markdown files (`pcs/specs/<feature-name>.md` and `pcs/tasks/<feature-name>.md`) with Engram persistent memory (`pcs/<feature-name>/*`).

| Artifact Type | Workspace File Locator | Engram Topic Key Locator | Producer | Consumers |
|---------------|------------------------|--------------------------|----------|-----------|
| **Exploration Map** | *(Optional section in spec)* | `pcs/{feature-name}/explore` | `pcs-explorer` | `pcs-strategist`, `pcs-architect`, `pcs-builder` |
| **Strategy & Scope (Planes 1–2)** | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/strategy` | `pcs-strategist` | `pcs-architect`, `pcs-builder`, `pcs-verifier` |
| **Structure & Skeleton (Planes 3–4)** | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/architecture` | `pcs-architect` | `pcs-builder`, `pcs-verifier` |
| **Task Breakdown** | `pcs/tasks/{feature-name}.md` | `pcs/{feature-name}/tasks` | `pcs-architect` | `pcs-builder`, `pcs-verifier` |
| **Surface Build Progress (Plane 5)** | `pcs/tasks/{feature-name}.md` | `pcs/{feature-name}/apply` | `pcs-builder` | `pcs-verifier` |
| **UX & Heuristic Verification** | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/verify` | `pcs-verifier` | Coordinator / User |

### Retrieval Rules

1. **Read Workspace Files First**: When `pcs/specs/{feature-name}.md` or `pcs/tasks/{feature-name}.md` exists on disk, read it directly.
2. **Read Engram Observations for Full Context**: When retrieving an Engram topic key (such as `pcs/{feature-name}/explore`), call `mem_search(query: "pcs/{feature-name}/...", project: "{project}")` and **always follow up with `mem_get_observation(id)`**. Never rely on 300-character search previews as source material.
3. **Parallel Reads**: Execute all required file reads and Engram retrievals in a single parallel batch.
4. **Missing Upstream Artifact Gate**:
   - `pcs-architect` MUST NOT invent Planes 3–4 if Planes 1–2 (`pcs/specs/{feature-name}.md` or `pcs/{feature-name}/strategy`) are missing—report `status: blocked`.
   - `pcs-builder` (for `Tier 2` and `Tier 3` features) MUST NOT write `.tsx` components if Planes 1–4 in `pcs/specs/{feature-name}.md` or `pcs/tasks/{feature-name}.md` are missing—report `status: blocked`.

---

## C. Dual Artifact Persistence Protocol

Every `pcs-*` phase MUST persist its outputs before emitting its final text response:

1. **Disk Persistence (`hybrid` default)**:
   - Write or update the designated Markdown file (`pcs/specs/{feature-name}.md` and/or `pcs/tasks/{feature-name}.md`) when the path is included in `## Allowed edit surfaces`.
   - Read back the written file region to confirm structural integrity.
2. **Engram Persistence (MANDATORY)**:
   - Call `mem_save` with:
     ```yaml
     title: "pcs/{feature-name}/{phase-key}"
     topic_key: "pcs/{feature-name}/{phase-key}"
     type: "architecture"
     project: "{project-name}"
     capture_prompt: false
     content: "{full markdown artifact or structured phase report}"
     ```
   - `topic_key` performs an idempotent upsert when a phase is re-run.
   - Set `capture_prompt: false` whenever supported by the Engram tool schema; omit the field only if an older schema rejects it.
   - If Engram is temporarily unreachable, preserve the disk write, note the pending mirror in `risks`, and do not fail the phase.

---

## D. Usability Severity Scale (0–4) & Verification Gate

All `pcs` phases evaluate UX and accessibility defects using this standardized 5-point scale:

| Severity | Classification | Operational Definition | Gate Impact |
|----------|----------------|------------------------|-------------|
| **`0`** | **No Issue** | Complies with all heuristics, cognitive laws, state completeness, and token rules. | Pass |
| **`1`** | **Cosmetic** | Minor visual misalignment or micro-copy polish; does not slow down task completion. | Pass (log note) |
| **`2`** | **Minor Usability** | Noticeable friction or suboptimal scanning rhythm; users can easily recover. | Pass (recommend fix) |
| **`3`** | **Major Usability Blocker** | Missing negative state (`Zero-Data` vs `No-Results`, missing error/loading), wrong container causing context loss, broken keyboard focus, or severe cognitive overload. | **FAIL (Blocker)** |
| **`4`** | **Usability Catastrophe** | Unrecoverable flow dead-end, unguarded destructive action causing data loss, or inaccessible primary action. | **FAIL (Blocker)** |

**Hard Gate**: A feature cannot be marked `done` in `pcs-verifier` if any finding has **Severity `3` or `4`**.

---

## E. Return Envelope Contract

> **CRITICAL — Response Ordering**: Your FINAL action in the turn MUST be your plain-text Return Envelope (followed by `## Key Learnings`), NEVER a tool call. Execute all `Write`, `Edit`, and `mem_save` tool calls **before** generating your final text message. Do NOT call `mem_session_summary` inside a subagent.

Return a structured Markdown envelope with these exact fields:

- **`status`**: `done` | `partial` | `blocked`
- **`executive_summary`**: 1–3 sentences stating the product/UX decisions made, components built, or heuristic verdict.
- **`artifacts`**: Exact file paths (`pcs/specs/<feature>.md`, `pcs/tasks/<feature>.md`, `src/...`) and Engram `topic_key`s written.
- **`next_recommended`**: Next `pcs-*` agent in the pipeline (`pcs-strategist`, `pcs-architect`, `pcs-builder`, `pcs-verifier`, or `none`).
- **`risks`**: Unresolved UX assumptions, edge cases, or Severity 1–4 findings (`None` if clean).
- **`skill_resolution`**: `paths-injected` | `fallback-registry` | `fallback-path` | `none` (list loaded skills).

### Return Envelope Template

```markdown
**Status**: done
**Executive Summary**: Completed Planes 3 and 4 for `invoice-reconciliation`. Selected a contextual right-hand `Sheet` for discrepancy resolution to preserve table scroll context and defined all 5 mandatory negative states.
**Artifacts**: `pcs/specs/invoice-reconciliation.md`, `pcs/tasks/invoice-reconciliation.md`, Engram `pcs/invoice-reconciliation/architecture`, Engram `pcs/invoice-reconciliation/tasks`
**Next Recommended**: `pcs-builder`
**Risks**: Assumed batch rejection requires an explicit confirmation dialog due to downstream ledger impact.
**Skill Resolution**: `paths-injected` (`pcs-core`, `pcs-adapter-react-shadcn`)

## Key Learnings

1. Preserving table filter state in URL search params prevents context loss when opening the row inspection sheet.
2. Separating zero-data onboarding from filtered no-results states eliminates ambiguous empty tables during reconciliation.
```
