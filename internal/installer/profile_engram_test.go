package installer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yorth21/pcs/internal/mcp"
)

// stubEngramBinary makes mcp engram resolution return path, or nothing when
// path is empty, without consulting the host.
func stubEngramBinary(t *testing.T, path string) {
	t.Helper()
	orig := mcp.LookPathFunc
	t.Cleanup(func() { mcp.LookPathFunc = orig })
	mcp.LookPathFunc = func(file string) (string, error) {
		if file == "engram" && path != "" {
			return path, nil
		}
		return "", errors.New("not found")
	}
}

// seedMainWithEngramPluginOnly gives the main profile a non-engram MCP server
// and provides Engram only through the engram@engram plugin.
func seedMainWithEngramPluginOnly(t *testing.T, home string) {
	t.Helper()
	seedMainProfile(t, home)
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers": {"main-server": {"command": "custom-mcp"}}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func sharedEngram(t *testing.T, profileDir string) (map[string]any, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(profileDir, "shared-mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		MCPServers map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	engram, ok := root.MCPServers["engram"].(map[string]any)
	return engram, ok
}

func TestProfileInstallProvidesEngramWhenMainHasOnlyThePlugin(t *testing.T) {
	home := isolateEnv(t)
	seedMainWithEngramPluginOnly(t, home)
	stubEngramBinary(t, "/fake/bin/engram")
	mainDir, mainJSON := filepath.Join(home, ".claude"), filepath.Join(home, ".claude.json")
	beforeDir, beforeJSON := snapshotTree(t, mainDir), snapshotTree(t, mainJSON)

	opts := Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin")}
	for _, run := range []struct {
		name string
		fn   func(Options) (*Report, error)
	}{{"install", Install}, {"sync", Sync}} {
		rep, err := run.fn(opts)
		if err != nil {
			t.Fatalf("%s error: %v", run.name, err)
		}
		engram, ok := sharedEngram(t, rep.TargetDir)
		if !ok {
			t.Fatalf("%s: shared-mcp.json has no engram server", run.name)
		}
		if engram["command"] != "/fake/bin/engram" {
			t.Errorf("%s: engram command = %v, want /fake/bin/engram", run.name, engram["command"])
		}
		for _, w := range rep.Warnings {
			if strings.Contains(w, "engram binary") {
				t.Errorf("%s: unexpected engram warning: %s", run.name, w)
			}
		}
	}
	assertSnapshotEqual(t, "main ~/.claude", beforeDir, snapshotTree(t, mainDir))
	assertSnapshotEqual(t, "main ~/.claude.json", beforeJSON, snapshotTree(t, mainJSON))
}

func TestProfileInstallWithoutEngramBinaryWarnsInsteadOfWritingABrokenEntry(t *testing.T) {
	home := isolateEnv(t)
	seedMainWithEngramPluginOnly(t, home)
	stubEngramBinary(t, "")

	rep, err := Install(Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin")})
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}
	if engram, ok := sharedEngram(t, rep.TargetDir); ok {
		t.Errorf("shared-mcp.json has an engram entry without a resolvable binary: %v", engram)
	}
	found := false
	for _, w := range rep.Warnings {
		if strings.Contains(w, "engram binary") && strings.Contains(w, "pcs sync --scope profile") {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings %q lack the engram install hint", rep.Warnings)
	}
}
