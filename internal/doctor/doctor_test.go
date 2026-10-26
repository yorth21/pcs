package doctor

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/yorth21/pcs/internal/installer"
)

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
			tmpHome := t.TempDir()
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
