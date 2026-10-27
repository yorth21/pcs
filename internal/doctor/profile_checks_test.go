package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yorth21/pcs/internal/installer"
)

func findCheck(t *testing.T, rep *DiagnosticReport, category, name string) (CheckResult, bool) {
	t.Helper()
	for _, c := range rep.Checks {
		if c.Category == category && c.Name == name {
			return c, true
		}
	}
	return CheckResult{}, false
}

func lookPathAll(file string) (string, error) { return "/usr/bin/" + file, nil }

func installProfile(t *testing.T, home string, withPlugins bool) string {
	t.Helper()
	target := filepath.Join(home, ".claude-pcs")
	// PATH is an empty directory (isolateDoctorEnv), so --with-plugins cannot
	// reach a real claude; the failure is only reported as an install warning.
	if _, err := installer.Install(installer.Options{
		Scope:       installer.ScopeProfile,
		TargetDir:   target,
		HomeDir:     home,
		LauncherDir: filepath.Join(home, "bin"),
		WithPlugins: withPlugins,
	}); err != nil {
		t.Fatal(err)
	}
	return target
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func runProfileDoctor(t *testing.T, home, target, goos string) *DiagnosticReport {
	t.Helper()
	rep, err := Run(Options{Scope: installer.ScopeProfile, TargetDir: target, HomeDir: home, LookPathFunc: lookPathAll, GOOS: goos})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	return rep
}

func TestDoctorProfilePluginsCheck(t *testing.T) {
	t.Run("requested plugin missing from enabledPlugins warns", func(t *testing.T) {
		home := isolateDoctorEnv(t)
		target := installProfile(t, home, true)
		c, ok := findCheck(t, runProfileDoctor(t, home, target, "linux"), "profile", "plugin:engram@engram")
		if !ok || c.Status != "warn" {
			t.Fatalf("plugin check = %+v (found %v), want warn", c, ok)
		}
		if !strings.Contains(c.Detail, "claude plugin install engram@engram") {
			t.Errorf("plugin check lacks the fix command: %q", c.Detail)
		}
	})
	t.Run("requested plugin enabled passes", func(t *testing.T) {
		home := isolateDoctorEnv(t)
		target := installProfile(t, home, true)
		writeJSON(t, filepath.Join(target, "settings.json"), map[string]any{"enabledPlugins": map[string]any{"engram@engram": true}})
		c, ok := findCheck(t, runProfileDoctor(t, home, target, "linux"), "profile", "plugin:engram@engram")
		if !ok || c.Status != "ok" {
			t.Fatalf("plugin check = %+v (found %v), want ok", c, ok)
		}
	})
	t.Run("plugins not requested skips the check", func(t *testing.T) {
		home := isolateDoctorEnv(t)
		target := installProfile(t, home, false)
		if c, ok := findCheck(t, runProfileDoctor(t, home, target, "linux"), "profile", "plugin:engram@engram"); ok {
			t.Fatalf("unexpected plugin check without --with-plugins: %+v", c)
		}
	})
}

func TestDoctorSharedMCPSyncCheck(t *testing.T) {
	mainJSON := func(home string) string { return filepath.Join(home, ".claude.json") }
	seed := map[string]any{"mcpServers": map[string]any{"main-server": map[string]any{"command": "custom-mcp"}}}

	t.Run("in sync with the main profile passes", func(t *testing.T) {
		home := isolateDoctorEnv(t)
		writeJSON(t, mainJSON(home), seed)
		target := installProfile(t, home, false)
		c, ok := findCheck(t, runProfileDoctor(t, home, target, "linux"), "profile", "shared-mcp-sync")
		if !ok || c.Status != "ok" {
			t.Fatalf("shared-mcp-sync = %+v (found %v), want ok", c, ok)
		}
	})
	t.Run("drift from the main profile warns with the sync hint", func(t *testing.T) {
		home := isolateDoctorEnv(t)
		writeJSON(t, mainJSON(home), seed)
		target := installProfile(t, home, false)
		writeJSON(t, mainJSON(home), map[string]any{"mcpServers": map[string]any{
			"main-server": map[string]any{"command": "custom-mcp", "args": []any{"--v2"}},
			"new-server":  map[string]any{"command": "new"},
		}})
		c, ok := findCheck(t, runProfileDoctor(t, home, target, "linux"), "profile", "shared-mcp-sync")
		if !ok || c.Status != "warn" {
			t.Fatalf("shared-mcp-sync = %+v (found %v), want warn", c, ok)
		}
		for _, w := range []string{"main-server", "new-server", "pcs sync --scope profile"} {
			if !strings.Contains(c.Detail, w) {
				t.Errorf("drift detail %q missing %q", c.Detail, w)
			}
		}
	})
}

func TestDoctorProfileAuthenticationCheck(t *testing.T) {
	tests := []struct {
		name       string
		goos       string
		withCreds  bool
		wantStatus string
	}{
		{name: "credentials file present", goos: "linux", withCreds: true, wantStatus: "ok"},
		{name: "linux without credentials file warns", goos: "linux", wantStatus: "warn"},
		{name: "windows without credentials file warns", goos: "windows", wantStatus: "warn"},
		{name: "darwin without credentials file is unknown (Keychain)", goos: "darwin", wantStatus: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateDoctorEnv(t)
			// A resolvable engram keeps the Engram source check healthy.
			stubInstallEngram(t, "/usr/bin/engram")
			target := installProfile(t, home, false)
			if tt.withCreds {
				writeJSON(t, filepath.Join(target, ".credentials.json"), map[string]any{})
			}
			rep := runProfileDoctor(t, home, target, tt.goos)
			c, ok := findCheck(t, rep, "profile", "authentication")
			if !ok || c.Status != tt.wantStatus {
				t.Fatalf("authentication = %+v (found %v), want %s", c, ok, tt.wantStatus)
			}
			if tt.goos == "darwin" && !strings.Contains(c.Detail, "Keychain") {
				t.Errorf("darwin detail should mention the Keychain: %q", c.Detail)
			}
			if !rep.Healthy {
				t.Errorf("authentication state must not make the report unhealthy: %+v", rep.Checks)
			}
		})
	}
}
