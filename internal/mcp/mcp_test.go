package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestResolveEngramBinaryPrefersLookPathFirst(t *testing.T) {
	origLookPath := LookPathFunc
	t.Cleanup(func() { LookPathFunc = origLookPath })

	LookPathFunc = func(file string) (string, error) {
		return "/custom/user/bin/engram", nil
	}
	if got := ResolveEngramBinary(); got != "/custom/user/bin/engram" {
		t.Fatalf("ResolveEngramBinary() = %q, want /custom/user/bin/engram", got)
	}
}

func TestMergeClaudeJSON(t *testing.T) {
	tests := []struct {
		name           string
		initialJSON    string
		components     []string
		wantBackup     bool
		wantCustomKey  string
		wantMCPServers []string
		wantAdded      []string
	}{
		{
			name:           "creates new .claude.json with all three MCP servers when file does not exist",
			initialJSON:    "",
			components:     nil,
			wantBackup:     false,
			wantMCPServers: []string{"engram", "context7", "codegraph"},
			wantAdded:      []string{"engram", "context7", "codegraph"},
		},
		{
			name: "preserves existing projects, custom MCP servers, and pre-existing engram config without overwriting",
			initialJSON: `{
  "numStartups": 42,
  "projects": {
    "/Users/dev/app": {"hasTrustDialogAccepted": true}
  },
  "mcpServers": {
    "playwright": {"command": "npx", "args": ["@playwright/mcp@latest"]},
    "engram": {"command": "/custom/engram", "args": ["mcp", "--tools=all"]}
  }
}`,
			components:     []string{"engram", "context7", "codegraph"},
			wantBackup:     true,
			wantCustomKey:  "numStartups",
			wantMCPServers: []string{"playwright", "engram", "context7", "codegraph"},
			wantAdded:      []string{"context7", "codegraph"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			claudePath := filepath.Join(tmpDir, ".claude.json")

			if tt.initialJSON != "" {
				if err := os.WriteFile(claudePath, []byte(tt.initialJSON), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			res, err := MergeClaudeJSONWithDelta(claudePath, tt.components, true)
			if err != nil {
				t.Fatalf("MergeClaudeJSONWithDelta() error: %v", err)
			}
			if !slices.Equal(res.AddedServers, tt.wantAdded) {
				t.Errorf("AddedServers = %v, want %v", res.AddedServers, tt.wantAdded)
			}

			_, bakErr := os.Stat(claudePath + PrePCSBackupSuffix)
			if tt.wantBackup && bakErr != nil {
				t.Errorf("expected backup file %s%s to exist, got err: %v", claudePath, PrePCSBackupSuffix, bakErr)
			}
			if !tt.wantBackup && bakErr == nil {
				t.Errorf("did not expect backup file when initial file was absent")
			}

			// Running a second merge must NOT overwrite .pre-pcs.bak!
			if tt.wantBackup {
				if err := MergeClaudeJSON(claudePath, tt.components); err != nil {
					t.Fatalf("second MergeClaudeJSON() error: %v", err)
				}
				bakBytes, err := os.ReadFile(claudePath + PrePCSBackupSuffix)
				if err != nil {
					t.Fatal(err)
				}
				if string(bakBytes) != tt.initialJSON {
					t.Errorf(".pre-pcs.bak was overwritten on subsequent run:\ngot:  %s\nwant: %s", string(bakBytes), tt.initialJSON)
				}
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

			// Verify pre-existing engram command/args were NOT overwritten.
			if tt.initialJSON != "" {
				engramSrv := servers["engram"].(map[string]any)
				if engramSrv["command"] != "/custom/engram" {
					t.Errorf("expected pre-existing engram command /custom/engram to be untouched, got %v", engramSrv["command"])
				}
			}
		})
	}
}

func TestMergeSettingsJSONAndUnmergeWithDelta(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")
	claudePath := filepath.Join(tmpDir, ".claude.json")

	initialClaude := `{
  "mcpServers": {
    "engram": {"command": "/custom/engram", "args": ["mcp", "--tools=all"]}
  }
}`
	if err := os.WriteFile(claudePath, []byte(initialClaude), 0o600); err != nil {
		t.Fatal(err)
	}

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

	claudeDelta, err := MergeClaudeJSONWithDelta(claudePath, nil, true)
	if err != nil {
		t.Fatalf("MergeClaudeJSONWithDelta() error: %v", err)
	}
	settingsDelta, err := MergeSettingsJSONWithDelta(settingsPath, nil, `C:\Users\dev\.claude-pcs`, true, true, true)
	if err != nil {
		t.Fatalf("MergeSettingsJSONWithDelta() error: %v", err)
	}

	// Running MergeSettingsJSON a second time must not duplicate permissions or hooks.
	if err := MergeSettingsJSON(settingsPath, nil, `C:\Users\dev\.claude-pcs`); err != nil {
		t.Fatalf("second MergeSettingsJSON() error: %v", err)
	}

	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	// Verify Windows backslashes in targetDir were normalized to forward slashes in hook command.
	if strings.Contains(string(raw), `\\`) {
		t.Errorf("expected hook command to use forward slashes, got:\n%s", string(raw))
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

	// Unmerge using recorded delta must remove context7 and codegraph while keeping pre-existing engram!
	err = UnmergeWithDelta(claudePath, settingsPath, UnmergeDelta{
		CreatedClaudeJSON:   claudeDelta.CreatedFile,
		CreatedSettingsJSON: settingsDelta.CreatedFile,
		AddedMCPServers:     claudeDelta.AddedServers,
		AddedPermissions:    settingsDelta.AddedPermissions,
		AddedPlugins:        settingsDelta.AddedPlugins,
		AddedHook:           settingsDelta.AddedHook,
	})
	if err != nil {
		t.Fatalf("UnmergeWithDelta() error: %v", err)
	}

	afterStatus, err := InspectConfiguredServers(claudePath)
	if err != nil {
		t.Fatal(err)
	}
	if !afterStatus[ServerEngram] {
		t.Errorf("expected pre-existing engram server to survive UnmergeWithDelta")
	}
	if afterStatus[ServerContext7] || afterStatus[ServerCodeGraph] {
		t.Errorf("expected pcs-added context7 and codegraph to be removed, got %+v", afterStatus)
	}

	rawSettingsAfter, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawSettingsAfter), "Bash(git status:*)") {
		t.Errorf("expected custom permission Bash(git status:*) to survive UnmergeWithDelta")
	}
	if !strings.Contains(string(rawSettingsAfter), "mcp__engram__*") {
		t.Errorf("expected pre-existing permission mcp__engram__* to survive UnmergeWithDelta")
	}
}
