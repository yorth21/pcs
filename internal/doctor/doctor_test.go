package doctor

import (
	"fmt"
	"github.com/yorth21/pcs/internal/mcp"
	"path/filepath"
	"testing"

	"github.com/yorth21/pcs/internal/installer"
)

// isolateDoctorEnv points every home/config environment variable at a fresh
// temporary home and PATH at an empty directory, so doctor tests (which also
// install profiles) can never read or mutate the real user profile.
func isolateDoctorEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("PCS_INSTALL_SCOPE", "")
	t.Setenv("PATH", t.TempDir())
	// Engram resolution must not see a real engram installed on the host.
	origFallback := mcp.EngramFallbackPaths
	mcp.EngramFallbackPaths = nil
	t.Cleanup(func() { mcp.EngramFallbackPaths = origFallback })
	return home
}

func TestRunDoctor(t *testing.T) {
	stubAllBinariesFound := func(file string) (string, error) {
		return "/usr/local/bin/" + file, nil
	}
	stubMissingBinaries := func(file string) (string, error) {
		return "", fmt.Errorf("%s not found", file)
	}

	tests := []struct {
		name        string
		setupTarget func(t *testing.T, dir string) string
		lookPath    func(string) (string, error)
		wantHealthy bool
	}{
		{
			name: "healthy when embedded brain is intact and no target dir is installed yet",
			setupTarget: func(t *testing.T, _ string) string {
				return ""
			},
			lookPath:    stubAllBinariesFound,
			wantHealthy: true,
		},
		{
			name: "remains healthy with warnings when optional MCP binaries are missing on PATH",
			setupTarget: func(t *testing.T, _ string) string {
				return ""
			},
			lookPath:    stubMissingBinaries,
			wantHealthy: true,
		},
		{
			name: "healthy when explicit target profile directory is fully installed",
			setupTarget: func(t *testing.T, dir string) string {
				target := filepath.Join(dir, ".claude-pcs")
				stubInstallEngram(t, "/usr/local/bin/engram")
				if _, err := installer.Install(installer.Options{
					Scope:     installer.ScopeProfile,
					TargetDir: target,
					HomeDir:   dir,
					WithMCP:   true,
				}); err != nil {
					t.Fatal(err)
				}
				return target
			},
			lookPath:    stubAllBinariesFound,
			wantHealthy: true,
		},
		{
			name: "unhealthy when explicit target directory is empty and missing markers/agents",
			setupTarget: func(t *testing.T, dir string) string {
				return filepath.Join(dir, "uninstalled-profile")
			},
			lookPath:    stubAllBinariesFound,
			wantHealthy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpHome := isolateDoctorEnv(t)
			target := tt.setupTarget(t, tmpHome)

			rep, err := Run(Options{
				TargetDir:    target,
				HomeDir:      tmpHome,
				LookPathFunc: tt.lookPath,
			})
			if err != nil {
				t.Fatalf("Run() error: %v", err)
			}
			if rep.Healthy != tt.wantHealthy {
				t.Errorf("rep.Healthy = %v, want %v (checks: %+v)", rep.Healthy, tt.wantHealthy, rep.Checks)
			}
		})
	}
}

func TestDoctorProfileScopeChecks(t *testing.T) {
	tmpHome := isolateDoctorEnv(t)
	target := filepath.Join(tmpHome, ".claude-pcs")

	if _, err := installer.Install(installer.Options{
		Scope:     installer.ScopeProfile,
		TargetDir: target,
		HomeDir:   tmpHome,
		WithMCP:   true,
	}); err != nil {
		t.Fatal(err)
	}

	rep, err := Run(Options{
		Scope:     installer.ScopeProfile,
		TargetDir: target,
		HomeDir:   tmpHome,
		LookPathFunc: func(file string) (string, error) {
			if file == "claude-pcs" {
				return "/usr/local/bin/claude-pcs", nil
			}
			return "/usr/bin/" + file, nil
		},
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	foundLauncher := false
	foundSharedMCP := false
	foundAuth := false

	for _, c := range rep.Checks {
		if c.Category == "profile" && c.Name == "launcher" && c.Status == "ok" {
			foundLauncher = true
		}
		if c.Category == "profile" && c.Name == "shared-mcp.json" && c.Status == "ok" {
			foundSharedMCP = true
		}
		if c.Category == "profile" && c.Name == "authentication" {
			foundAuth = true
		}
	}

	if !foundLauncher {
		t.Errorf("expected ok check for profile launcher on PATH")
	}
	if !foundSharedMCP {
		t.Errorf("expected ok check for shared-mcp.json in profile")
	}
	if !foundAuth {
		t.Errorf("expected authentication check for profile")
	}
}
