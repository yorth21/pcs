package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/yorth21/pcs/internal/flow"
	"github.com/yorth21/pcs/internal/mcp"
	"github.com/yorth21/pcs/internal/registry"
)

func TestInstallScopesAndLifecycle(t *testing.T) {
	tests := []struct {
		name  string
		scope string
	}{
		{name: "isolated profile scope", scope: ScopeProfile},
		{name: "global overlay scope", scope: ScopeGlobal},
		{name: "workspace overlay scope", scope: ScopeWorkspace},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeHome := isolateEnv(t)
			var targetDir string
			switch tt.scope {
			case ScopeProfile:
				targetDir = filepath.Join(fakeHome, ".claude-pcs")
			case ScopeGlobal:
				targetDir = filepath.Join(fakeHome, ".claude")
			case ScopeWorkspace:
				targetDir = filepath.Join(fakeHome, "my-frontend-app", ".claude")
			}

			opts := Options{
				Scope:          tt.scope,
				TargetDir:      targetDir,
				HomeDir:        fakeHome,
				Adapter:        "react-shadcn",
				Persona:        "gentleman",
				WriteMCPConfig: true,
				WriteHook:      true,
			}

			// 1. Dry-run must not create targetDir or mutate disk.
			dryOpts := opts
			dryOpts.DryRun = true
			dryReport, err := Install(dryOpts)
			if err != nil {
				t.Fatalf("Install(DryRun) error: %v", err)
			}
			if !dryReport.DryRun || len(dryReport.Actions) == 0 {
				t.Fatalf("expected non-empty dry-run actions")
			}
			if _, statErr := os.Stat(filepath.Join(targetDir, StateFileName)); statErr == nil {
				t.Fatalf("expected dry-run not to write %s", StateFileName)
			}

			// 2. Real Install.
			rep, err := Install(opts)
			if err != nil {
				t.Fatalf("Install() error: %v", err)
			}
			if rep.Scope != tt.scope {
				t.Errorf("rep.Scope = %q, want %q", rep.Scope, tt.scope)
			}
			// Every stack-agnostic brain skill is installed next to the adapter.
			for _, skill := range []string{"pcs-core", "pcs-saas", "pcs-adapter-react-shadcn"} {
				if _, statErr := os.Stat(filepath.Join(targetDir, "skills", skill, "SKILL.md")); statErr != nil {
					t.Errorf("skill %s not installed: %v", skill, statErr)
				}
			}

			// 3. Status check must report Installed = true.
			status, err := Status(opts)
			if err != nil {
				t.Fatalf("Status() error: %v", err)
			}
			if !status.Installed {
				t.Fatalf("expected Status().Installed = true, got false: %+v", status)
			}
			for k, ok := range status.MCPServers {
				if !ok {
					t.Errorf("expected MCP server %q to be configured", k)
				}
			}

			if tt.scope == ScopeProfile {
				for _, ext := range []string{"", ".cmd", ".ps1"} {
					launcher := filepath.Join(targetDir, "bin", "claude-pcs"+ext)
					if _, err := os.Stat(launcher); err != nil {
						t.Errorf("expected profile launcher %s to exist: %v", launcher, err)
					}
				}
			}
			if tt.scope == ScopeWorkspace {
				reg := filepath.Join(filepath.Dir(targetDir), ".atl", registry.RegistryFileName)
				if _, err := os.Stat(reg); err != nil {
					t.Errorf("expected workspace skill registry %s to exist: %v", reg, err)
				}
			}

			// 4. Sync should succeed using saved state.
			syncRep, err := Sync(Options{Scope: tt.scope, TargetDir: targetDir, HomeDir: fakeHome})
			if err != nil {
				t.Fatalf("Sync() error: %v", err)
			}
			if syncRep.Operation != "sync" {
				t.Errorf("expected Operation=sync, got %q", syncRep.Operation)
			}

			// 5. Uninstall should cleanly remove pcs artifacts.
			if _, err := Uninstall(opts); err != nil {
				t.Fatalf("Uninstall() error: %v", err)
			}
			statusAfter, err := Status(opts)
			if err != nil {
				t.Fatalf("Status() after uninstall error: %v", err)
			}
			if statusAfter.Installed {
				t.Errorf("expected Status().Installed = false after Uninstall")
			}
			if tt.scope == ScopeProfile {
				for _, ext := range []string{"", ".cmd", ".ps1"} {
					launcher := filepath.Join(targetDir, "bin", "claude-pcs"+ext)
					if _, err := os.Stat(launcher); !os.IsNotExist(err) {
						t.Errorf("expected profile launcher %s to be removed on Uninstall", launcher)
					}
				}
			}
			if tt.scope == ScopeWorkspace {
				reg := filepath.Join(filepath.Dir(targetDir), ".atl", registry.RegistryFileName)
				if _, err := os.Stat(reg); !os.IsNotExist(err) {
					t.Errorf("expected workspace skill registry %s to be removed on Uninstall", reg)
				}
			}
		})
	}
}

