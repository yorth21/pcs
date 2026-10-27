package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedBlocksAndContracts(t *testing.T) {
	tests := []struct {
		name         string
		load         func() (string, error)
		wantContains []string
	}{
		{
			name: "orchestrator block loads with pcs:orchestrator markers",
			load: OrchestratorBlock,
			wantContains: []string{
				"<!-- pcs:orchestrator -->",
				"<!-- /pcs:orchestrator -->",
				"Product-Centered Development (PCD)",
				"## Allowed edit surfaces",
			},
		},
		{
			name: "ux-routing block loads with pcs:ux-routing markers",
			load: UXRoutingBlock,
			wantContains: []string{
				"<!-- pcs:ux-routing -->",
				"<!-- /pcs:ux-routing -->",
				"### PCD Protocol",
				"<!-- pcs:remote-authorization -->",
			},
		},
		{
			name: "shared phase contract loads with return envelope and dual persistence",
			load: func() (string, error) {
				b, err := SharedPhaseContract()
				return string(b), err
			},
			wantContains: []string{
				"pcs/specs/",
				"pcs/tasks/",
				"skill_resolution",
				".atl/pcs-skill-registry.md",
			},
		},
		{
			name: "skill style guide loads from docs/skill-style-guide.md",
			load: func() (string, error) {
				b, err := SkillStyleGuide()
				return string(b), err
			},
			wantContains: []string{
				"Activation Contract",
				"Hard Rules",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := tt.load()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.TrimSpace(content) == "" {
				t.Fatal("expected non-empty embedded content")
			}
			for _, sub := range tt.wantContains {
				if !strings.Contains(content, sub) {
					t.Errorf("expected content to contain %q", sub)
				}
			}
		})
	}
}

func TestNativeAgents(t *testing.T) {
	agents, err := NativeAgents()
	if err != nil {
		t.Fatalf("NativeAgents() error: %v", err)
	}
	if len(agents) != 6 {
		t.Fatalf("expected 6 native agents, got %d", len(agents))
	}

	for _, agent := range agents {
		t.Run(agent.RelPath, func(t *testing.T) {
			text := string(agent.Content)
			if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
				t.Errorf("agent %s missing YAML frontmatter start", agent.RelPath)
			}
			if !strings.Contains(text, "name: "+strings.TrimSuffix(agent.RelPath, ".md")) {
				t.Errorf("agent %s missing matching YAML name field", agent.RelPath)
			}
			if !strings.Contains(text, "mcp__engram__") {
				t.Errorf("agent %s missing engram MCP tool declaration", agent.RelPath)
			}
			// Read-only agents without Bash tool must never instruct running pcs codegraph init.
			if agent.RelPath == "pcs-explorer.md" || agent.RelPath == "pcs-strategist.md" || agent.RelPath == "pcs-architect.md" || agent.RelPath == "pcs-auditor.md" {
				if strings.Contains(text, "pcs codegraph init") {
					t.Errorf("read-only agent %s (without Bash tool) must not instruct running 'pcs codegraph init'", agent.RelPath)
				}
			}
		})
	}
}

func TestAuditorIsReadOnly(t *testing.T) {
	agents, err := NativeAgents()
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range agents {
		if agent.RelPath != "pcs-auditor.md" {
			continue
		}
		text := string(agent.Content)
		var tools, disallowed string
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if v, ok := strings.CutPrefix(line, "tools:"); ok {
				tools = v
			}
			if v, ok := strings.CutPrefix(line, "disallowedTools:"); ok {
				disallowed = v
			}
		}
		toolSet := map[string]bool{}
		for _, tool := range strings.Split(tools, ",") {
			toolSet[strings.TrimSpace(tool)] = true
		}
		for _, mutating := range []string{"Edit", "Write", "MultiEdit", "NotebookEdit", "Bash", "mcp__engram__mem_save"} {
			if toolSet[mutating] {
				t.Errorf("pcs-auditor must not declare %s in tools", mutating)
			}
		}
		for _, blocked := range []string{"Edit", "Write", "MultiEdit", "NotebookEdit", "Bash"} {
			if !strings.Contains(disallowed, blocked) {
				t.Errorf("pcs-auditor disallowedTools lacks %s", blocked)
			}
		}
		for _, want := range []string{"mcp__playwright__browser_navigate", "mcp__codegraph__codegraph_explore", "**Status**: success"} {
			if !strings.Contains(text, want) {
				t.Errorf("pcs-auditor lacks %q", want)
			}
		}
		return
	}
	t.Fatal("pcs-auditor.md not embedded")
}

