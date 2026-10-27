# PCS Phase — Common Protocol (`pcs-phase-common.md`)

Shared runtime contract. Every `pcs-*` subagent is a bounded **EXECUTOR** (never call delegation tools) and loads this file with the skills named in its payload.

## A. Skill Loading & Feature Locator Protocol

1. **Feature Locator (`## Feature locator`)**: Extract `{feature-name}`, `pcs/specs/{feature-name}.md`, `pcs/tasks/{feature-name}.md`, and Engram `pcs/{feature-name}/*`.
2. **Primary (`paths-injected`)**: Read all paths under `## Skills to load before work` (`pcs-core`, `pcs-saas` for strategy/compose/apply/verify, the Stack Adapter for compose/apply/verify, and this file).
3. **Registry Fallback (`fallback-registry`)**: If omitted, read workspace `AGENTS.md` or `.atl/pcs-skill-registry.md` (`brain/skills/...` or `.claude/skills/...`).
4. **Profile Fallback (`fallback-path`)**: Otherwise check `$CLAUDE_CONFIG_DIR/skills` (or active profile `<skills-dir>`), then `~/.claude/skills`.
5. Record the mode in `**Skill Resolution**` (`paths-injected` | `fallback-registry` | `fallback-path` | `none`).

## B. Dual Persistence & Retrieval Matrix

| Artifact | Workspace File | Engram `topic_key` | Producer -> Consumers |
|----------|----------------|--------------------|-----------------------|
| Discovery Map | — | `pcs/{feature-name}/explore` | `pcs-explorer` -> all |
| UI Inventory | `pcs/design/inventory.md` | — | `pcs-explorer` (builder appends) -> compose, apply, verify |
| Strategy & Scope (Planes 1–2) | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/strategy` | `pcs-strategist` -> compose |
| Page Composition Spec (Planes 3–4) | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/compose` | `pcs-architect` -> builder, verifier |
| Task Breakdown | `pcs/tasks/{feature-name}.md` | `pcs/{feature-name}/tasks` | `pcs-architect` -> builder |
| Build & Fix Progress | `pcs/tasks/{feature-name}.md` | `pcs/{feature-name}/apply` | `pcs-builder` -> verifier |
| Verification Report | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/verify` | `pcs-verifier` -> builder (fix), Coordinator |
| Baseline Audit | `pcs/audits/{feature-name}.md` | `pcs/{feature-name}/audit` | `pcs-auditor` -> Coordinator (persists both) |

1. **Read First**: Read on-disk files in parallel with Engram (`mem_search`, then `mem_get_observation(id)`; never rely on previews).
2. **Upstream Gate**: `pcs-architect` returns `blocked` without Planes 1–2; `pcs-builder` without the Page Composition Spec or `pcs/tasks/{feature-name}.md`; `pcs-verifier` without a spec (baseline audits belong to `pcs-auditor`).
3. **Write & Mirror**: Write only within `## Allowed edit surfaces`, verify the write, and call `mem_save` (`type: "architecture"`, `capture_prompt: false`) before your final text response. Never edit `.pcs/` (CLI-owned flow state). Read-only agents write nothing.

## C. Shared CodeGraph Protocol

- Use `mcp__codegraph__codegraph_explore` before broad `Read`/`Glob`/`Grep` searches.
- Agents with `Bash` (`pcs-builder`, `pcs-verifier`, the Coordinator) may initialize a missing `<project-root>/.codegraph/` once via `pcs codegraph init --cwd <project-root>` (never in `$HOME` or `/tmp`).
- Agents without `Bash` query `codegraph_explore` directly and fall back to bounded `Grep`/`Glob`/`Read`.

## D. Usability Severity Scale (0–4) & Hard Gate

- **`0` Pass**; **`1` Cosmetic** (polish); **`2` Minor Friction** (hesitation, easy recovery).
- **`3` Major Blocker — FAIL**: missing state of the state matrix, wrong container losing context, failed `AC-*` check, color-only meaning, broken keyboard flow, mobile overflow.
- **`4` Catastrophe — FAIL**: data loss risk, unguarded destructive action, dead-end flow, inaccessible primary action.
- **Hard Gate**: Severity `3–4` findings are blocking; a build flow closes only with zero.

## E. Return Envelope Contract (MANDATORY)

Your FINAL action is this plain-text Return Envelope, then `## Key Learnings` (1–5 numbered standalone sentences); never end on a tool call. The first line MUST be exactly `**Status**: success`, `**Status**: partial`, or `**Status**: blocked`. The SubagentStop hook parses it to record your phase: a missing line records `unverified`. Write no later line starting with `Status:`.

```markdown
**Status**: success | partial | blocked
**Executive Summary**: <1–3 sentences>
**Artifacts**: <paths and Engram topic_keys written>
**Next Recommended**: pcs-strategist | pcs-architect | pcs-builder | pcs-verifier | pcs-auditor | none
**Risks**: <assumptions, unverified states, or Sev 1–4 findings>
**Skill Resolution**: paths-injected | fallback-registry | fallback-path | none

## Key Learnings
1. <Standalone factual takeaway sentence from this phase.>
```
