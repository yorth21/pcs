package assets

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/yorth21/pcs"
)

const (
	MarkerOrchestrator      = "pcs:orchestrator"
	MarkerUXRouting         = "pcs:ux-routing"
	MarkerCodeGraphGuidance = "pcs:codegraph-guidance"
	MarkerPersona           = "pcs:persona"
	MarkerEngramProtocol    = "pcs:engram-protocol"
)

// AllPCSMarkers lists all five CLAUDE.md section markers owned and managed by pcs.
var AllPCSMarkers = []string{
	MarkerCodeGraphGuidance,
	MarkerPersona,
	MarkerEngramProtocol,
	MarkerOrchestrator,
	MarkerUXRouting,
}

// NativeAgentNames lists the five canonical Product Composition System subagents.
var NativeAgentNames = []string{
	"pcs-explorer.md",
	"pcs-strategist.md",
	"pcs-architect.md",
	"pcs-builder.md",
	"pcs-verifier.md",
}

// SupportedAdapters lists the stack adapter names shipped in the embedded brain.
var SupportedAdapters = []string{
	"react-shadcn",
}

// FileEntry represents a relative file path and its embedded byte contents.
type FileEntry struct {
	RelPath string
	Content []byte
}

// OrchestratorBlock returns the embedded <!-- pcs:orchestrator --> block.
func OrchestratorBlock() (string, error) {
	b, err := pcs.BrainFS.ReadFile("brain/orchestrator/pcs-orchestrator.md")
	if err != nil {
		return "", fmt.Errorf("read orchestrator block: %w", err)
	}
	return string(b), nil
}

// UXRoutingBlock returns the embedded <!-- pcs:ux-routing --> block.
func UXRoutingBlock() (string, error) {
	b, err := pcs.BrainFS.ReadFile("brain/orchestrator/pcs-routing.md")
	if err != nil {
		return "", fmt.Errorf("read ux-routing block: %w", err)
	}
	return string(b), nil
}

// SharedPhaseContract returns the embedded brain/skills/_shared/pcs-phase-common.md content.
func SharedPhaseContract() ([]byte, error) {
	b, err := pcs.BrainFS.ReadFile("brain/skills/_shared/pcs-phase-common.md")
	if err != nil {
		return nil, fmt.Errorf("read shared phase contract: %w", err)
	}
	return b, nil
}

// SkillStyleGuide returns the embedded docs/skill-style-guide.md content.
func SkillStyleGuide() ([]byte, error) {
	b, err := pcs.BrainFS.ReadFile("docs/skill-style-guide.md")
	if err != nil {
		return nil, fmt.Errorf("read skill style guide: %w", err)
	}
	return b, nil
}

// NativeAgents returns the five embedded native subagent markdown files.
func NativeAgents() ([]FileEntry, error) {
	entries := make([]FileEntry, 0, len(NativeAgentNames))
	for _, name := range NativeAgentNames {
		full := path.Join("brain/agents", name)
		b, err := pcs.BrainFS.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("read native agent %s: %w", name, err)
		}
		entries = append(entries, FileEntry{
			RelPath: name,
			Content: b,
		})
	}
	return entries, nil
}

// CoreSkillFiles returns all embedded files under brain/skills/pcs-core/ with
// paths relative to the skills directory (e.g. "pcs-core/SKILL.md").
func CoreSkillFiles() ([]FileEntry, error) {
	return walkSkillSubtree("pcs-core")
}

// AdapterSkillDirName maps a short adapter name (e.g. "react-shadcn") or full
// skill name ("pcs-adapter-react-shadcn") to its embedded directory name.
func AdapterSkillDirName(adapter string) (string, error) {
	trimmed := strings.TrimSpace(adapter)
	trimmed = strings.TrimPrefix(trimmed, "pcs-adapter-")
	for _, supported := range SupportedAdapters {
		if trimmed == supported {
			return "pcs-adapter-" + supported, nil
		}
	}
	return "", fmt.Errorf("unsupported stack adapter %q (supported: %s)", adapter, strings.Join(SupportedAdapters, ", "))
}

// AdapterSkillFiles returns all embedded files for the requested stack adapter
// with paths relative to the skills directory (e.g. "pcs-adapter-react-shadcn/SKILL.md").
func AdapterSkillFiles(adapter string) ([]FileEntry, error) {
	dirName, err := AdapterSkillDirName(adapter)
	if err != nil {
		return nil, err
	}
	return walkSkillSubtree(dirName)
}

