package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
				Scope:     tt.scope,
				TargetDir: targetDir,
				HomeDir:   fakeHome,
				Adapter:   "react-shadcn",
				Persona:   "gentleman",
				WithMCP:   true,
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
				launcher := filepath.Join(targetDir, "bin", "claude-pcs")
				if _, err := os.Stat(launcher); err != nil {
					t.Errorf("expected profile launcher %s to exist: %v", launcher, err)
				}
			}
			if tt.scope == ScopeWorkspace {
				reg := filepath.Join(filepath.Dir(targetDir), ".atl", "skill-registry.md")
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
		})
	}
}

func TestUninstallPreservesGentleAIAndUserFiles(t *testing.T) {
	fakeHome := t.TempDir()
	globalDir := filepath.Join(fakeHome, ".claude")

	// Seed pre-existing gentle-ai CLAUDE.md, custom agent, and custom skill.
	claudeMD := filepath.Join(globalDir, "CLAUDE.md")
	initialMD := `<!-- gentle-ai:persona -->
Gentleman Persona
<!-- /gentle-ai:persona -->

<!-- gentle-ai:engram-protocol -->
Engram Protocol
<!-- /gentle-ai:engram-protocol -->
`
	if err := writeFileCreatingDirs(claudeMD, []byte(initialMD), 0o644); err != nil {
		t.Fatal(err)
	}
	customAgent := filepath.Join(globalDir, "agents", "sdd-explorer.md")
	if err := writeFileCreatingDirs(customAgent, []byte("# SDD Explorer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	customSkill := filepath.Join(globalDir, "skills", "go-testing", "SKILL.md")
	if err := writeFileCreatingDirs(customSkill, []byte("# Go Testing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Scope:     ScopeGlobal,
		TargetDir: globalDir,
		HomeDir:   fakeHome,
		Adapter:   "react-shadcn",
		Persona:   "gentleman",
		WithMCP:   true,
	}

	if _, err := Install(opts); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	if _, err := Uninstall(opts); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}

	// Verify gentle-ai markers survived in CLAUDE.md while pcs markers were removed.
	afterMDBytes, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatal(err)
	}
	afterMD := string(afterMDBytes)
	if !strings.Contains(afterMD, "<!-- gentle-ai:persona -->") {
		t.Error("expected gentle-ai:persona marker to be preserved after Uninstall")
	}
	if !strings.Contains(afterMD, "<!-- gentle-ai:engram-protocol -->") {
		t.Error("expected gentle-ai:engram-protocol marker to be preserved after Uninstall")
	}
	if strings.Contains(afterMD, "<!-- pcs:orchestrator -->") || strings.Contains(afterMD, "<!-- pcs:ux-routing -->") {
		t.Error("expected pcs markers to be removed after Uninstall")
	}

	// Verify custom sdd-explorer agent and go-testing skill survived.
	if _, err := os.Stat(customAgent); err != nil {
		t.Errorf("expected pre-existing agent %s to be preserved: %v", customAgent, err)
	}
	if _, err := os.Stat(customSkill); err != nil {
		t.Errorf("expected pre-existing skill %s to be preserved: %v", customSkill, err)
	}
}