func TestPromptSizeBudget(t *testing.T) {
	orch, err := OrchestratorBlock()
	if err != nil {
		t.Fatal(err)
	}
	routing, err := UXRoutingBlock()
	if err != nil {
		t.Fatal(err)
	}
	persona, err := PersonaBlock("gentleman")
	if err != nil {
		t.Fatal(err)
	}
	engram := EngramProtocolBlock()
	cg := CodeGraphGuidanceBlock()

	totalAlwaysLoaded := len(orch) + len(routing) + len(persona) + len(engram) + len(cg)
	const maxAlwaysLoadedBytes = 12 * 1024 // 12 KB
	if totalAlwaysLoaded > maxAlwaysLoadedBytes {
		t.Errorf("always-loaded CLAUDE.md blocks total %d bytes, exceeding budget of %d bytes (12 KB)",
			totalAlwaysLoaded, maxAlwaysLoadedBytes)
	}

	shared, err := SharedPhaseContract()
	if err != nil {
		t.Fatal(err)
	}
	const maxSharedBytes = 5 * 1024 // 5 KB
	if len(shared) > maxSharedBytes {
		t.Errorf("pcs-phase-common.md is %d bytes, exceeding budget of %d bytes (5 KB)",
			len(shared), maxSharedBytes)
	}

	agents, err := NativeAgents()
	if err != nil {
		t.Fatal(err)
	}
	const maxAgentBytes = 6656 // 6.5 KB
	for _, agent := range agents {
		if len(agent.Content) > maxAgentBytes {
			t.Errorf("agent %s is %d bytes, exceeding budget of %d bytes (6.5 KB)",
				agent.RelPath, len(agent.Content), maxAgentBytes)
		}
	}
}

func TestNoLocalFileURLsInDocsAndBrain(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	targets := []string{
		filepath.Join(repoRoot, "README.md"),
		filepath.Join(repoRoot, "AGENTS.md"),
		filepath.Join(repoRoot, "docs"),
		filepath.Join(repoRoot, "brain"),
	}

	for _, target := range targets {
		err := filepath.WalkDir(target, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(b)
			if strings.Contains(content, "file:///") {
				t.Errorf("%s contains machine-local 'file:///' link", path)
			}
			if strings.Contains(content, "/Users/") {
				t.Errorf("%s contains machine-local '/Users/' path", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", target, err)
		}
	}
}

func TestSkillsAndAdapters(t *testing.T) {
	coreFiles, err := CoreSkillFiles()
	if err != nil {
		t.Fatalf("CoreSkillFiles() error: %v", err)
	}
	if len(coreFiles) < 7 {
		t.Fatalf("expected at least 7 files in pcs-core, got %d", len(coreFiles))
	}

	adapterFiles, err := AdapterSkillFiles("react-shadcn")
	if err != nil {
		t.Fatalf("AdapterSkillFiles(react-shadcn) error: %v", err)
	}
	if len(adapterFiles) < 6 {
		t.Fatalf("expected at least 6 files in pcs-adapter-react-shadcn, got %d", len(adapterFiles))
	}

	if _, err := AdapterSkillFiles("unknown-framework"); err == nil {
		t.Fatal("expected error for unsupported adapter, got nil")
	}
}

func TestBaseInfrastructureBlocks(t *testing.T) {
	cg := CodeGraphGuidanceBlock()
	if !strings.Contains(cg, "<!-- pcs:codegraph-guidance -->") {
		t.Error("expected codegraph guidance marker")
	}

	engram := EngramProtocolBlock()
	if !strings.Contains(engram, "<!-- pcs:engram-protocol -->") {
		t.Error("expected engram protocol marker")
	}

	for _, variant := range []string{"gentleman", "neutral"} {
		t.Run("persona_"+variant, func(t *testing.T) {
			p, err := PersonaBlock(variant)
			if err != nil {
				t.Fatalf("PersonaBlock(%q) error: %v", variant, err)
			}
			if !strings.Contains(p, "<!-- pcs:persona -->") {
				t.Errorf("PersonaBlock(%q) missing start marker", variant)
			}
		})
	}

	if _, err := PersonaBlock("invalid"); err == nil {
		t.Fatal("expected error for invalid persona")
	}
}
