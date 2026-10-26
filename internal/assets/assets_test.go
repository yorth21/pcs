package assets

import (
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
				"<!-- gentle-ai:remote-authorization -->",
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
	if len(agents) != 5 {
		t.Fatalf("expected 5 native agents, got %d", len(agents))
	}

	for _, agent := range agents {
		t.Run(agent.RelPath, func(t *testing.T) {
			text := string(agent.Content)
			if !strings.HasPrefix(text, "---\n") {
				t.Errorf("agent %s missing YAML frontmatter start", agent.RelPath)
			}
			if !strings.Contains(text, "name: "+strings.TrimSuffix(agent.RelPath, ".md")) {
				t.Errorf("agent %s missing matching YAML name field", agent.RelPath)
			}
			if !strings.Contains(text, "mcp__engram__") {
				t.Errorf("agent %s missing engram MCP tool declaration", agent.RelPath)
			}
		})
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
	if !strings.Contains(cg, "<!-- gentle-ai:codegraph-guidance -->") {
		t.Error("expected codegraph guidance marker")
	}

	engram := EngramProtocolBlock()
	if !strings.Contains(engram, "<!-- gentle-ai:engram-protocol -->") {
		t.Error("expected engram protocol marker")
	}

	for _, variant := range []string{"gentleman", "neutral"} {
		t.Run("persona_"+variant, func(t *testing.T) {
			p, err := PersonaBlock(variant)
			if err != nil {
				t.Fatalf("PersonaBlock(%q) error: %v", variant, err)
			}
			if !strings.Contains(p, "<!-- gentle-ai:persona -->") {
				t.Errorf("PersonaBlock(%q) missing start marker", variant)
			}
		})
	}

	if _, err := PersonaBlock("invalid"); err == nil {
		t.Fatal("expected error for invalid persona")
	}
}
