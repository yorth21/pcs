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

const foreignSettings = `{
  "hooks": {
    "SessionStart": [{"matcher": "", "hooks": [{"type": "command", "command": "herdr session-start"}]}],
    "Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "notify-done"}]}]
  },
  "theme": "dark"
}
`

func TestMergeFlowHooksPreservesForeignHooksAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(foreignSettings), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := `C:\Users\x\AppData\Local\pcs\bin\pcs.exe`
	changed, created, err := MergeFlowHooks(path, bin, true)
	if err != nil || !changed || created {
		t.Fatalf("first merge = %v %v %v", changed, created, err)
	}
	if _, err := os.Stat(path + PrePCSBackupSuffix); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	present, cmds, err := InspectFlowHooks(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range FlowHookSpecs {
		if !present[spec.Event] {
			t.Fatalf("%s hook missing", spec.Event)
		}
	}
	if !slices.Contains(cmds, `"C:/Users/x/AppData/Local/pcs/bin/pcs.exe" hook stop`) {
		t.Fatalf("commands = %v", cmds)
	}
	first, _ := os.ReadFile(path)
	if changed, _, err := MergeFlowHooks(path, bin, true); err != nil || changed {
		t.Fatalf("second merge changed=%v err=%v", changed, err)
	}
	if again, _ := os.ReadFile(path); string(again) != string(first) {
		t.Fatal("idempotent merge rewrote the file")
	}
	// Re-rendering with a new binary replaces, never duplicates.
	if changed, _, _ := MergeFlowHooks(path, "/opt/pcs/pcs", true); !changed {
		t.Fatal("binary change not applied")
	}
	_, cmds, _ = InspectFlowHooks(path)
	if len(cmds) != len(FlowHookSpecs) || !strings.HasPrefix(cmds[0], `"/opt/pcs/pcs" hook`) {
		t.Fatalf("re-rendered commands = %v", cmds)
	}
	var root map[string]any
	b, _ := os.ReadFile(path)
	json.Unmarshal(b, &root)
	pre := root["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)
	if !strings.Contains(pre["matcher"].(string), "Bash") || !strings.Contains(pre["matcher"].(string), "Agent") {
		t.Fatalf("PreToolUse matcher = %v", pre["matcher"])
	}

	if removed, err := RemoveFlowHooks(path); err != nil || !removed {
		t.Fatalf("remove = %v %v", removed, err)
	}
	var after, want map[string]any
	b, _ = os.ReadFile(path)
	json.Unmarshal(b, &after)
	json.Unmarshal([]byte(foreignSettings), &want)
	gotJSON, _ := json.Marshal(after)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("after remove = %s, want %s", gotJSON, wantJSON)
	}
	if removed, _ := RemoveFlowHooks(path); removed {
		t.Fatal("second remove reported a change")
	}
}

func TestUnmergeRemovesFlowHooksAndPrunesCreatedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	_, created, err := MergeFlowHooks(path, "/opt/pcs/pcs", false)
	if err != nil || !created {
		t.Fatalf("merge = %v %v", created, err)
	}
	if err := UnmergeWithDelta("", path, UnmergeDelta{CreatedSettingsJSON: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("settings.json created by pcs still present: %v", err)
	}
}

func TestIsFlowHookCommand(t *testing.T) {
	for cmd, want := range map[string]bool{
		`"C:/x/pcs.exe" hook pre-tool-use`: true,
		`/usr/local/bin/pcs hook stop`:     true,
		`pcs hook user-prompt-submit`:      true,
		`"/a b/pcs" hook subagent-stop`:    true,
		`pcs skill-registry refresh`:       false,
		`mypcs hook stop`:                  false,
		`pcs hook stop && rm -rf x`:        false,
		`herdr hook stop`:                  false,
	} {
		if got := IsFlowHookCommand(cmd); got != want {
			t.Errorf("IsFlowHookCommand(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestGenerateSharedMCPAddsPlaywright(t *testing.T) {
	tests := []struct {
		name           string
		mainJSON       string
		wantAdded      []string
		wantPCSServer  bool
		wantNoDriftFor string
	}{
		{name: "main without playwright gains the pcs server", mainJSON: `{"mcpServers": {"engram": {"command": "engram"}}}`, wantAdded: []string{"playwright"}, wantPCSServer: true},
		{name: "main playwright is kept unchanged", mainJSON: `{"mcpServers": {"engram": {"command": "engram"}, "playwright": {"command": "custom-playwright"}}}`, wantAdded: nil},
		{name: "defaults gain the pcs server", mainJSON: `{}`, wantAdded: []string{"engram", "playwright"}, wantPCSServer: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			fakeEngram := filepath.Join(dir, "engram")
			if err := os.WriteFile(fakeEngram, []byte("x"), 0o755); err != nil {
				t.Fatal(err)
			}
			origLook, origFallback := LookPathFunc, EngramFallbackPaths
			LookPathFunc = func(string) (string, error) { return fakeEngram, nil }
			t.Cleanup(func() { LookPathFunc, EngramFallbackPaths = origLook, origFallback })

			mainPath := filepath.Join(dir, ".claude.json")
			if err := os.WriteFile(mainPath, []byte(tt.mainJSON), 0o600); err != nil {
				t.Fatal(err)
			}
			sharedPath := filepath.Join(dir, "profile", "shared-mcp.json")
			res, err := GenerateSharedMCP(sharedPath, mainPath)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(res.AddedServers, tt.wantAdded) {
				t.Fatalf("AddedServers = %v, want %v", res.AddedServers, tt.wantAdded)
			}
			b, err := os.ReadFile(sharedPath)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				MCPServers map[string]map[string]any `json:"mcpServers"`
			}
			if err := json.Unmarshal(b, &doc); err != nil {
				t.Fatal(err)
			}
			pw := doc.MCPServers[ServerPlaywright]
			args, _ := json.Marshal(pw["args"])
			isPCS := pw["command"] == "npx" && string(args) == `["-y","`+PlaywrightMCPPackage+`","--output-dir","`+PlaywrightOutputDir+`"]`
			if isPCS != tt.wantPCSServer {
				t.Fatalf("playwright server = %v (pcs server %v, want %v)", pw, isPCS, tt.wantPCSServer)
			}
			if drift, err := SharedMCPDrift(sharedPath, mainPath); err != nil || len(drift) != 0 {
				t.Fatalf("SharedMCPDrift() = %v, %v; want none", drift, err)
			}
		})
	}
	if PlaywrightMCPPackage != "@playwright/mcp@0.0.83" || PlaywrightOutputDir != ".pcs/playwright" {
		t.Fatalf("playwright constants = %q, %q", PlaywrightMCPPackage, PlaywrightOutputDir)
	}
}
