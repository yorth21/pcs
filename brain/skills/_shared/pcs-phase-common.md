# PCS Phase — Common Protocol (`pcs-phase-common.md`)

Shared runtime contract. Every `pcs-*` subagent is a bounded **EXECUTOR** (never call delegation tools) and loads this file with `pcs-core` and the active Stack Adapter.

## A. Skill Loading & Feature Locator Protocol

1. **Feature Locator (`## Feature locator`)**: Extract `{feature-name}`, `pcs/specs/{feature-name}.md`, `pcs/tasks/{feature-name}.md`, and Engram `pcs/{feature-name}/*`.
2. **Primary (`paths-injected`)**: Read all paths under `## Skills to load before work` (`pcs-core/SKILL.md`, `pcs-adapter-react-shadcn/SKILL.md`, `_shared/pcs-phase-common.md`).
3. **Registry Fallback (`fallback-registry`)**: If omitted, read workspace `AGENTS.md` or `.atl/pcs-skill-registry.md` (`brain/skills/...` or `.claude/skills/...`).
4. **Profile Fallback (`fallback-path`)**: Otherwise check `$CLAUDE_CONFIG_DIR/skills` (or active profile `<skills-dir>`), then `~/.claude/skills`.
5. Record the mode in `**Skill Resolution**` (`paths-injected` | `fallback-registry` | `fallback-path` | `none`).

## B. Dual Persistence & Retrieval Matrix

| Artifact | Workspace File | Engram `topic_key` (`type: "architecture"`) | Producer -> Consumers |
|----------|----------------|---------------------------------------------|-----------------------|
| Discovery Map | — | `pcs/{feature-name}/explore` | `pcs-explorer` -> all later phases |
| Strategy & Scope (Planes 1–2) | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/strategy` | `pcs-strategist` -> later phases |
| Structure & Skeleton (Planes 3–4) | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/architecture` | `pcs-architect` -> `builder`, `verifier` |
| Task Breakdown | `pcs/tasks/{feature-name}.md` | `pcs/{feature-name}/tasks` | `pcs-architect` -> `builder`, `verifier` |
| Surface Progress (Plane 5) | `pcs/tasks/{feature-name}.md` | `pcs/{feature-name}/apply` | `pcs-builder` -> `verifier` |
| Post-Build Verification | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/verify` | `pcs-verifier` -> Coordinator |
| Baseline Audit (existing UI) | `pcs/audits/{feature-name}.md` | `pcs/{feature-name}/audit` | `pcs-auditor` -> Coordinator (persists both) |

1. **Read First**: Read on-disk `pcs/specs/{feature-name}.md` and `pcs/tasks/{feature-name}.md` in parallel with Engram (`mem_search`, then `mem_get_observation(id)`; never rely on search previews).
2. **Upstream Gate**: `pcs-architect` returns `blocked` if Planes 1–2 are missing; `pcs-builder` (`Tier 2`/`Tier 3`) returns `blocked` if Planes 1–4 or `pcs/tasks/{feature-name}.md` are missing; `pcs-verifier` returns `blocked` without a spec (baseline audits belong to `pcs-auditor`).
3. **Write & Mirror**: Write files only within `## Allowed edit surfaces`, verify them, and call `mem_save` (`capture_prompt: false` when supported) before your final text response. Read-only agents write nothing.

## C. Shared CodeGraph Protocol

- Use `mcp__codegraph__codegraph_explore` before broad `Read`/`Glob`/`Grep` searches.
- Agents with `Bash` (`pcs-builder`, `pcs-verifier`, the Coordinator) may initialize a missing `<project-root>/.codegraph/` once via `pcs codegraph init --cwd <project-root>` (never in `$HOME` or `/tmp`; `pcs codegraph` is not a query proxy).
- Agents without `Bash` (`pcs-explorer`, `pcs-strategist`, `pcs-architect`, `pcs-auditor`) query `codegraph_explore` directly and fall back to bounded `Grep`/`Glob`/`Read` when unindexed.

## D. Usability Severity Scale (0–4) & Hard Gate

- **`0` (Pass)**: Full compliance.
- **`1` (Cosmetic)**: Visual/copy polish; no task speed impact.
- **`2` (Minor Friction)**: Hesitation; user recovers easily.
- **`3` (Major Blocker — FAIL)**: Missing negative state (`Zero-Data` vs `No-Results`, loading, partial/inline error), wrong container losing context, or broken keyboard flow.
- **`4` (Catastrophe — FAIL)**: Data loss risk, unguarded destructive action, dead-end flow, or inaccessible primary CTA.
- **Hard Gate**: Zero Severity `3` or `4` findings are permitted when closing a build feature.

## E. Return Envelope Contract (MANDATORY)

Your FINAL action is this plain-text Return Envelope, then `## Key Learnings` (1–5 numbered standalone sentences, >=20 chars and >=4 words each); never end on a tool call. The first line MUST be exactly `**Status**: success`, `**Status**: partial`, or `**Status**: blocked`. The SubagentStop hook parses that line to record your flow phase: a missing or unparseable line records `unverified`, and the flow stays open until you are re-delegated. Write no later line starting with `Status:`.

```markdown
**Status**: success | partial | blocked
**Executive Summary**: <1–3 sentences on UX decisions, built components, or heuristic verdict>
**Artifacts**: <paths and Engram topic_keys written>
**Next Recommended**: pcs-strategist | pcs-architect | pcs-builder | pcs-verifier | pcs-auditor | none
**Risks**: <assumptions, edge cases, or Sev 1–4 findings>
**Skill Resolution**: paths-injected | fallback-registry | fallback-path | none

## Key Learnings
1. <Standalone factual takeaway sentence from this phase.>
```
