# AGENTS.md — Product Composition System (`pcs`) Registry

This file is the authoritative registry of orchestrator blocks, native subagents, and LLM-first skills inside the Product Composition System (`pcs`) repository.

---

## 1. Repository Governance & Hard Rules

1. **Normative Skill Style Guide**: Before creating or updating any skill in `brain/skills/`, read and obey [`docs/skill-style-guide.md`](file:///Users/yorth21/Developer/SideProjects/pcs/docs/skill-style-guide.md).
2. **Artifact Language Contract**: Every file, prompt, specification, template, comment, UI string, and code blueprint in this repository MUST be written in clear, rigorous, professional English. Never inject regional slang or non-English identifiers into any artifact.
3. **Zero External Company Names in `brain/`**: Inside `brain/`, never reference external product companies as shorthand for design quality. Encode every UX principle, cognitive law, heuristic, and visual rule directly as an explicit, testable engineering constraint. (Technical library and tool dependencies such as React, TypeScript, Tailwind CSS, `shadcn/ui`, Radix UI, TanStack, React Hook Form, Zod, Playwright, Engram, Context7, CodeGraph, and `gentle-ai` are permitted where technically relevant.)

---

## 2. Project Skills Index (`brain/skills/`)

| Skill Name | Scope | Trigger / Description | Exact Path |
|------------|-------|-----------------------|------------|
| `pcs-core` | Agnostic UX Methodology (Planes 1–5) | `Trigger: UX design, new feature, screen flow, wireframe, usability audit, empty states, PCD planes. Apply technology-agnostic 5-Plane UX rules.` | `brain/skills/pcs-core/SKILL.md` |
| `pcs-adapter-react-shadcn` | Frontend Stack Adapter (React 19 + shadcn/ui) | `Trigger: React component, shadcn/ui, Tailwind v4, TanStack Table/Router/Query, React Hook Form, Zod, UI build. Compose vanilla shadcn/ui surfaces.` | `brain/skills/pcs-adapter-react-shadcn/SKILL.md` |

### Shared Skill Protocol

| Shared File | Consumer | Purpose | Exact Path |
|-------------|----------|---------|------------|
| `pcs-phase-common` | All `pcs-*` subagents | Skill loading hierarchy, dual artifact retrieval/persistence (`pcs/specs/`, `pcs/tasks/`, Engram `pcs/<feature>/*`), 0–4 severity scale, and Result Envelope contract | `brain/skills/_shared/pcs-phase-common.md` |

---

## 3. Native Subagents Index (`brain/agents/`)

| Agent Name | PCD Plane / Role | Model | Write Access | Engram Topic Key | Exact Path |
|------------|------------------|-------|--------------|------------------|------------|
| `pcs-explorer` | Plane 0 — UX & Codebase Discovery | `sonnet` | Read-Only | `pcs/{feature-name}/explore` | `brain/agents/pcs-explorer.md` |
| `pcs-strategist` | Planes 1 & 2 — Strategy & Scope | `opus` | `pcs/specs/{feature-name}.md` | `pcs/{feature-name}/strategy` | `brain/agents/pcs-strategist.md` |
| `pcs-architect` | Planes 3 & 4 — Structure, Skeleton & Tasks | `opus` | `pcs/specs/{feature-name}.md`, `pcs/tasks/{feature-name}.md` | `pcs/{feature-name}/architecture`, `pcs/{feature-name}/tasks` | `brain/agents/pcs-architect.md` |
| `pcs-builder` | Plane 5 — Surface & Component Composition | `sonnet` | Authorized `src/**`, `pcs/tasks/{feature-name}.md` | `pcs/{feature-name}/apply` | `brain/agents/pcs-builder.md` |
| `pcs-verifier` | Transversal — Heuristics, A11y & Playwright | `sonnet` | `pcs/specs/{feature-name}.md` (Verification Report) | `pcs/{feature-name}/verify` | `brain/agents/pcs-verifier.md` |

---

## 4. Orchestrator & Routing Injection Blocks (`brain/orchestrator/`)

| Block Marker | Purpose | Exact Path |
|--------------|---------|------------|
| `<!-- pcs:orchestrator -->` ... `<!-- /pcs:orchestrator -->` | Product & UX/UI Coordinator role, Lossless Blocking Prompts, Allowed Edit Surfaces, Skill & Stack Adapter Resolution, and UX Delegation Triggers | `brain/orchestrator/pcs-orchestrator.md` |
| `<!-- pcs:ux-routing -->` ... `<!-- /pcs:ux-routing -->` | Mandatory 8-step Product-Centered Development (PCD) workflow across the 5 Planes of UX | `brain/orchestrator/pcs-routing.md` |