func TestDefaultInstallDoesNotTouchClaudeJSONOrSettingsJSON(t *testing.T) {
	fakeHome := isolateEnv(t)
	globalDir := filepath.Join(fakeHome, ".claude")

	// Default install options: MCPGuidance=nil (defaults to true), WriteMCPConfig=false, WriteHook=false.
	opts := Options{
		Scope:     ScopeGlobal,
		TargetDir: globalDir,
		HomeDir:   fakeHome,
	}

	if _, err := Install(opts); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	st, err := Status(opts)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if !st.Installed {
		t.Fatalf("expected Status().Installed = true")
	}

	// All 5 CLAUDE.md markers (including codegraph-guidance and engram-protocol) must be present by default.
	for _, m := range []string{
		"pcs:codegraph-guidance",
		"pcs:persona",
		"pcs:engram-protocol",
		"pcs:orchestrator",
		"pcs:ux-routing",
	} {
		if !st.Markers[m] {
			t.Errorf("expected marker %q to be present in default install", m)
		}
	}

	// Neither .claude.json nor settings.json may be created when --write-mcp-config and --write-hook are false.
	if _, err := os.Stat(st.ClaudeJSONPath); !os.IsNotExist(err) {
		t.Errorf("expected %s not to exist on default install, got err=%v", st.ClaudeJSONPath, err)
	}
	if _, err := os.Stat(st.SettingsJSONPath); !os.IsNotExist(err) {
		t.Errorf("expected %s not to exist on default install, got err=%v", st.SettingsJSONPath, err)
	}
}

func TestGranularFlagsWriteMCPConfigWriteHookAndMCPGuidanceFalse(t *testing.T) {
	t.Run("--mcp-guidance=false omits codegraph-guidance and engram-protocol", func(t *testing.T) {
		fakeHome := isolateEnv(t)
		globalDir := filepath.Join(fakeHome, ".claude")
		opts := Options{
			Scope:       ScopeGlobal,
			TargetDir:   globalDir,
			HomeDir:     fakeHome,
			MCPGuidance: BoolPtr(false),
		}
		if _, err := Install(opts); err != nil {
			t.Fatalf("Install() error: %v", err)
		}
		st, err := Status(opts)
		if err != nil {
			t.Fatal(err)
		}
		if st.Markers["pcs:codegraph-guidance"] || st.Markers["pcs:engram-protocol"] {
			t.Errorf("expected MCP guidance markers to be absent when MCPGuidance=false, got %+v", st.Markers)
		}
		if !st.Markers["pcs:persona"] || !st.Markers["pcs:orchestrator"] || !st.Markers["pcs:ux-routing"] {
			t.Errorf("expected core markers to be present when MCPGuidance=false, got %+v", st.Markers)
		}
	})

	t.Run("--write-mcp-config without --write-hook updates .claude.json and permissions without adding hook", func(t *testing.T) {
		fakeHome := isolateEnv(t)
		globalDir := filepath.Join(fakeHome, ".claude")
		opts := Options{
			Scope:          ScopeGlobal,
			TargetDir:      globalDir,
			HomeDir:        fakeHome,
			WriteMCPConfig: true,
			WriteHook:      false,
		}
		if _, err := Install(opts); err != nil {
			t.Fatalf("Install() error: %v", err)
		}
		st, err := Status(opts)
		if err != nil {
			t.Fatal(err)
		}
		for _, srv := range []string{"engram", "context7", "codegraph"} {
			if !st.MCPServers[srv] {
				t.Errorf("expected MCP server %q in .claude.json", srv)
			}
		}
		settingsBytes, err := os.ReadFile(st.SettingsJSONPath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(settingsBytes), "pcs skill-registry refresh") {
			t.Errorf("did not expect hook in settings.json when WriteHook=false:\n%s", string(settingsBytes))
		}
	})

	t.Run("--write-hook without --write-mcp-config adds hook without touching .claude.json", func(t *testing.T) {
		fakeHome := isolateEnv(t)
		globalDir := filepath.Join(fakeHome, ".claude")
		opts := Options{
			Scope:          ScopeGlobal,
			TargetDir:      globalDir,
			HomeDir:        fakeHome,
			WriteMCPConfig: false,
			WriteHook:      true,
		}
		if _, err := Install(opts); err != nil {
			t.Fatalf("Install() error: %v", err)
		}
		st, err := Status(opts)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(st.ClaudeJSONPath); !os.IsNotExist(err) {
			t.Errorf("did not expect .claude.json to be created when WriteMCPConfig=false")
		}
		settingsBytes, err := os.ReadFile(st.SettingsJSONPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(settingsBytes), "pcs skill-registry refresh") {
			t.Errorf("expected hook in settings.json when WriteHook=true:\n%s", string(settingsBytes))
		}
	})
}

