# AGENTS.md — Product Composition System (`pcs`) Registry

This file is the authoritative registry of orchestrator blocks, native subagents, LLM-first skills, and Go CLI packages inside the Product Composition System (`pcs`) repository.

---

## 1. Repository Governance & Hard Rules

1. **Normative Skill Style Guide**: Before creating or updating any skill in `brain/skills/`, read and obey [`docs/skill-style-guide.md`](docs/skill-style-guide.md).
2. **Artifact Language Contract**: Every file, prompt, specification, template, comment, UI string, Go identifier, test, and code blueprint in this repository MUST be written in clear, rigorous, professional English. Never inject regional slang or non-English identifiers into any artifact.
3. **Zero External Company Names in `brain/` and CLI**: Never reference external product companies as shorthand for design quality. Encode every UX principle, cognitive law, heuristic, and visual rule directly as an explicit, testable engineering constraint. (Technical library and tool dependencies such as React, TypeScript, Tailwind CSS, `shadcn/ui`, Radix UI, TanStack, React Hook Form, Zod, Playwright, Engram, Context7, and CodeGraph are permitted where technically relevant.)
4. **Go Testing & Zero-Dependency Discipline**: The Go CLI (`cmd/pcs`, `internal/**`) uses only the Go standard library (`go.mod` has zero external dependencies). All filesystem tests MUST use `t.TempDir()` and pass `go test -v -race ./...` and `go vet ./...`.

---

## 2. Project Skills Index (`brain/skills/`)

| Skill Name | Scope | Trigger / Description | Exact Path |
|------------|-------|-----------------------|------------|
| `pcs-core` | Agnostic UX Methodology (Planes 1–5) | `Trigger: UX design, new feature, screen flow, wireframe, usability audit, empty states, PCD planes. Apply technology-agnostic 5-Plane UX rules.` | `brain/skills/pcs-core/SKILL.md` |
| `pcs-saas` | Agnostic SaaS Pattern Catalog (templates, organisms, composition) | `Trigger: SaaS screen, page template, compose page, organism, dashboard, table page, settings, wizard, UI inventory. Select template and compose state-complete SaaS pages.` | `brain/skills/pcs-saas/SKILL.md` |
| `pcs-adapter-react-shadcn` | Frontend Stack Adapter (React 19 + shadcn/ui) | `Trigger: React component, shadcn/ui, Tailwind v4, TanStack Table/Router/Query, React Hook Form, Zod, UI build. Compose vanilla shadcn/ui surfaces.` | `brain/skills/pcs-adapter-react-shadcn/SKILL.md` |

### Shared Skill Protocol

| Shared File | Consumer | Purpose | Exact Path |
|-------------|----------|---------|------------|
| `pcs-phase-common` | All `pcs-*` subagents | Skill loading hierarchy (`.atl/pcs-skill-registry.md`), dual artifact retrieval/persistence (`pcs/specs/`, `pcs/tasks/`, Engram `pcs/<feature>/*`), CodeGraph protocol, 0–4 severity scale, and Result Envelope contract | `brain/skills/_shared/pcs-phase-common.md` |

---

## 3. Native Subagents Index (`brain/agents/`)

