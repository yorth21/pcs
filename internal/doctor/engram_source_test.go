package doctor

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yorth21/pcs/internal/mcp"
)

// stubInstallEngram makes the installer resolve engram to path, or to nothing
// when path is empty, without consulting the host.
func stubInstallEngram(t *testing.T, path string) {
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

func TestDoctorProfileEngramSourceCheck(t *testing.T) {
	seedMainPluginOnly := func(t *testing.T, home string) {
		writeJSON(t, filepath.Join(home, ".claude.json"), map[string]any{"mcpServers": map[string]any{"main-server": map[string]any{"command": "custom-mcp"}}})
		writeJSON(t, filepath.Join(home, ".claude", "settings.json"), map[string]any{"enabledPlugins": map[string]any{"engram@engram": true}})
	}

	t.Run("pcs-provided engram server passes and keeps shared-mcp in sync", func(t *testing.T) {
		home := isolateDoctorEnv(t)
		seedMainPluginOnly(t, home)
		stubInstallEngram(t, "/fake/bin/engram")
		target := installProfile(t, home, false)
		rep := runProfileDoctor(t, home, target, "linux")
		c, ok := findCheck(t, rep, "profile", "engram")
		if !ok || c.Status != "ok" || !strings.Contains(c.Detail, "shared-mcp.json") {
			t.Fatalf("engram check = %+v (found %v), want ok naming shared-mcp.json", c, ok)
		}
		if c, ok := findCheck(t, rep, "profile", "shared-mcp-sync"); !ok || c.Status != "ok" {
			t.Errorf("shared-mcp-sync = %+v (found %v), want ok (pcs-provided engram is not drift)", c, ok)
		}
	})
	t.Run("no engram binary and no profile plugin fails", func(t *testing.T) {
		home := isolateDoctorEnv(t)
		seedMainPluginOnly(t, home)
		stubInstallEngram(t, "")
		target := installProfile(t, home, false)
		rep := runProfileDoctor(t, home, target, "linux")
		c, ok := findCheck(t, rep, "profile", "engram")
		if !ok || c.Status != "error" {
			t.Fatalf("engram check = %+v (found %v), want error", c, ok)
		}
		if rep.Healthy {
			t.Errorf("report is healthy without any Engram source")
		}
		for _, w := range []string{"pcs sync --scope profile", "--with-plugins"} {
			if !strings.Contains(c.Detail, w) {
				t.Errorf("engram check detail %q missing %q", c.Detail, w)
			}
		}
	})
	t.Run("engram plugin enabled in the profile passes", func(t *testing.T) {
		home := isolateDoctorEnv(t)
		seedMainPluginOnly(t, home)
		stubInstallEngram(t, "")
		target := installProfile(t, home, false)
		writeJSON(t, filepath.Join(target, "settings.json"), map[string]any{"enabledPlugins": map[string]any{"engram@engram": true}})
		c, ok := findCheck(t, runProfileDoctor(t, home, target, "linux"), "profile", "engram")
		if !ok || c.Status != "ok" || !strings.Contains(c.Detail, "engram@engram") {
			t.Fatalf("engram check = %+v (found %v), want ok naming the plugin", c, ok)
		}
	})
}