func TestSyncHonorsExplicitFlagOverridesAndRemovesDisabledMarkers(t *testing.T) {
	fakeHome := isolateEnv(t)
	globalDir := filepath.Join(fakeHome, ".claude")

	// 1. Initial install with MCPGuidance=true (default).
	installOpts := Options{
		Scope:     ScopeGlobal,
		TargetDir: globalDir,
		HomeDir:   fakeHome,
	}
	if _, err := Install(installOpts); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	st1, err := Status(installOpts)
	if err != nil {
		t.Fatal(err)
	}
	if !st1.Markers["pcs:codegraph-guidance"] || !st1.Markers["pcs:engram-protocol"] {
		t.Fatalf("expected MCP guidance markers after initial install")
	}

	// 2. Run Sync with explicit --mcp-guidance=false override.
	syncOpts := Options{
		Scope:         ScopeGlobal,
		TargetDir:     globalDir,
		HomeDir:       fakeHome,
		MCPGuidance:   BoolPtr(false),
		FlagOverrides: map[string]bool{"mcp-guidance": true},
	}
	if _, err := Sync(syncOpts); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	st2, err := Status(installOpts)
	if err != nil {
		t.Fatal(err)
	}
	if st2.Markers["pcs:codegraph-guidance"] || st2.Markers["pcs:engram-protocol"] {
		t.Errorf("expected pcs:codegraph-guidance and pcs:engram-protocol to be actively removed by Sync(--mcp-guidance=false), got %+v", st2.Markers)
	}
	if st2.State == nil || st2.State.MCPGuidance {
		t.Errorf("expected .pcs-state.json MCPGuidance to be updated to false")
	}
}

