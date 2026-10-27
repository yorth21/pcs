package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yorth21/pcs/internal/mcp"
)

// rewriteStateAsLegacy drops keys from the profile state file, simulating a
// state written by an older pcs (before schema_version, with omitempty lists).
func rewriteStateAsLegacy(t *testing.T, dir string, keys ...string) {
	t.Helper()
	statePath := filepath.Join(dir, StateFileName)
	b, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "schema_version")
	for _, k := range keys {
		delete(raw, k)
	}
	if err := writeJSONFile(statePath, raw); err != nil {
		t.Fatal(err)
	}
}

func profileOpts(home string) Options {
	return Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin")}
}

func TestLegacyStateWithUnknownNoStatusLineKeepsTheProfileStatusLineState(t *testing.T) {
	tests := []struct {
		name          string
		profileStatus map[string]any
		wantStatus    bool
		wantNoStatus  bool
	}{
		{name: "profile without statusLine stays without one", profileStatus: nil, wantStatus: false, wantNoStatus: true},
		{name: "profile with its own statusLine keeps it", profileStatus: map[string]any{"type": "command", "command": "echo mine"}, wantStatus: true, wantNoStatus: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateEnv(t)
			seedMainStatusLine(t, home)
			profileDir := filepath.Join(home, ".claude-pcs")
			install := profileOpts(home)
			install.NoStatusLine = true
			if _, err := Install(install); err != nil {
				t.Fatalf("Install(--no-statusline) error: %v", err)
			}
			if tt.profileStatus != nil {
				writeSettings(t, profileDir, map[string]any{"statusLine": tt.profileStatus})
			}
			// An older pcs did not persist no_statusline.
			rewriteStateAsLegacy(t, profileDir, "no_statusline")

			if _, err := Sync(profileOpts(home)); err != nil {
				t.Fatalf("Sync() error: %v", err)
			}
			got, has := readSettings(t, profileDir)["statusLine"]
			if has != tt.wantStatus {
				t.Fatalf("statusLine present after legacy sync = %v (%v), want %v", has, got, tt.wantStatus)
			}
			if has && !reflect.DeepEqual(got, tt.profileStatus) {
				t.Errorf("legacy sync replaced the profile statusLine: got %v, want %v", got, tt.profileStatus)
			}
			s := mustReadState(t, profileDir)
			if s.NoStatusLine != tt.wantNoStatus {
				t.Errorf("migrated no_statusline = %v, want %v", s.NoStatusLine, tt.wantNoStatus)
			}
			if s.SchemaVersion != StateSchemaVersion {
				t.Errorf("migrated schema_version = %d, want %d", s.SchemaVersion, StateSchemaVersion)
			}
		})
	}
}

func TestSyncDoesNotBackUpFilesPcsCreated(t *testing.T) {
	home := isolateEnv(t)
	seedMainStatusLine(t, home)
	profileDir := filepath.Join(home, ".claude-pcs")
	if _, err := Install(profileOpts(home)); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	s := mustReadState(t, profileDir)
	if !s.CreatedClaudeMD || !s.CreatedSettingsJSON {
		t.Fatalf("state must record that pcs created CLAUDE.md and settings.json: created_claude_md=%v created_settings_json=%v", s.CreatedClaudeMD, s.CreatedSettingsJSON)
	}

	withMCP := profileOpts(home)
	withMCP.WithMCP = true
	withMCP.FlagOverrides = map[string]bool{"with-mcp": true}
	if _, err := Sync(withMCP); err != nil {
		t.Fatalf("Sync(--with-mcp) error: %v", err)
	}
	if _, err := Sync(profileOpts(home)); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}
	for _, name := range []string{"CLAUDE.md", "settings.json", ".claude.json"} {
		bak := filepath.Join(profileDir, name+mcp.PrePCSBackupSuffix)
		if _, err := os.Stat(bak); err == nil {
			got, _ := os.ReadFile(bak)
			t.Errorf("sync wrote %s although pcs created %s:\n%s", bak, name, got)
		}
	}
}

func TestLegacyStateDoesNotBackUpAPcsOnlyCLAUDEMD(t *testing.T) {
	home := isolateEnv(t)
	profileDir := filepath.Join(home, ".claude-pcs")
	if _, err := Install(profileOpts(home)); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	rewriteStateAsLegacy(t, profileDir, "created_claude_md")
	if _, err := Sync(profileOpts(home)); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(profileDir, "CLAUDE.md"+mcp.PrePCSBackupSuffix)); err == nil {
		t.Errorf("sync backed up a CLAUDE.md holding only pcs blocks")
	}
}

