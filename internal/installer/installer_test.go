package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
			fakeHome := t.TempDir()
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
	fakeHome := t.TempDir()
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
		fakeHome := t.TempDir()
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
		fakeHome := t.TempDir()
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
		fakeHome := t.TempDir()
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
	fakeHome := t.TempDir()
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
	fakeHome := t.TempDir()
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

func TestCustomProfileDirAndPowerShellSingleQuoteEscaping(t *testing.T) {
	fakeHome := t.TempDir()
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

	// Verify PowerShell launcher uses single quotes with '' escaping for apostrophes.
	ps1Bytes, err := os.ReadFile(filepath.Join(customProfile, "bin", "claude-pcs.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	ps1Text := string(ps1Bytes)
	if !strings.Contains(ps1Text, "O''Reilly Profile") {
		t.Errorf("expected single quote in path to be escaped as '' in PowerShell launcher, got:\n%s", ps1Text)
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
			name:               "uses CLAUDE_CONFIG_DIR when scope is profile and profile-dir/target-dir are empty",
			envInstallScope:    ScopeProfile,
			envClaudeConfigDir: "env-profile-dir",
			wantScope:          ScopeProfile,
			wantTargetSuffix:   "env-profile-dir",
			wantClaudeJSONIn:   "target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeHome := t.TempDir()
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