func TestUninstallRemovesAllPCSMarkersByteForByteAndPreservesPreExistingEngram(t *testing.T) {
	fakeHome := isolateEnv(t)
	globalDir := filepath.Join(fakeHome, ".claude")
	claudeJSON := filepath.Join(fakeHome, ".claude.json")

	// Seed pre-existing user CLAUDE.md with CRLF and triple newlines, custom agent, custom skill, and pre-existing engram.
	claudeMD := filepath.Join(globalDir, "CLAUDE.md")
	initialMD := "# Custom User Instructions\r\n\r\n\r\nAlways keep builds deterministic.\r\n"
	if err := writeFileCreatingDirs(claudeMD, []byte(initialMD), 0o644); err != nil {
		t.Fatal(err)
	}
	initialClaudeJSON := `{
  "mcpServers": {
    "engram": {
      "args": [
        "mcp",
        "--tools=all"
      ],
      "command": "/custom/engram"
    }
  }
}
`
	if err := writeFileCreatingDirs(claudeJSON, []byte(initialClaudeJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	customAgent := filepath.Join(globalDir, "agents", "custom-reviewer.md")
	if err := writeFileCreatingDirs(customAgent, []byte("# Custom Reviewer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	customSkill := filepath.Join(globalDir, "skills", "go-testing", "SKILL.md")
	if err := writeFileCreatingDirs(customSkill, []byte("# Go Testing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Scope:          ScopeGlobal,
		TargetDir:      globalDir,
		HomeDir:        fakeHome,
		Adapter:        "react-shadcn",
		Persona:        "gentleman",
		WriteMCPConfig: true,
		WriteHook:      true,
	}

	if _, err := Install(opts); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	// Running Sync twice must not overwrite CLAUDE.md.pre-pcs.bak!
	for range 2 {
		if _, err := Sync(Options{Scope: ScopeGlobal, TargetDir: globalDir, HomeDir: fakeHome}); err != nil {
			t.Fatalf("Sync() error: %v", err)
		}
	}
	bakMD, err := os.ReadFile(claudeMD + mcp.PrePCSBackupSuffix)
	if err != nil {
		t.Fatalf("read %s%s: %v", claudeMD, mcp.PrePCSBackupSuffix, err)
	}
	if string(bakMD) != initialMD {
		t.Errorf("CLAUDE.md.pre-pcs.bak was overwritten by Sync:\ngot:  %q\nwant: %q", string(bakMD), initialMD)
	}

	if _, err := Uninstall(opts); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}

	// Verify CLAUDE.md is byte-for-byte identical to initialMD!
	afterMDBytes, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterMDBytes) != initialMD {
		t.Errorf("CLAUDE.md after Uninstall mismatch:\ngot:  %q\nwant: %q", string(afterMDBytes), initialMD)
	}

	// Verify pre-existing engram survived in .claude.json while context7 and codegraph were removed.
	stAfter, err := Status(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !stAfter.MCPServers["engram"] {
		t.Errorf("expected pre-existing engram server to survive Uninstall")
	}
	if stAfter.MCPServers["context7"] || stAfter.MCPServers["codegraph"] {
		t.Errorf("expected pcs-added context7 and codegraph to be removed on Uninstall, got %+v", stAfter.MCPServers)
	}

	// Verify custom agent and go-testing skill survived.
	if _, err := os.Stat(customAgent); err != nil {
		t.Errorf("expected pre-existing agent %s to be preserved: %v", customAgent, err)
	}
	if _, err := os.Stat(customSkill); err != nil {
		t.Errorf("expected pre-existing skill %s to be preserved: %v", customSkill, err)
	}
}

func TestCustomProfileDirWithApostropheIsUsedInInstalledAssets(t *testing.T) {
	fakeHome := isolateEnv(t)
	customProfile := filepath.Join(fakeHome, "O'Reilly Profile", ".claude-custom")

	opts := Options{
		Scope:      ScopeProfile,
		ProfileDir: customProfile,
		HomeDir:    fakeHome,
	}
	rep, err := Install(opts)
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if rep.TargetDir != customProfile {
		t.Fatalf("rep.TargetDir = %q, want %q", rep.TargetDir, customProfile)
	}

	// Verify installed CLAUDE.md and native agents reference the custom profile skills path instead of ~/.claude-pcs/skills.
	claudeMDBytes, err := os.ReadFile(filepath.Join(customProfile, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	wantSkillsSlash := filepath.ToSlash(filepath.Join(customProfile, "skills"))
	if !strings.Contains(string(claudeMDBytes), wantSkillsSlash) {
		t.Errorf("expected installed CLAUDE.md to contain custom profile skills path %q", wantSkillsSlash)
	}

	agentBytes, err := os.ReadFile(filepath.Join(customProfile, "agents", "pcs-architect.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agentBytes), wantSkillsSlash) {
		t.Errorf("expected installed pcs-architect.md to contain custom profile skills path %q", wantSkillsSlash)
	}
}

func TestDefaultGlobalScopeAndEnvironmentFallbacks(t *testing.T) {
	tests := []struct {
		name               string
		envInstallScope    string
		envClaudeConfigDir string
		wantScope          string
		wantTargetSuffix   string
		wantClaudeJSONIn   string // "parent" or "target"
	}{
		{
			name:             "defaults to global scope and <HomeDir>/.claude when no flags or env vars are set",
			wantScope:        ScopeGlobal,
			wantTargetSuffix: ".claude",
			wantClaudeJSONIn: "parent",
		},
		{
			name:               "uses CLAUDE_CONFIG_DIR when scope is global and target-dir is empty",
			envClaudeConfigDir: "custom-claude-profile",
			wantScope:          ScopeGlobal,
			wantTargetSuffix:   "custom-claude-profile",
			wantClaudeJSONIn:   "target",
		},
		{
			name:             "respects PCS_INSTALL_SCOPE when Scope option is empty",
			envInstallScope:  ScopeProfile,
			wantScope:        ScopeProfile,
			wantTargetSuffix: ".claude-pcs",
			wantClaudeJSONIn: "target",
		},
		{
			name:               "ignores CLAUDE_CONFIG_DIR when scope is profile and profile-dir/target-dir are empty",
			envInstallScope:    ScopeProfile,
			envClaudeConfigDir: "env-profile-dir",
			wantScope:          ScopeProfile,
			wantTargetSuffix:   ".claude-pcs",
			wantClaudeJSONIn:   "target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeHome := isolateEnv(t)
			t.Setenv("PCS_INSTALL_SCOPE", tt.envInstallScope)

			var customCfg string
			if tt.envClaudeConfigDir != "" {
				customCfg = filepath.Join(fakeHome, tt.envClaudeConfigDir)
			}
			t.Setenv("CLAUDE_CONFIG_DIR", customCfg)

			opts := Options{
				HomeDir: fakeHome,
				WithMCP: true,
			}
			rep, err := Install(opts)
			if err != nil {
				t.Fatalf("Install() error: %v", err)
			}
			if rep.Scope != tt.wantScope {
				t.Errorf("rep.Scope = %q, want %q", rep.Scope, tt.wantScope)
			}
			if !strings.HasSuffix(rep.TargetDir, tt.wantTargetSuffix) {
				t.Errorf("rep.TargetDir = %q, want suffix %q", rep.TargetDir, tt.wantTargetSuffix)
			}

			status, err := Status(opts)
			if err != nil {
				t.Fatalf("Status() error: %v", err)
			}
			if !status.Installed {
				t.Errorf("expected Status().Installed = true")
			}

			var expectedClaudeJSON string
			if tt.wantClaudeJSONIn == "parent" {
				expectedClaudeJSON = filepath.Join(fakeHome, ".claude.json")
			} else {
				expectedClaudeJSON = filepath.Join(rep.TargetDir, ".claude.json")
			}
			if status.ClaudeJSONPath != expectedClaudeJSON {
				t.Errorf("status.ClaudeJSONPath = %q, want %q", status.ClaudeJSONPath, expectedClaudeJSON)
			}
		})
	}
}

func TestProfileScopeLauncherIsolationAndSharedMCP(t *testing.T) {
	fakeHome := isolateEnv(t)
	mainClaudeJSONPath := filepath.Join(fakeHome, ".claude.json")
	mainSettingsDir := filepath.Join(fakeHome, ".claude")
	if err := os.MkdirAll(mainSettingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mainSettingsPath := filepath.Join(mainSettingsDir, "settings.json")

	// Seed main profile with a custom MCP server and statusLine
	mainClaudeJSONContent := `{"mcpServers": {"main-server": {"command": "custom-mcp", "args": ["serve"]}}}` + "\n"
	if err := os.WriteFile(mainClaudeJSONPath, []byte(mainClaudeJSONContent), 0o600); err != nil {
		t.Fatal(err)
	}
	mainSettingsContent := `{"statusLine": {"command": "echo profile:gentle"}}` + "\n"
	if err := os.WriteFile(mainSettingsPath, []byte(mainSettingsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	profileDir := filepath.Join(fakeHome, "custom profile with spaces", ".claude-pcs")
	launcherDir := filepath.Join(fakeHome, "bin with spaces")

	opts := Options{
		Scope:        ScopeProfile,
		TargetDir:    profileDir,
		HomeDir:      fakeHome,
		LauncherDir:  launcherDir,
		LauncherArgs: "--dangerously-skip-permissions",
	}

	rep, err := Install(opts)
	if err != nil {
		t.Fatalf("Install() profile error: %v", err)
	}

	// 1. Verify shared-mcp.json was created and reused main profile's MCP servers
	sharedMCPPath := filepath.Join(profileDir, "shared-mcp.json")
	sharedData, err := os.ReadFile(sharedMCPPath)
	if err != nil {
		t.Fatalf("failed to read shared-mcp.json: %v", err)
	}
	if !strings.Contains(string(sharedData), "main-server") {
		t.Errorf("expected shared-mcp.json to contain main-server from main profile:\n%s", string(sharedData))
	}
	// The main profile has no playwright server, so pcs adds its own and records it.
	if !strings.Contains(string(sharedData), mcp.PlaywrightMCPPackage) || !strings.Contains(string(sharedData), mcp.PlaywrightOutputDir) {
		t.Errorf("expected shared-mcp.json to contain the pcs playwright server:\n%s", string(sharedData))
	}
	var profState State
	if b, err := os.ReadFile(filepath.Join(profileDir, StateFileName)); err != nil || json.Unmarshal(b, &profState) != nil {
		t.Fatalf("read profile state: %v", err)
	}
	if !slices.Contains(profState.SharedMCPAddedServers, mcp.ServerPlaywright) {
		t.Errorf("SharedMCPAddedServers = %v, want playwright recorded", profState.SharedMCPAddedServers)
	}

	// 2. Verify main profile's .claude.json was NOT mutated
	afterMainJSON, err := os.ReadFile(mainClaudeJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterMainJSON) != mainClaudeJSONContent {
		t.Errorf("main profile .claude.json was modified!\ngot:  %s\nwant: %s", string(afterMainJSON), mainClaudeJSONContent)
	}

	// 3. Verify the main statusLine command became the base of the pcs
	// statusline-wrap command in the profile's settings.json.
	profileSettingsPath := filepath.Join(profileDir, "settings.json")
	profSettingsData, err := os.ReadFile(profileSettingsPath)
	if err != nil {
		t.Fatalf("failed to read profile settings.json: %v", err)
	}
	if !strings.Contains(string(profSettingsData), "statusline-wrap") || profState.FlowStatusLineBaseCommand != "echo profile:gentle" {
		t.Errorf("expected the wrapped statusLine with base %q, got base %q:\n%s", "echo profile:gentle", profState.FlowStatusLineBaseCommand, string(profSettingsData))
	}

	// 4. Verify launchers were created in launcherDir. Their runtime behavior
	// (environment isolation, argument order, quoting) is covered by the
	// executable launcher tests in launcher_test.go.
	shPath := filepath.Join(launcherDir, "claude-pcs")
	for _, p := range []string{shPath, filepath.Join(launcherDir, "claude-pcs.cmd"), filepath.Join(launcherDir, "claude-pcs.ps1")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing launcher %s: %v", p, err)
		}
	}

	// 5. Verify NextSteps guidance
	if len(rep.NextSteps) == 0 {
		t.Errorf("expected NextSteps guidance in report for profile install")
	}

	// 6. Test Sync regenerates shared-mcp.json and preserves settings
	syncRep, err := Sync(opts)
	if err != nil {
		t.Fatalf("Sync() error: %v", err)
	}
	if syncRep == nil {
		t.Fatal("expected non-nil sync report")
	}

	// 7. Test Uninstall cleans up launchers, shared-mcp.json, and statusLine
	unRep, err := Uninstall(opts)
	if err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if unRep == nil {
		t.Fatal("expected non-nil uninstall report")
	}

	if _, err := os.Stat(sharedMCPPath); !os.IsNotExist(err) {
		t.Errorf("expected shared-mcp.json to be deleted after uninstall")
	}
	if _, err := os.Stat(shPath); !os.IsNotExist(err) {
		t.Errorf("expected launcher %s to be deleted after uninstall", shPath)
	}

	// Verify main profile .claude.json is STILL byte-identical after uninstall
	finalMainJSON, err := os.ReadFile(mainClaudeJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(finalMainJSON) != mainClaudeJSONContent {
		t.Errorf("main profile .claude.json modified after uninstall!\ngot:  %s\nwant: %s", string(finalMainJSON), mainClaudeJSONContent)
	}
}

// testHookBinary is a stable (non-temporary) pcs path the flow hooks reference
// in tests, so installs never point settings.json at the test binary.
var testHookBinary = filepath.Join(string(filepath.Separator)+"opt", "pcs-test", "bin", "pcs")

func init() {
	if abs, err := filepath.Abs(testHookBinary); err == nil {
		testHookBinary = abs
	}
	ExecutableFunc = func() (string, error) { return testHookBinary, nil }
}

func TestProfileInstallWritesFlowHooksByDefault(t *testing.T) {
	home := isolateEnv(t)
	target := filepath.Join(home, ".claude-pcs")
	settings := filepath.Join(target, "settings.json")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := `{"hooks":{"SessionStart":[{"matcher":"","hooks":[{"type":"command","command":"herdr session-start"}]}]}}`
	if err := os.WriteFile(settings, []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}
	mainSnapshot := snapshotTree(t, filepath.Join(home, ".claude"))

	rep, err := Install(Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin"), NoStatusLine: true})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.FlowHooks || strings.Contains(strings.Join(rep.Warnings, "\n"), "flow hooks") {
		t.Fatalf("report flow_hooks=%v warnings=%v", rep.FlowHooks, rep.Warnings)
	}
	present, cmds, err := mcp.InspectFlowHooks(settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range mcp.FlowHookSpecs {
		if !present[spec.Event] {
			t.Fatalf("%s hook missing", spec.Event)
		}
	}
	wantCmd := mcp.FlowHookCommand(testHookBinary, "stop")
	if !strings.Contains(strings.Join(cmds, "\n"), wantCmd) {
		t.Fatalf("commands %v lack %s", cmds, wantCmd)
	}
	st, err := readState(filepath.Join(target, StateFileName))
	if err != nil || st.FlowHooks == nil || !*st.FlowHooks || !st.AddedFlowHooks || st.FlowHookBinary != testHookBinary {
		t.Fatalf("state = %+v, %v", st, err)
	}

	// Sync is idempotent and keeps exactly one copy of each hook.
	if _, err := Sync(Options{Scope: ScopeProfile, HomeDir: home, FlagOverrides: map[string]bool{"scope": true}}); err != nil {
		t.Fatal(err)
	}
	if _, cmds, _ := mcp.InspectFlowHooks(settings); len(cmds) != len(mcp.FlowHookSpecs) {
		t.Fatalf("after sync commands = %v", cmds)
	}

	// Sync re-renders hooks for a moved binary, replacing the old ones.
	moved, _ := filepath.Abs(filepath.Join(string(filepath.Separator)+"srv", "tools", "pcs-renamed"))
	ExecutableFunc = func() (string, error) { return moved, nil }
	t.Cleanup(func() { ExecutableFunc = func() (string, error) { return testHookBinary, nil } })
	if _, err := Sync(Options{Scope: ScopeProfile, HomeDir: home, FlagOverrides: map[string]bool{"scope": true}}); err != nil {
		t.Fatal(err)
	}
	_, cmds, _ = mcp.InspectFlowHooks(settings, moved)
	if len(cmds) != len(mcp.FlowHookSpecs) || !strings.Contains(cmds[0], "pcs-renamed") {
		t.Fatalf("re-rendered commands = %v", cmds)
	}

	if _, err := Uninstall(Options{Scope: ScopeProfile, HomeDir: home}); err != nil {
		t.Fatal(err)
	}
	root := readSettings(t, target)
	hooks, _ := root["hooks"].(map[string]any)
	if len(hooks) != 1 || hooks["SessionStart"] == nil {
		t.Fatalf("hooks after uninstall = %v (foreign hook must survive alone)", hooks)
	}
	if after := snapshotTree(t, filepath.Join(home, ".claude")); len(after) != len(mainSnapshot) {
		t.Fatal("main ~/.claude profile changed")
	}
}

func TestFlowHooksOptOutAndOtherScopes(t *testing.T) {
	t.Run("profile --no-hooks writes none and sync remembers it", func(t *testing.T) {
		home := isolateEnv(t)
		target := filepath.Join(home, ".claude-pcs")
		settings := filepath.Join(target, "settings.json")
		if _, err := Install(Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin"), NoStatusLine: true, FlowHooks: BoolPtr(false)}); err != nil {
			t.Fatal(err)
		}
		if present, _, _ := mcp.InspectFlowHooks(settings); present["Stop"] {
			t.Fatal("hooks written despite --no-hooks")
		}
		if _, err := Sync(Options{Scope: ScopeProfile, HomeDir: home, FlagOverrides: map[string]bool{"scope": true}}); err != nil {
			t.Fatal(err)
		}
		if present, _, _ := mcp.InspectFlowHooks(settings); present["Stop"] {
			t.Fatal("sync re-enabled hooks")
		}
		// Enabling later, then disabling with sync, removes them again.
		if _, err := Sync(Options{Scope: ScopeProfile, HomeDir: home, FlowHooks: BoolPtr(true), FlagOverrides: map[string]bool{"scope": true}}); err != nil {
			t.Fatal(err)
		}
		if present, _, _ := mcp.InspectFlowHooks(settings); !present["Stop"] {
			t.Fatal("sync --with-hooks did not write hooks")
		}
		if _, err := Sync(Options{Scope: ScopeProfile, HomeDir: home, FlowHooks: BoolPtr(false), FlagOverrides: map[string]bool{"scope": true}}); err != nil {
			t.Fatal(err)
		}
		if present, _, _ := mcp.InspectFlowHooks(settings); present["Stop"] {
			t.Fatal("sync --no-hooks did not remove hooks")
		}
	})
	t.Run("workspace default writes none, --with-hooks writes them", func(t *testing.T) {
		isolateEnv(t)
		ws := t.TempDir()
		settings := filepath.Join(ws, ".claude", "settings.json")
		if _, err := Install(Options{Scope: ScopeWorkspace, TargetDir: ws}); err != nil {
			t.Fatal(err)
		}
		if present, _, _ := mcp.InspectFlowHooks(settings); present["PreToolUse"] {
			t.Fatal("workspace install wrote hooks by default")
		}
		if _, err := Install(Options{Scope: ScopeWorkspace, TargetDir: ws, FlowHooks: BoolPtr(true)}); err != nil {
			t.Fatal(err)
		}
		if present, _, _ := mcp.InspectFlowHooks(settings); !present["PreToolUse"] {
			t.Fatal("--with-hooks did not write hooks")
		}
		if _, err := Uninstall(Options{Scope: ScopeWorkspace, TargetDir: ws}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(settings); !os.IsNotExist(err) {
			t.Fatalf("settings.json pcs created survives uninstall: %v", err)
		}
	})
	t.Run("temporary binary warns", func(t *testing.T) {
		home := isolateEnv(t)
		tmpBin := filepath.Join(os.TempDir(), "go-build123", "pcs.exe")
		ExecutableFunc = func() (string, error) { return tmpBin, nil }
		t.Cleanup(func() { ExecutableFunc = func() (string, error) { return testHookBinary, nil } })
		rep, err := Install(Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin"), NoStatusLine: true, DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(rep.Warnings) == 0 || !strings.Contains(rep.Warnings[0], "temporary directory") {
			t.Fatalf("warnings = %v", rep.Warnings)
		}
		if _, err := os.Stat(filepath.Join(home, ".claude-pcs", "settings.json")); !os.IsNotExist(err) {
			t.Fatal("dry run wrote settings.json")
		}
	})
}

func TestFlowStatusLineWrapLifecycle(t *testing.T) {
	home := isolateEnv(t)
	mainValue := map[string]any{"type": "command", "command": "echo main", "padding": float64(2)}
	writeSettings(t, filepath.Join(home, ".claude"), map[string]any{"statusLine": mainValue})
	mainSnapshot := snapshotTree(t, filepath.Join(home, ".claude"))
	target := filepath.Join(home, ".claude-pcs")
	statePath := filepath.Join(target, StateFileName)
	settings := filepath.Join(target, "settings.json")
	opts := Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin")}
	sync := func(choice *bool) {
		t.Helper()
		o := opts
		o.FlowStatusLine = choice
		o.FlagOverrides = map[string]bool{"scope": true}
		if _, err := Sync(o); err != nil {
			t.Fatal(err)
		}
	}
	wantWrapped := func() {
		t.Helper()
		got, _ := readSettings(t, target)["statusLine"].(map[string]any)
		if got["command"] != FlowStatusLineCommand(testHookBinary, statePath) || got["type"] != "command" || got["padding"] != float64(2) {
			t.Fatalf("statusLine = %v, want the pcs wrapper keeping padding", got)
		}
		if base := flow.ReadStatusLineBase(statePath); base != "echo main" {
			t.Fatalf("recorded base command = %q", base)
		}
	}

	if _, err := Install(opts); err != nil {
		t.Fatal(err)
	}
	wantWrapped()
	st := mustReadState(t, target)
	if st.FlowStatusLine == nil || !*st.FlowStatusLine || !st.AddedFlowStatusLine {
		t.Fatalf("state = %+v", st)
	}

	// Sync is idempotent.
	before, _ := os.ReadFile(settings)
	sync(nil)
	sync(nil)
	if after, _ := os.ReadFile(settings); string(after) != string(before) {
		t.Fatalf("sync changed settings.json:\n%s\n---\n%s", before, after)
	}
	wantWrapped()

	// Opting out restores the base exactly; opting in wraps it again.
	sync(BoolPtr(false))
	if got := readSettings(t, target)["statusLine"]; !reflect.DeepEqual(got, mainValue) {
		t.Fatalf("restored statusLine = %v, want %v", got, mainValue)
	}
	if st := mustReadState(t, target); st.AddedFlowStatusLine || st.FlowStatusLineBaseCommand != "" {
		t.Fatalf("state after opt-out = %+v", st)
	}
	sync(nil) // the opt-out is remembered
	if got := readSettings(t, target)["statusLine"]; !reflect.DeepEqual(got, mainValue) {
		t.Fatalf("sync re-wrapped despite the saved opt-out: %v", got)
	}
	sync(BoolPtr(true))
	wantWrapped()

	if _, err := Uninstall(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(settings); !os.IsNotExist(err) {
		t.Fatalf("settings.json pcs created survives uninstall: %s", readSettings(t, target))
	}
	if after := snapshotTree(t, filepath.Join(home, ".claude")); !reflect.DeepEqual(after, mainSnapshot) {
		t.Fatal("main ~/.claude profile changed")
	}
}

func TestFlowStatusLineRestoresProfileOwnStatusLine(t *testing.T) {
	home := isolateEnv(t)
	target := filepath.Join(home, ".claude-pcs")
	own := map[string]any{"command": "echo mine"}
	writeSettings(t, target, map[string]any{"statusLine": own, "theme": "dark"})
	opts := Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin"), FlowHooks: BoolPtr(false)}
	if _, err := Install(opts); err != nil {
		t.Fatal(err)
	}
	if got := mustReadState(t, target).FlowStatusLineBaseCommand; got != "echo mine" {
		t.Fatalf("base command = %q", got)
	}
	if _, err := Uninstall(opts); err != nil {
		t.Fatal(err)
	}
	if got, want := readSettings(t, target), map[string]any{"statusLine": own, "theme": "dark"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("settings after uninstall = %v, want %v", got, want)
	}
}

func TestFlowStatusLineDefaults(t *testing.T) {
	t.Run("--no-statusline disables it unless explicitly enabled", func(t *testing.T) {
		home := isolateEnv(t)
		target := filepath.Join(home, ".claude-pcs")
		opts := Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin"), NoStatusLine: true}
		if _, err := Install(opts); err != nil {
			t.Fatal(err)
		}
		if _, has := readSettings(t, target)["statusLine"]; has {
			t.Fatal("statusLine written despite --no-statusline")
		}
		opts.FlowStatusLine = BoolPtr(true)
		if _, err := Install(opts); err != nil {
			t.Fatal(err)
		}
		got, _ := readSettings(t, target)["statusLine"].(map[string]any)
		if !isFlowStatusLineValue(mustJSON(t, got)) || mustReadState(t, target).FlowStatusLineBaseCommand != "" {
			t.Fatalf("statusLine = %v, want the wrapper without a base", got)
		}
	})
	t.Run("workspace scope never wraps", func(t *testing.T) {
		isolateEnv(t)
		ws := t.TempDir()
		rep, err := Install(Options{Scope: ScopeWorkspace, TargetDir: ws, FlowStatusLine: BoolPtr(true)})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(rep.Warnings, "\n"), "--with-flow-statusline applies only") {
			t.Fatalf("warnings = %v", rep.Warnings)
		}
		if _, has := readSettings(t, filepath.Join(ws, ".claude"))["statusLine"]; has {
			t.Fatal("workspace statusLine written")
		}
	})
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