func walkSkillSubtree(skillDirName string) ([]FileEntry, error) {
	root := path.Join("brain/skills", skillDirName)
	var out []FileEntry
	err := fs.WalkDir(pcs.BrainFS, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, readErr := pcs.BrainFS.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		rel := strings.TrimPrefix(p, "brain/skills/")
		out = append(out, FileEntry{
			RelPath: rel,
			Content: b,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk embedded skill %s: %w", skillDirName, err)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RelPath < out[j].RelPath
	})
	return out, nil
}

// CodeGraphGuidanceBlock returns the <!-- pcs:codegraph-guidance --> block
// injected into CLAUDE.md when MCP infrastructure is enabled.
func CodeGraphGuidanceBlock() string {
	return `<!-- pcs:codegraph-guidance -->
## CodeGraph Intelligence Protocol

When ` + "`codegraph`" + ` MCP tools (` + "`mcp__codegraph__codegraph_explore`" + `) are available, use structural AST exploration before broad filesystem scans:
1. If ` + "`.codegraph/`" + ` is missing in a real project workspace, initialize it once via ` + "`pcs codegraph init --cwd <project-root>`" + ` (which validates the root directory before calling ` + "`codegraph init`" + `).
2. Call ` + "`codegraph_explore`" + ` first to trace routes, component trees, hooks, symbol callers, and blast radius in a single call, then read only the returned file ranges.
3. Fall back to ` + "`Grep`" + ` / ` + "`Glob`" + ` only for non-code assets, styles, or when CodeGraph is unavailable.
<!-- /pcs:codegraph-guidance -->`
}

// PersonaBlock returns the <!-- pcs:persona --> block for the requested
// persona variant ("gentleman" or "neutral").
func PersonaBlock(persona string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(persona)) {
	case "", "gentleman":
		return `<!-- pcs:persona -->
## Persona & Artifact Language Contract

- **Role**: Senior Product & Software Architect (15+ years experience). Direct, caring, and rigorous mentor who prioritizes user outcomes, cognitive clarity, and solid architectural foundations over impulsive code generation.
- **Persona Scope (CRITICAL)**: Conversational tone and language apply ONLY to direct chat replies to the user.
- **Artifact Language Contract**: Every technical or product artifact (` + "`pcs/specs/*`" + `, ` + "`pcs/tasks/*`" + `, code identifiers, comments, UI copy, accessibility labels, tests, docs, and commits) MUST be in rigorous, professional English unless the project explicitly mandates another UI locale.
- **Zero External Company Shorthand**: Never cite external product companies as shorthand for UX quality; state the concrete cognitive law, usability heuristic, or design token rule directly.
<!-- /pcs:persona -->`, nil
	case "neutral":
		return `<!-- pcs:persona -->
## Persona & Artifact Language Contract

- **Role**: Principal Product & Frontend Systems Engineer. Concise, analytical, and evidence-driven.
- **Persona Scope (CRITICAL)**: Match the user's language in direct conversation while keeping all technical and product artifacts (` + "`pcs/specs/*`" + `, ` + "`pcs/tasks/*`" + `, source code, UI strings, tests, docs, and commits) in rigorous, professional English.
- **Zero External Company Shorthand**: Never cite external product companies as shorthand for UX quality; state the concrete cognitive law, usability heuristic, or design token rule directly.
<!-- /pcs:persona -->`, nil
	default:
		return "", fmt.Errorf("unsupported persona %q (supported: gentleman, neutral)", persona)
	}
}

// EngramProtocolBlock returns the <!-- pcs:engram-protocol --> block
// injected into CLAUDE.md when MCP infrastructure is enabled.
func EngramProtocolBlock() string {
	return `<!-- pcs:engram-protocol -->
## Engram Persistent Memory — Protocol

Engram (` + "`mcp__engram__*`" + `) provides persistent cross-session memory across compactions:
1. **Session Detection**: Call ` + "`mem_current_project`" + ` with the workspace directory at session start and reuse the active session identity.
2. **Dual PCD Persistence**: Every ` + "`pcs-*`" + ` phase mirrors its specification and task state to Engram under ` + "`pcs/{feature-name}/{explore|strategy|architecture|tasks|apply|verify}`" + ` (` + "`capture_prompt: false`" + `).
3. **Recovery & Delivery**: Before resuming or after compaction, reconcile ` + "`mem_context`" + ` / ` + "`mem_search`" + ` / ` + "`mem_get_observation`" + ` with on-disk ` + "`pcs/specs/`" + ` and ` + "`pcs/tasks/`" + `. Saving to Engram never replaces the user-facing reply.
<!-- /pcs:engram-protocol -->`
}
