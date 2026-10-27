package doctor

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/yorth21/pcs/internal/mcp"
)

func TestDoctorProfileChecksMCPServersInSharedMCPJSON(t *testing.T) {
	home := isolateDoctorEnv(t)
	stubInstallEngram(t, "/fake/bin/engram")
	writeJSON(t, filepath.Join(home, ".claude.json"), map[string]any{"mcpServers": map[string]any{
		"context7":  map[string]any{"command": "npx"},
		"codegraph": map[string]any{"command": "codegraph"},
	}})
	target := installProfile(t, home, false)

	rep := runProfileDoctor(t, home, target, "linux")
	for _, srv := range []string{mcp.ServerEngram, mcp.ServerContext7, mcp.ServerCodeGraph} {
		c, ok := findCheck(t, rep, "target-mcp", "mcp:"+srv)
		if !ok || c.Status != "ok" {
			t.Errorf("mcp:%s = %+v (found %v), want ok from shared-mcp.json", srv, c, ok)
			continue
		}
		if !strings.Contains(c.Detail, "shared-mcp.json") {
			t.Errorf("mcp:%s detail %q does not name shared-mcp.json", srv, c.Detail)
		}
	}
}

func TestDoctorProfileWarnsForMCPServerMissingFromSharedMCPJSON(t *testing.T) {
	home := isolateDoctorEnv(t)
	stubInstallEngram(t, "/fake/bin/engram")
	// A main profile with servers is mirrored as is, so context7 stays missing.
	writeJSON(t, filepath.Join(home, ".claude.json"), map[string]any{"mcpServers": map[string]any{"main-server": map[string]any{"command": "custom-mcp"}}})
	target := installProfile(t, home, false)

	c, ok := findCheck(t, runProfileDoctor(t, home, target, "linux"), "target-mcp", "mcp:"+mcp.ServerContext7)
	if !ok || c.Status != "warn" || !strings.Contains(c.Detail, "shared-mcp.json") {
		t.Fatalf("mcp:context7 = %+v (found %v), want warn naming shared-mcp.json", c, ok)
	}
}
