package pcs

import "embed"

// BrainFS embeds the Product Composition System brain (orchestrator blocks,
// native subagents, shared contracts, core UX methodology skill, and stack
// adapters) together with the normative skill style guide.
// Note: all:brain is used so that brain/skills/_shared (which starts with an
// underscore) is included in the embedded filesystem.
//
//go:embed all:brain docs/skill-style-guide.md
var BrainFS embed.FS
