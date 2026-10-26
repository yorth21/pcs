# ODD Task Tracker — `pcs-brain` (Product Composition System Brain)

## Objective
Design, implement, audit, and polish the complete LLM-first **Brain** of `pcs` (Product Composition System) inside `/Users/yorth21/Developer/SideProjects/pcs`. `pcs` directly integrates `Engram`, `Persona`, `Context7`, `CodeGraph`, and runtime permissions and injects the **PCD (Product-Centered Development)** orchestrator, 5 native subagents, shared phase contracts, the technology-agnostic `pcs-core` UX methodology skill (zero external company names), and the `pcs-adapter-react-shadcn` stack adapter skill.

## Work Units

- [x] **WU-1: Foundation, Style Guide & Architecture Docs**
  - `README.md` (Executive overview, architecture diagram, PCD workflow, Core + Adapter model, standalone CLI integration)
  - `AGENTS.md` (Skill & agent registry and repository rules)
  - `docs/skill-style-guide.md` (Normative LLM-first skill style guide)
  - `docs/architecture.md` (Deep technical specification of the standalone `pcs` architecture, HTML comment injection blocks, Engram topic schema, and profile isolation)
  - **Verification**: All 4 root and architecture documents created and cross-linked.
- [x] **WU-2: Orchestrator & PCD Routing Blocks (`brain/orchestrator/` & `brain/skills/_shared/`)**
  - `brain/orchestrator/pcs-orchestrator.md` (`<!-- pcs:orchestrator -->`: Coordinator contract, UX Delegation Triggers, Allowed Edit Surfaces, Lossless Blocking Prompts, Skill & Adapter Resolution Protocol)
  - `brain/orchestrator/pcs-routing.md` (`<!-- pcs:ux-routing -->`: Mandatory PCD 8-step protocol across the 5 Planes of UX)
  - `brain/skills/_shared/pcs-phase-common.md` (Shared artifact locator, dual-persistence contract in `pcs/specs/<feature>.md` + `pcs/tasks/<feature>.md` + Engram `pcs/<feature>/*`, 0–4 severity scale, Return Envelope)
  - **Verification**: HTML comment markers, Lossless Blocking Prompts, Allowed Edit Surfaces, and dual-persistence contracts verified.
- [x] **WU-3: Native Subagents (`brain/agents/`)**
  - `brain/agents/pcs-explorer.md` (Read-only UX/UI & Frontend Codebase Mapper)
  - `brain/agents/pcs-strategist.md` (Planes 1 & 2: Strategy & Scope — JTBD, Real Context, OOUX ORCA, HTA/KLM Step Budget)
  - `brain/agents/pcs-architect.md` (Planes 3 & 4: Structure & Skeleton — IA, 5 Mandatory Negative Paths, Context-Dependent Container Selection, Risk-Proportional Friction, Text Wireframe)
  - `brain/agents/pcs-builder.md` (Plane 5: Surface — Stack Adapter consumer, Context7 live doc verifier, Allowed Edit Surfaces enforcer)
  - `brain/agents/pcs-verifier.md` (Transversal Verification — 10 Heuristics 0–4, 4-Question Cognitive Walkthrough, WCAG AA & Keyboard Walk, Playwright verification)
  - **Verification**: All 5 native subagents include valid YAML frontmatter, MCP tool declarations, non-delegation guardrails, Engram `mem_save` (`capture_prompt: false`), Result Contract, CodeGraph guidance, Artifact Language Contract, and Remote Authorization blocks.
- [x] **WU-4: Technology-Agnostic UX Core Skill (`brain/skills/pcs-core/`)**
  - `brain/skills/pcs-core/SKILL.md` (Strict LLM-first format, <=450 words/tokens, 143-char quoted `description`)
  - `brain/skills/pcs-core/references/01-planes-strategy-and-scope.md`
  - `brain/skills/pcs-core/references/02-planes-structure-and-skeleton.md`
  - `brain/skills/pcs-core/references/03-plane-surface-and-visual-hierarchy.md`
  - `brain/skills/pcs-core/references/04-heuristic-and-walkthrough-verification.md`
  - `brain/skills/pcs-core/assets/ux-spec-template.md`
  - `brain/skills/pcs-core/assets/ux-tasks-template.md`
  - **Verification**: Verified against `docs/skill-style-guide.md`; all local references and templates exist on disk.
