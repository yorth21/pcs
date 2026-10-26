package marker

import (
	"strings"
	"testing"
)

func TestUpsertBlock(t *testing.T) {
	tests := []struct {
		name         string
		initial      string
		markerID     string
		blockContent string
		wantContains []string
		wantExact    string
	}{
		{
			name:         "inserts into empty content and wraps tags automatically",
			initial:      "",
			markerID:     "pcs:orchestrator",
			blockContent: "# Orchestrator\nRules here.",
			wantExact:    "<!-- pcs:orchestrator -->\n# Orchestrator\nRules here.\n<!-- /pcs:orchestrator -->\n",
		},
		{
			name:         "preserves pre-tagged block without duplicating markers",
			initial:      "",
			markerID:     "pcs:ux-routing",
			blockContent: "<!-- pcs:ux-routing -->\n## Routing\n<!-- /pcs:ux-routing -->\n",
			wantExact:    "<!-- pcs:ux-routing -->\n## Routing\n<!-- /pcs:ux-routing -->\n",
		},
		{
			name:         "appends to existing user content with blank line separator",
			initial:      "# User Project Rules\n\nAlways run tests.\n",
			markerID:     "pcs:orchestrator",
			blockContent: "# PCS Coordinator",
			wantExact:    "# User Project Rules\n\nAlways run tests.\n\n<!-- pcs:orchestrator -->\n# PCS Coordinator\n<!-- /pcs:orchestrator -->\n",
		},
		{
			name:         "replaces existing block in place preserving surrounding blocks",
			initial:      "# Header\n\n<!-- pcs:orchestrator -->\nOld content\n<!-- /pcs:orchestrator -->\n\n# Footer\n",
			markerID:     "pcs:orchestrator",
			blockContent: "New content v2",
			wantExact:    "# Header\n\n<!-- pcs:orchestrator -->\nNew content v2\n<!-- /pcs:orchestrator -->\n\n# Footer\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UpsertBlock(tt.initial, tt.markerID, tt.blockContent)
			if tt.wantExact != "" && got != tt.wantExact {
				t.Fatalf("UpsertBlock() mismatch:\ngot:\n%q\nwant:\n%q", got, tt.wantExact)
			}
			for _, sub := range tt.wantContains {
				if !strings.Contains(got, sub) {
					t.Errorf("expected output to contain %q, got:\n%s", sub, got)
				}
			}
			if !HasBlock(got, tt.markerID) {
				t.Errorf("expected HasBlock(%q) to be true after UpsertBlock", tt.markerID)
			}
		})
	}
}

func TestUpsertBlockIdempotencyAndMultiBlockOrdering(t *testing.T) {
	base := "# Team Handbook\n"
	first := UpsertBlock(base, "gentle-ai:persona", "Persona v1")
	second := UpsertBlock(first, "pcs:orchestrator", "Orchestrator v1")
	third := UpsertBlock(second, "pcs:ux-routing", "Routing v1")

	// Re-upserting the same blocks must be idempotent.
	again := UpsertBlock(third, "pcs:orchestrator", "Orchestrator v1")
	again = UpsertBlock(again, "pcs:ux-routing", "Routing v1")
	if again != third {
		t.Fatalf("expected idempotent UpsertBlock output:\ngot:\n%s\nwant:\n%s", again, third)
	}

	// Updating middle block keeps order: persona -> orchestrator -> ux-routing.
	updated := UpsertBlock(again, "pcs:orchestrator", "Orchestrator v2")
	idxPersona := strings.Index(updated, "<!-- gentle-ai:persona -->")
	idxOrch := strings.Index(updated, "<!-- pcs:orchestrator -->")
	idxRoute := strings.Index(updated, "<!-- pcs:ux-routing -->")
	if !(idxPersona >= 0 && idxPersona < idxOrch && idxOrch < idxRoute) {
		t.Fatalf("expected block order persona < orchestrator < ux-routing, got indices %d, %d, %d in:\n%s",
			idxPersona, idxOrch, idxRoute, updated)
	}
}

func TestRemoveBlock(t *testing.T) {
	tests := []struct {
		name        string
		initial     string
		markerID    string
		wantRemoved bool
		wantContent string
	}{
		{
			name:        "returns false when marker does not exist",
			initial:     "# Existing Content\n",
			markerID:    "pcs:orchestrator",
			wantRemoved: false,
			wantContent: "# Existing Content\n",
		},
		{
			name:        "removes only target block and avoids triple newlines",
			initial:     "# Intro\n\n<!-- pcs:orchestrator -->\nCoord\n<!-- /pcs:orchestrator -->\n\n<!-- pcs:ux-routing -->\nRoute\n<!-- /pcs:ux-routing -->\n\n# Outro\n",
			markerID:    "pcs:orchestrator",
			wantRemoved: true,
			wantContent: "# Intro\n\n<!-- pcs:ux-routing -->\nRoute\n<!-- /pcs:ux-routing -->\n\n# Outro\n",
		},
		{
			name:        "returns empty string when last block is removed from file",
			initial:     "<!-- pcs:orchestrator -->\nOnly block\n<!-- /pcs:orchestrator -->\n",
			markerID:    "pcs:orchestrator",
			wantRemoved: true,
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, removed := RemoveBlock(tt.initial, tt.markerID)
			if removed != tt.wantRemoved {
				t.Fatalf("RemoveBlock() removed = %v, want %v", removed, tt.wantRemoved)
			}
			if got != tt.wantContent {
				t.Fatalf("RemoveBlock() content mismatch:\ngot:\n%q\nwant:\n%q", got, tt.wantContent)
			}
			if strings.Contains(got, "\n\n\n") {
				t.Errorf("RemoveBlock() left triple newlines in:\n%q", got)
			}
		})
	}
}

func TestExtractBlock(t *testing.T) {
	doc := "Header\n\n<!-- pcs:orchestrator -->\nBody\n<!-- /pcs:orchestrator -->\n"
	block, ok := ExtractBlock(doc, "pcs:orchestrator")
	if !ok {
		t.Fatal("expected ExtractBlock to find pcs:orchestrator")
	}
	want := "<!-- pcs:orchestrator -->\nBody\n<!-- /pcs:orchestrator -->"
	if block != want {
		t.Fatalf("ExtractBlock() = %q, want %q", block, want)
	}
	if _, ok := ExtractBlock(doc, "pcs:missing"); ok {
		t.Fatal("expected ExtractBlock to return false for missing marker")
	}
}
