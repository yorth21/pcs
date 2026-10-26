# PCS Phase — Common Protocol (`pcs-phase-common.md`)

Shared runtime contract across all Product Composition System (`pcs-*`) native subagents. Every `pcs-*` subagent is a bounded **EXECUTOR** (never call delegation/subagent tools) and MUST load this file alongside `pcs-core` and the active Stack Adapter.

## A. Skill Loading & Feature Locator Protocol

1. **Feature Locator (`## Feature locator`)**: Extract `{feature-name}`, `pcs/specs/{feature-name}.md`, `pcs/tasks/{feature-name}.md`, and Engram `pcs/{feature-name}/*`.
2. **Primary (`paths-injected`)**: Read all paths listed under `## Skills to load before work` (`pcs-core/SKILL.md`, `pcs-adapter-react-shadcn/SKILL.md`, and `_shared/pcs-phase-common.md`).
3. **Registry Fallback (`fallback-registry`)**: If omitted, read workspace `AGENTS.md` or `.atl/pcs-skill-registry.md` (`brain/skills/...` or `.claude/skills/...`).
4. **Profile Fallback (`fallback-path`)**: Otherwise check `$CLAUDE_CONFIG_DIR/skills` (or active profile `<skills-dir>`), then `~/.claude/skills`.
5. Record the resolution mode in `skill_resolution` (`paths-injected` | `fallback-registry` | `fallback-path` | `none`).

## B. Dual Persistence & Retrieval Matrix

| Artifact | Workspace File | Engram `topic_key` (`type: "architecture"`) | Producer -> Consumers |
|----------|----------------|---------------------------------------------|-----------------------|
| Discovery Map | *(Optional spec section)* | `pcs/{feature-name}/explore` | `pcs-explorer` -> `strategist`, `architect`, `builder` |
| Strategy & Scope (Planes 1–2) | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/strategy` | `pcs-strategist` -> `architect`, `builder`, `verifier` |
| Structure & Skeleton (Planes 3–4) | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/architecture` | `pcs-architect` -> `builder`, `verifier` |
| Task Breakdown | `pcs/tasks/{feature-name}.md` | `pcs/{feature-name}/tasks` | `pcs-architect` -> `builder`, `verifier` |
| Surface Progress (Plane 5) | `pcs/tasks/{feature-name}.md` | `pcs/{feature-name}/apply` | `pcs-builder` -> `verifier` |
| Heuristic & A11y Audit | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/verify` | `pcs-verifier` -> Coordinator |

1. **Read First**: Read on-disk `pcs/specs/{feature-name}.md` and `pcs/tasks/{feature-name}.md` in parallel with Engram (`mem_search` followed by `mem_get_observation(id)`—never rely on search previews).
2. **Upstream Gate**: `pcs-architect` reports `status: blocked` if Planes 1–2 are missing; `pcs-builder` (`Tier 2`/`Tier 3`) reports `status: blocked` if Planes 1–4 or `pcs/tasks/{feature-name}.md` are missing.
3. **Write & Mirror**: Write authorized disk files (within `## Allowed edit surfaces`), verify integrity, and call `mem_save` (`capture_prompt: false` when supported by schema) before emitting your final text response.

## C. Shared CodeGraph Protocol

- Use `mcp__codegraph__codegraph_explore` before broad `Read`/`Glob`/`Grep` searches to trace routes, components, hooks, callers, and blast radius in one call.
- Agents with `Bash` (`pcs-builder`, `pcs-verifier`, or the Orchestrator) may initialize a missing `<project-root>/.codegraph/` index once via `pcs codegraph init --cwd <project-root>` (never in `$HOME` or `/tmp`, and never using `pcs codegraph` as a general query proxy; read-only CLI queries belong to upstream `codegraph`).
- Read-only agents without `Bash` (`pcs-explorer`, `pcs-strategist`, `pcs-architect`) query `codegraph_explore` MCP directly and fall back to bounded `Grep`/`Glob`/`Read` if `.codegraph/` is not yet initialized.

## D. Usability Severity Scale (0–4) & Hard Gate

- **`0` (Pass)**: Full compliance with heuristics, cognitive laws, state completeness, and token rules.
- **`1` (Cosmetic)**: Minor visual/copy polish; zero task speed impact.
- **`2` (Minor Friction)**: Noticeable hesitation; user recovers easily.
- **`3` (Major Blocker — FAIL)**: Missing negative state (`Zero-Data` vs `No-Results`, loading, partial/inline error), wrong container losing context, or broken keyboard flow.
- **`4` (Catastrophe — FAIL)**: Data loss risk, unguarded destructive action, dead-end flow, or inaccessible primary CTA.
- **Hard Gate**: Zero Severity `3` or `4` findings are permitted when closing a feature.

## E. Return Envelope Contract

Your FINAL action in the turn MUST be your plain-text Return Envelope followed by `## Key Learnings` (1–5 numbered standalone factual sentences, >=20 chars and >=4 words each), NEVER a tool call:

```markdown
**Status**: done | partial | blocked
**Executive Summary**: <1–3 sentences on UX decisions, built components, or heuristic verdict>
**Artifacts**: <paths and Engram topic_keys written>
**Next Recommended**: pcs-strategist | pcs-architect | pcs-builder | pcs-verifier | none
**Risks**: <assumptions, edge cases, or Sev 1–4 findings>
**Skill Resolution**: paths-injected | fallback-registry | fallback-path | none

## Key Learnings
1. <Standalone factual takeaway sentence from this phase.>
```