func TestPreExistingCLAUDEMDIsStillBackedUp(t *testing.T) {
	home := isolateEnv(t)
	profileDir := filepath.Join(home, ".claude-pcs")
	claudeMD := filepath.Join(profileDir, "CLAUDE.md")
	if err := writeFileCreatingDirs(claudeMD, []byte("# my profile notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(profileOpts(home)); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if _, err := Sync(profileOpts(home)); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}
	got, err := os.ReadFile(claudeMD + mcp.PrePCSBackupSuffix)
	if err != nil || string(got) != "# my profile notes\n" {
		t.Fatalf("pre-pcs backup = %q (%v), want the user's original content", got, err)
	}
	if mustReadState(t, profileDir).CreatedClaudeMD {
		t.Errorf("state claims pcs created a pre-existing CLAUDE.md")
	}
}

func TestProfileUninstallRemovesSettingsJSONPcsCreated(t *testing.T) {
	home := isolateEnv(t)
	seedMainStatusLine(t, home)
	profileDir := filepath.Join(home, ".claude-pcs")
	if _, err := Install(profileOpts(home)); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if _, err := Uninstall(profileOpts(home)); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(profileDir, "settings.json")); err == nil {
		t.Errorf("uninstall left the settings.json pcs created: %s", b)
	}
}

func TestProfileUninstallPrunesContainersPcsCreated(t *testing.T) {
	home := isolateEnv(t)
	seedMainStatusLine(t, home)
	profileDir := filepath.Join(home, ".claude-pcs")
	writeSettings(t, profileDir, map[string]any{"theme": "dark"})
	install := profileOpts(home)
	install.WithMCP = true
	install.FlagOverrides = map[string]bool{"with-mcp": true}
	if _, err := Install(install); err != nil {
		t.Fatalf("Install(--with-mcp) error: %v", err)
	}
	uninstall := profileOpts(home)
	uninstall.RemoveMCP = true
	if _, err := Uninstall(uninstall); err != nil {
		t.Fatalf("Uninstall(--remove-mcp) error: %v", err)
	}
	if got, want := readSettings(t, profileDir), map[string]any{"theme": "dark"}; !reflect.DeepEqual(got, want) {
		t.Errorf("settings.json after uninstall = %v, want %v (no empty containers pcs created)", got, want)
	}
}

func TestProfileUninstallKeepsUserContainers(t *testing.T) {
	home := isolateEnv(t)
	profileDir := filepath.Join(home, ".claude-pcs")
	user := map[string]any{"permissions": map[string]any{"allow": []any{}}, "enabledPlugins": map[string]any{}}
	writeSettings(t, profileDir, user)
	install := profileOpts(home)
	install.WithMCP = true
	install.FlagOverrides = map[string]bool{"with-mcp": true}
	if _, err := Install(install); err != nil {
		t.Fatalf("Install(--with-mcp) error: %v", err)
	}
	uninstall := profileOpts(home)
	uninstall.RemoveMCP = true
	if _, err := Uninstall(uninstall); err != nil {
		t.Fatalf("Uninstall(--remove-mcp) error: %v", err)
	}
	if got := readSettings(t, profileDir); !reflect.DeepEqual(got, user) {
		t.Errorf("settings.json after uninstall = %v, want the user's containers %v", got, user)
	}
}

func TestSyncMigratesLegacyStateRecordsConservatively(t *testing.T) {
	t.Run("no default entries present: an empty record is provable", func(t *testing.T) {
		home := isolateEnv(t)
		profileDir := filepath.Join(home, ".claude-pcs")
		if _, err := Install(profileOpts(home)); err != nil {
			t.Fatalf("Install() error: %v", err)
		}
		rewriteStateAsLegacy(t, profileDir, "added_mcp_servers", "added_permissions", "added_plugins")
		if mustReadState(t, profileDir).hasMCPDeltaRecord() {
			t.Fatalf("legacy state unexpectedly carries a record")
		}
		if _, err := Sync(profileOpts(home)); err != nil {
			t.Fatalf("Sync() error: %v", err)
		}
		s := mustReadState(t, profileDir)
		if s.SchemaVersion != StateSchemaVersion {
			t.Errorf("schema_version = %d, want %d", s.SchemaVersion, StateSchemaVersion)
		}
		if !s.hasMCPDeltaRecord() || len(s.AddedMCPServers)+len(s.AddedPermissions)+len(s.AddedPlugins) != 0 {
			t.Errorf("migrated record = %v %v %v, want three empty lists", s.AddedMCPServers, s.AddedPermissions, s.AddedPlugins)
		}
	})
	t.Run("default entries of unknown origin stay unrecorded", func(t *testing.T) {
		home := isolateEnv(t)
		profileDir := filepath.Join(home, ".claude-pcs")
		if _, err := Install(profileOpts(home)); err != nil {
			t.Fatalf("Install() error: %v", err)
		}
		if err := writeFileCreatingDirs(filepath.Join(profileDir, ".claude.json"), []byte(`{"mcpServers": {"engram": {"command": "engram"}}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		writeSettings(t, profileDir, map[string]any{"enabledPlugins": map[string]any{"engram@engram": true}})
		rewriteStateAsLegacy(t, profileDir, "added_mcp_servers", "added_permissions", "added_plugins")
		if _, err := Sync(profileOpts(home)); err != nil {
			t.Fatalf("Sync() error: %v", err)
		}
		s := mustReadState(t, profileDir)
		if s.AddedMCPServers != nil || s.AddedPlugins != nil {
			t.Errorf("migration invented a record for entries of unknown origin: servers=%v plugins=%v", s.AddedMCPServers, s.AddedPlugins)
		}
		if s.AddedPermissions == nil || len(s.AddedPermissions) != 0 {
			t.Errorf("permissions with no default entry present must migrate to [], got %v", s.AddedPermissions)
		}
	})
}
