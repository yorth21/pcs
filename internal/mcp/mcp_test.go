package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMergeClaudeJSON(t *testing.T) {
	tests := []struct {
		name           string
		initialJSON    string
		components     []string
		wantBackup     bool
		wantCustomKey  string
		wantMCPServers []string
	}{
		{
			name:           "creates new .claude.json with all three MCP servers when file does not exist",
			initialJSON:    "",
			components:     nil,
			wantBackup:     false,
			wantMCPServers: []string{"engram", "context7", "codegraph"},
		},
		{
			name: "preserves existing projects and custom MCP servers and creates backup",
			initialJSON: `{
  "numStartups": 42,
  "projects": {
    "/Users/dev/app": {"hasTrustDialogAccepted": true}
  },
  "mcpServers": {
    "playwright": {"command": "npx", "args": ["@playwright/mcp@latest"]}
  }
}`,
			components:     []string{"engram", "context7", "codegraph"},
			wantBackup:     true,
			wantCustomKey:  "numStartups",
			wantMCPServers: []string{"playwright", "engram", "context7", "codegraph"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			claudePath := filepath.Join(tmpDir, ".claude.json")

			if tt.initialJSON != "" {
				if err := os.WriteFile(claudePath, []byte(tt.initialJSON), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			if err := MergeClaudeJSON(claudePath, tt.components); err != nil {
				t.Fatalf("MergeClaudeJSON() error: %v", err)
			}

			_, bakErr := os.Stat(claudePath + ".bak")
			if tt.wantBackup && bakErr != nil {
				t.Errorf("expected backup file %s.bak to exist, got err: %v", claudePath, bakErr)
			}
			if !tt.wantBackup && bakErr == nil {
				t.Errorf("did not expect backup file when initial file was absent")
			}

			raw, err := os.ReadFile(claudePath)
			if err != nil {
				t.Fatalf("read merged .claude.json: %v", err)
			}
			var parsed map[string]any
			if err := json.Unmarshal(raw, &parsed); err != nil {
				t.Fatalf("unmarshal merged .claude.json: %v", err)
			}

			if tt.wantCustomKey != "" {
				if _, ok := parsed[tt.wantCustomKey]; !ok {
					t.Errorf("expected existing key %q to be preserved", tt.wantCustomKey)
				}
			}

			servers, ok := parsed["mcpServers"].(map[string]any)
			if !ok {
				t.Fatalf("expected mcpServers map in .claude.json")
			}
			for _, srv := range tt.wantMCPServers {
				if _, exists := servers[srv]; !exists {
					t.Errorf("expected MCP server %q to be present", srv)
				}
			}
		})
	}
}

func TestMergeSettingsJSONAndUnmerge(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")
	claudePath := filepath.Join(tmpDir, ".claude.json")

	initialSettings := `{
  "theme": "dark",
  "permissions": {
    "allow": [
      "Bash(git status:*)",
      "mcp__engram__*"
    ]
  }
}`
	if err := os.WriteFile(settingsPath, []byte(initialSettings), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MergeClaudeJSON(claudePath, nil); err != nil {
		t.Fatalf("MergeClaudeJSON() error: %v", err)
	}
	if err := MergeSettingsJSON(settingsPath, nil, tmpDir); err != nil {
		t.Fatalf("MergeSettingsJSON() error: %v", err)
	}

	// Running MergeSettingsJSON a second time must not duplicate permissions or hooks.
	if err := MergeSettingsJSON(settingsPath, nil, tmpDir); err != nil {
		t.Fatalf("second MergeSettingsJSON() error: %v", err)
	}

	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed["theme"] != "dark" {
		t.Errorf("expected user key theme=dark to be preserved, got %v", parsed["theme"])
	}

	perms := parsed["permissions"].(map[string]any)
	allow := ensureStringSlice(perms, "allow")

	// Count occurrences of mcp__engram__* to prove deduplication.
	countEngram := 0
	for _, a := range allow {
		if a == "mcp__engram__*" {
			countEngram++
		}
	}
	if countEngram != 1 {
		t.Errorf("expected mcp__engram__* once, got %d in %v", countEngram, allow)
	}
	for _, expectedPerm := range DefaultPermissionAllows {
		if !slices.Contains(allow, expectedPerm) {
			t.Errorf("expected permission %q in allow list %v", expectedPerm, allow)
		}
	}

	plugins := parsed["enabledPlugins"].(map[string]any)
	if plugins["engram@engram"] != true {
		t.Errorf("expected enabledPlugins[engram@engram] = true, got %v", plugins["enabledPlugins"])
	}

	// Verify InspectConfiguredServers reports all three true.
	status, err := InspectConfiguredServers(claudePath)
	if err != nil {
		t.Fatalf("InspectConfiguredServers() error: %v", err)
	}
	for _, comp := range DefaultComponents {
		if !status[comp] {
			t.Errorf("expected InspectConfiguredServers[%q] = true", comp)
		}
	}

	// Unmerge with onlyPCSOwned=false removes MCP servers and pcs hooks while keeping Bash(git status:*).
	if err := UnmergeMCP(claudePath, settingsPath, false); err != nil {
		t.Fatalf("UnmergeMCP() error: %v", err)
	}
	afterStatus, err := InspectConfiguredServers(claudePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, comp := range DefaultComponents {
		if afterStatus[comp] {
			t.Errorf("expected %q to be removed after full UnmergeMCP", comp)
		}
	}

	rawSettingsAfter, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawSettingsAfter), "Bash(git status:*)") {
		t.Errorf("expected custom permission Bash(git status:*) to survive UnmergeMCP")
	}
}
