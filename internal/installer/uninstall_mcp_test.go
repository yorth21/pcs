package installer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// canonicalJSON renders v exactly as pcs writes JSON files, so byte equality
// after a merge/unmerge round trip proves the content is unchanged.
func canonicalJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

// seedPreExistingMCP writes a global profile where engram, context7, codegraph,
// the engram@engram plugin, and all pcs permissions already exist.
func seedPreExistingMCP(t *testing.T, home string) (claudeJSONPath, settingsPath, claudeJSON, settings string) {
	t.Helper()
	claudeJSONPath = filepath.Join(home, ".claude.json")
	settingsPath = filepath.Join(home, ".claude", "settings.json")
	claudeJSON = canonicalJSON(t, map[string]any{
		"mcpServers": map[string]any{
			"engram":    map[string]any{"command": "/custom/engram", "args": []any{"mcp"}},
			"context7":  map[string]any{"command": "npx", "args": []any{"-y", "custom-context7"}},
			"codegraph": map[string]any{"command": "/custom/codegraph", "args": []any{"serve"}},
		},
		"theme": "dark",
	})
	settings = canonicalJSON(t, map[string]any{
		"enabledPlugins": map[string]any{"engram@engram": true},
		"permissions": map[string]any{"allow": []any{
			"Bash(git status)",
			"mcp__engram__*",
			"mcp__plugin_engram_engram__*",
			"mcp__codegraph__*",
			"mcp__context7__*",
		}},
	})
	if err := writeFileCreatingDirs(claudeJSONPath, []byte(claudeJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeFileCreatingDirs(settingsPath, []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	return claudeJSONPath, settingsPath, claudeJSON, settings
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s changed\ngot:  %s\nwant: %s", path, got, want)
	}
}

func TestUninstallRemoveMCPKeepsPreExistingServersPluginAndPermissions(t *testing.T) {
	home := isolateEnv(t)
	claudeJSONPath, settingsPath, claudeJSON, settings := seedPreExistingMCP(t, home)

	opts := Options{Scope: ScopeGlobal, HomeDir: home, WriteMCPConfig: true, WriteHook: true}
	if _, err := Install(opts); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	saved, err := readState(filepath.Join(home, ".claude", StateFileName))
	if err != nil {
		t.Fatal(err)
	}
	if saved.AddedMCPServers == nil || len(saved.AddedMCPServers) != 0 {
		t.Errorf("AddedMCPServers = %#v, want an empty (non-nil) record", saved.AddedMCPServers)
	}
	if saved.AddedPlugins == nil || len(saved.AddedPlugins) != 0 {
		t.Errorf("AddedPlugins = %#v, want an empty (non-nil) record", saved.AddedPlugins)
	}

	opts.RemoveMCP = true
	if _, err := Uninstall(opts); err != nil {
		t.Fatalf("Uninstall(--remove-mcp) error: %v", err)
	}
	assertFileContent(t, claudeJSONPath, claudeJSON)
	assertFileContent(t, settingsPath, settings)
}

func TestUninstallRemoveMCPWithoutRecordRefusesUnlessForced(t *testing.T) {
	tests := []struct {
		name  string
		state string // raw .pcs-state.json content; empty means no state file
	}{
		{name: "no state file"},
		{name: "legacy state without MCP delta record", state: `{"version":"0.0.9","scope":"global","adapter":"react-shadcn","persona":"gentleman"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateEnv(t)
			claudeJSONPath, settingsPath, claudeJSON, settings := seedPreExistingMCP(t, home)
			if tt.state != "" {
				if err := writeFileCreatingDirs(filepath.Join(home, ".claude", StateFileName), []byte(tt.state), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotTree(t, home)

			_, err := Uninstall(Options{Scope: ScopeGlobal, HomeDir: home, RemoveMCP: true})
			if !errors.Is(err, ErrNoMCPRecord) {
				t.Fatalf("Uninstall(--remove-mcp) error = %v, want ErrNoMCPRecord", err)
			}
			assertSnapshotEqual(t, "home after refused uninstall", before, snapshotTree(t, home))

			// Without --remove-mcp, uninstall proceeds and leaves MCP config untouched.
			if _, err := Uninstall(Options{Scope: ScopeGlobal, HomeDir: home}); err != nil {
				t.Fatalf("Uninstall() error: %v", err)
			}
			assertFileContent(t, claudeJSONPath, claudeJSON)
			assertFileContent(t, settingsPath, settings)
		})
	}
}

func TestUninstallRemoveMCPWithoutRecordForced(t *testing.T) {
	home := isolateEnv(t)
	claudeJSONPath, settingsPath, _, _ := seedPreExistingMCP(t, home)

	rep, err := Uninstall(Options{Scope: ScopeGlobal, HomeDir: home, RemoveMCP: true, Force: true})
	if err != nil {
		t.Fatalf("Uninstall(--remove-mcp --force) error: %v", err)
	}
	if len(rep.Warnings) == 0 {
		t.Errorf("expected a warning for forced MCP removal")
	}

	claudeBytes, err := os.ReadFile(claudeJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`"engram"`, `"context7"`, `"codegraph"`} {
		if strings.Contains(string(claudeBytes), s) {
			t.Errorf("expected forced removal of %s, got:\n%s", s, claudeBytes)
		}
	}
	settingsBytes, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(settingsBytes), "engram@engram") || strings.Contains(string(settingsBytes), "mcp__engram__*") {
		t.Errorf("expected forced removal of engram plugin and permissions, got:\n%s", settingsBytes)
	}
	if !strings.Contains(string(settingsBytes), "Bash(git status)") {
		t.Errorf("expected user permission to survive forced removal, got:\n%s", settingsBytes)
	}
}

func TestStateMCPDeltaRecordJSONRoundTrip(t *testing.T) {
	recorded := State{AddedMCPServers: []string{}, AddedPermissions: []string{}, AddedPlugins: []string{}}
	b, err := json.Marshal(recorded)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"added_mcp_servers":[]`, `"added_permissions":[]`, `"added_plugins":[]`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("marshalled state %s missing %s", b, key)
		}
	}
	var back State
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !back.hasMCPDeltaRecord() {
		t.Errorf("empty recorded lists must survive a JSON round trip as a record")
	}

	var legacy State
	if err := json.Unmarshal([]byte(`{"scope":"global"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.hasMCPDeltaRecord() {
		t.Errorf("a state without added_* fields must not count as a record")
	}
}
