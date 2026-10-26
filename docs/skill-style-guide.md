# LLM-First Skill Style Guide

Use this normative guide when creating, auditing, or refactoring skills in the Product Composition System (`pcs`) repository. A skill is a **runtime instruction contract for an LLM**, not human-facing tutorial documentation: it defines when the model activates the skill, which rules are non-negotiable, how to branch at decision gates, what steps to execute, and what output structure to return.

## Required Structure

Every `SKILL.md` MUST use this exact section order:

1. **Frontmatter** — complete YAML metadata for deterministic skill discovery.
2. **`## Activation Contract`** — concrete triggers and situations that require loading the skill, plus explicit exclusions.
3. **`## Hard Rules`** — non-negotiable engineering constraints the LLM MUST NOT violate.
4. **`## Decision Gates`** — compact decision tables for branching choices.
5. **`## Execution Steps`** — ordered imperative operational workflow.
6. **`## Output Contract`** — required final return format and artifacts.
7. **`## References`** — relative paths to local supporting files under `references/` and `assets/`.

Do NOT add a `## Keywords` or `## Compact Rules` section. The skill registry and orchestrator index `name`, `description` trigger text, scope, and exact `SKILL.md` path; subagents load the full `SKILL.md` as the runtime contract.

## Frontmatter Rules

```yaml
---
name: {skill-name}
description: "Trigger: {essential trigger words users or agents will say}. {What this skill does}."
license: Apache-2.0
metadata:
  author: "{author-handle-or-org}"
  version: "1.0"
---
```

- `description` MUST be one physical line, YAML-safe, and double-quoted.
- Put trigger words first: `"Trigger: ... . {Actionable summary}."`
- `description` SHOULD be <= 160 characters and MUST be <= 250 characters.
- Frontmatter MUST include `name`, `description`, `license`, `metadata.author`, and `metadata.version`.
- Never use multi-line YAML scalars (`>` or `|`) for `description`.

## Body Token Budget

- Target **180–450 tokens** for the `SKILL.md` body (excluding frontmatter).
- Recommended maximum: **700 tokens**.
- Hard maximum: **1000 tokens**.
- Move deep methodology specifications, rubrics, heuristics, and edge cases into `references/*.md`.
- Move templates, checklists, schemas, and code blueprints into `assets/*`.

## Writing Rules

### DO

- Write imperative runtime instructions: "Load X", "Evaluate Y", "Reject Z", "Return W".
- Lead with activation triggers and hard constraints.
- Use compact Markdown tables for decision gates.
- Keep `SKILL.md` self-contained enough to route work while delegating deep tables and code blueprints to local files in `references/` and `assets/`.
- Enforce the Artifact Language Contract (100% rigorous English) and Zero External Company Names rule across all `brain/` skill files.

### DON'T

- Explain historical background, persuasive essays, or conversational filler.
- Duplicate full reference manuals or multi-hundred-line code blocks inside `SKILL.md`.
- Add vague advice that an LLM cannot verify or execute deterministically.
- Reference external URLs as primary runtime instructions.

## Directory Layout

```text
brain/skills/{skill-name}/
├── SKILL.md              # Required: concise runtime contract (180-450 tokens)
├── references/           # Deep domain rules, matrices, rubrics, and patterns
│   ├── 01-*.md
│   └── 02-*.md
└── assets/               # Templates, schemas, and production code blueprints
    ├── *-template.md
    └── *-blueprints.tsx
```

## Quality Gates Checklist

- [ ] YAML frontmatter contains `name`, single-line quoted `description` starting with `Trigger:`, `license`, `metadata.author`, and `metadata.version`.
- [ ] `description` length is <= 250 characters (target <= 160 characters).
- [ ] Sections appear in the exact required order (`Activation Contract`, `Hard Rules`, `Decision Gates`, `Execution Steps`, `Output Contract`, `References`).
- [ ] Body token count falls within 180–450 tokens (never > 700 tokens).
- [ ] Every path listed under `## References` exists on disk relative to the skill directory.
- [ ] Zero external product company names appear anywhere in the skill or its supporting files.