| Agent Name | PCD Plane / Role | Model | Write Access | Engram Topic Key | Exact Path |
|------------|------------------|-------|--------------|------------------|------------|
| `pcs-explorer` | Plane 0 — UX & Codebase Discovery and UI Inventory | `sonnet` | `pcs/design/inventory.md` (build flows only) | `pcs/{feature-name}/explore` | `brain/agents/pcs-explorer.md` |
| `pcs-strategist` | Planes 1 & 2 — Strategy & Scope | `opus` | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/strategy` | `brain/agents/pcs-strategist.md` |
| `pcs-architect` | Planes 3 & 4 — Page Composition Spec & Tasks (phase `compose`) | `opus` | `pcs/specs/{feature-name}.md`, `pcs/tasks/{feature-name}.md` | `pcs/{feature-name}/compose`, `pcs/{feature-name}/tasks` | `brain/agents/pcs-architect.md` |
| `pcs-builder` | Plane 5 — Surface Build & Verify Fix Rounds | `sonnet` | Authorized `src/**`, `pcs/tasks/{feature-name}.md`, `pcs/design/inventory.md` | `pcs/{feature-name}/apply` | `brain/agents/pcs-builder.md` |
| `pcs-verifier` | Transversal — Post-Build Page Composition Contract, States, A11y & Playwright | `sonnet` | `pcs/specs/{feature-name}.md` (Verification Report) | `pcs/{feature-name}/verify` | `brain/agents/pcs-verifier.md` |
| `pcs-auditor` | Transversal — Baseline Audit of Existing UI (no spec) | `sonnet` | Read-Only (Coordinator persists `pcs/audits/{feature-name}.md`) | `pcs/{feature-name}/audit` | `brain/agents/pcs-auditor.md` |

---

## 4. Orchestrator & Infrastructure Injection Blocks (`brain/orchestrator/` & `internal/assets/`)

| Block Marker | Purpose | Source |
|--------------|---------|--------|
| `<!-- pcs:codegraph-guidance -->` ... `<!-- /pcs:codegraph-guidance -->` | Structural AST exploration rules and safe `pcs codegraph init` workflow | `internal/assets/assets.go` |
| `<!-- pcs:persona -->` ... `<!-- /pcs:persona -->` | Coordinator persona (`gentleman` or `neutral`) and English Artifact Language Contract | `internal/assets/assets.go` |
| `<!-- pcs:engram-protocol -->` ... `<!-- /pcs:engram-protocol -->` | Persistent cross-session memory protocol (`mcp__engram__*`) | `internal/assets/assets.go` |
| `<!-- pcs:orchestrator -->` ... `<!-- /pcs:orchestrator -->` | Product & UX/UI Coordinator role, Lossless Blocking Prompts, Allowed Edit Surfaces, Flow-First Rule (`pcs flow start` / `pcs flow next --json`), and the Launch Payload Contract | `brain/orchestrator/pcs-orchestrator.md` |
| `<!-- pcs:ux-routing -->` ... `<!-- /pcs:ux-routing -->` | Flow kinds (`audit`, `inline`, `t2`, `t3`) and the mandatory 8-step Product-Centered Development (PCD) workflow across the 5 Planes of UX | `brain/orchestrator/pcs-routing.md` |

---

## 5. Go CLI Architecture (`cmd/pcs` & `internal/`)

| Package | Responsibility | Exact Path |
|---------|----------------|------------|
| `pcs` (root) | Embeds `all:brain` and `docs/skill-style-guide.md` into `BrainFS` via `//go:embed` | `embed.go` |
| `internal/assets` | Typed accessors for embedded orchestrator blocks, native subagents, skills, and `pcs:*` infrastructure blocks (`pcs:codegraph-guidance`, `pcs:persona`, `pcs:engram-protocol`) | `internal/assets/assets.go` |
| `internal/marker` | Idempotent, CRLF-aware HTML-comment section manager (`UpsertBlock`, `RemoveBlock`, `HasBlock`, `ExtractBlock`) for `CLAUDE.md` | `internal/marker/marker.go` |
| `internal/mcp` | Atomic JSON merger/unmerger with one-time `.pre-pcs.bak` backups and delta tracking for `engram`, `context7`, and `codegraph` MCP servers in `.claude.json`, and permissions, the skill-registry hook, and the PCD flow hooks in `settings.json` | `internal/mcp/mcp.go` |
| `internal/installer` | Multi-scope (`global` default respecting `$CLAUDE_CONFIG_DIR` and `$PCS_INSTALL_SCOPE`, `workspace`, and `profile` selected only by `--profile-dir`, defaulting to `~/.claude-pcs` and never the home dir, the main config dir, or its ancestors) installer, syncer, uninstaller, status inspector, `.pcs-state.json` tracker, cross-platform (`sh`, `.cmd`, `.ps1`) launcher generator, and build metadata (`version`/`commit`/`date` set by `-ldflags -X`, with a `debug.ReadBuildInfo` fallback) | `internal/installer/installer.go` |
| `internal/registry` | Deterministic `.atl/pcs-skill-registry.md` table generator with YAML frontmatter parser and SHA-256 content hash fast path | `internal/registry/registry.go` |
| `internal/doctor` | Ecosystem health verifier for `engram`, `codegraph`, `npx`, `git`, embedded `BrainFS`, and target installation state | `internal/doctor/doctor.go` |
| `internal/flow` | Deterministic PCD flow state machine (`pcs flow start\|status\|next\|record\|close`), `<project>/.pcs/flows/<feature>.json` state (`pcs.flow/v1`), and typed `pcs.flow.transition/v1` next transitions | `internal/flow/flow.go` |
| `internal/hook` | Claude Code hook commands (`pcs hook pre-tool-use\|subagent-stop\|stop\|user-prompt-submit`) that gate main-thread reads, edits, and delegations, record phases from Return Envelopes, and block incomplete turns; they fail open | `internal/hook/hook.go` |
| `internal/cli` | CLI command router (`install`, `sync`, `uninstall`, `status`, `doctor`, `skill-registry refresh`, `codegraph init`, `flow`, `hook`, `version`, `help`) | `internal/cli/cli.go` |