- [x] **WU-5: Custom Stack Adapter Skill (`brain/skills/pcs-adapter-react-shadcn/`)**
  - `brain/skills/pcs-adapter-react-shadcn/SKILL.md` (Strict LLM-first format, <=450 words/tokens, 146-char quoted `description`)
  - `brain/skills/pcs-adapter-react-shadcn/references/01-folder-and-component-architecture.md`
  - `brain/skills/pcs-adapter-react-shadcn/references/02-vanilla-shadcn-and-radix-catalog.md`
  - `brain/skills/pcs-adapter-react-shadcn/references/03-tanstack-rhf-zod-patterns.md`
  - `brain/skills/pcs-adapter-react-shadcn/references/04-tailwind-v4-tokens-and-motion.md`
  - `brain/skills/pcs-adapter-react-shadcn/assets/composition-blueprints.tsx`
  - **Verification**: Verified against `docs/skill-style-guide.md`; TypeScript blueprints demonstrate all 5 negative states, containers, and risk-proportional friction in 100% English.
- [x] **WU-6: Adversarial Audit & Polish Gate**
  - **Dimension A — Zero External Company Name Purity**: Verified via case-insensitive regex grep across all 26 files (`README.md`, `AGENTS.md`, `docs/`, `brain/`, `odd/`) for `Apple`, `Google`, `Nubank`, `Linear`, `Vercel`, `Stripe`, `Airbnb`, `Spotify`, `Shopify`, `Cloudflare`, `Notion`, `Figma`, `Slack`, and `GitHub`. Replaced `{github-handle-or-org}` with `{author-handle-or-org}` in `docs/skill-style-guide.md`; confirmed `0` matches repo-wide.
  - **Dimension B — LLM-First Skill Style Guide Compliance (`skill-improver`)**: Audited `brain/skills/pcs-core/SKILL.md` (143-char single-line quoted `description`, ~420-word body) and `brain/skills/pcs-adapter-react-shadcn/SKILL.md` (146-char single-line quoted `description`, ~390-word body); verified exact 6-section order and confirmed all 11 local `references/` and `assets/` paths exist and are non-empty.
  - **Dimension C — Orchestrator & Subagent Wiring Completeness**: Added the explicit `Canonical Subagent Launch Payload Contract` (`## Feature locator`, `## Allowed edit surfaces`, `## Skills to load before work`, and `## Key Learnings`) to `brain/orchestrator/pcs-orchestrator.md`, `brain/orchestrator/pcs-routing.md`, and `brain/skills/_shared/pcs-phase-common.md`; standardized 3-tier skill path resolution (`paths-injected` -> `fallback-registry` -> `fallback-path` in `~/.claude-pcs/skills/` or `~/.claude/skills/`) across all 5 `brain/agents/pcs-*.md` files; added the Safe Playwright Dev Server Lifecycle (probe existing server first without killing it; use `EXIT` trap cleanup for ephemeral servers so zero orphan processes remain; fallback to static code verification when no live server is requested) to `brain/agents/pcs-verifier.md` and `brain/skills/pcs-core/references/04-heuristic-and-walkthrough-verification.md`.
  - **Dimension D — Depth of UX/UI Engineering Criteria**: Upgraded `brain/skills/pcs-core/references/01-planes-strategy-and-scope.md` and `brain/skills/pcs-core/assets/ux-spec-template.md` with Declared Assumptions (`Risk if Wrong & UX Impact` + `Validation Method`), `Measurable Outcome Metrics`, OOUX ORCA (`Core T1–T3` vs. `Metadata T4` attributes and role permissions), HTA/KLM `Unpruned Baseline vs. KISS-Pruned Budget`, and all 7 Cognitive Laws (`Fitts`, `Hick`, `Miller`, `Jakob`, `Tesler`, `Doherty`, `Gestalt`).
  - **Dimension E — Stack Adapter Completeness (`pcs-adapter-react-shadcn`)**: Added React 19 `React.ComponentProps<"element">` (zero `forwardRef`) rules to `01-folder-and-component-architecture.md`, `@tanstack/react-table` v8 `ColumnDef<T>[]` and `Field` / `FieldLabel` / `FieldDescription` / `FieldError` examples to `03-tanstack-rhf-zod-patterns.md`, canonical Tailwind v4 `@theme inline` + OKLCH CSS variables to `04-tailwind-v4-tokens-and-motion.md`, and upgraded `assets/composition-blueprints.tsx` to use `<Empty>`, `<Field>`, `ColumnDef<ProcedureRecord>[]`, and `React.ComponentProps<"div">`.
