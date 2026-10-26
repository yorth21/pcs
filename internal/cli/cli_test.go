package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLISubcommands(t *testing.T) {
	fakeHome := t.TempDir()
	profileDir := filepath.Join(fakeHome, ".claude-pcs")

	tests := []struct {
		name         string
		args         []string
		wantExitCode int
		wantStdout   []string
		wantJSONKey  string
	}{
		{
			name:         "help command prints usage",
			args:         []string{"help"},
			wantExitCode: 0,
			wantStdout:   []string{"Product Composition System (pcs)", "skill-registry refresh"},
		},
		{
			name:         "version command text output",
			args:         []string{"version"},
			wantExitCode: 0,
			wantStdout:   []string{"pcs version 0.1.0"},
		},
		{
			name:         "version command json output",
			args:         []string{"version", "--json"},
			wantExitCode: 0,
			wantJSONKey:  "version",
		},
		{
			name:         "install dry-run json output",
			args:         []string{"install", "--scope", "profile", "--target-dir", profileDir, "--dry-run", "--json"},
			wantExitCode: 0,
			wantJSONKey:  "actions",
		},
		{
			name:         "install profile scope succeeds",
			args:         []string{"install", "--scope", "profile", "--target-dir", profileDir, "--persona", "gentleman", "--with-mcp=true"},
			wantExitCode: 0,
			wantStdout:   []string{"pcs install completed", "claude-pcs"},
		},
		{
			name:         "status command reports installed true",
			args:         []string{"status", "--scope", "profile", "--target-dir", profileDir},
			wantExitCode: 0,
			wantStdout:   []string{"Installed:  true", "pcs:orchestrator: true"},
		},
		{
			name:         "sync command refreshes installed profile",
			args:         []string{"sync", "--scope", "profile", "--target-dir", profileDir, "--json"},
			wantExitCode: 0,
			wantJSONKey:  "operation",
		},
		{
			name:         "doctor command verifies installed target dir",
			args:         []string{"doctor", "--target-dir", profileDir},
			wantExitCode: 0,
			wantStdout:   []string{"Verdict: HEALTHY"},
		},
		{
			name:         "skill-registry refresh generates .atl/skill-registry.md",
			args:         []string{"skill-registry", "refresh", "--cwd", fakeHome, "--skills-dir", filepath.Join(profileDir, "skills")},
			wantExitCode: 0,
			wantStdout:   []string{".atl/skill-registry.md", "2 skills"},
		},
		{
			name:         "uninstall command removes pcs artifacts cleanly",
			args:         []string{"uninstall", "--scope", "profile", "--target-dir", profileDir},
			wantExitCode: 0,
			wantStdout:   []string{"pcs uninstall completed"},
		},
		{
			name:         "unknown command returns exit code 1",
			args:         []string{"nonexistent"},
			wantExitCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PCS_INSTALL_SCOPE", "")
			t.Setenv("CLAUDE_CONFIG_DIR", "")
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, Env{
				Stdout:  &stdout,
				Stderr:  &stderr,
				HomeDir: fakeHome,
				WorkDir: fakeHome,
			})
			if code != tt.wantExitCode {
				t.Fatalf("Run(%v) exit code = %d, want %d\nstdout: %s\nstderr: %s",
					tt.args, code, tt.wantExitCode, stdout.String(), stderr.String())
			}

			out := stdout.String()
			for _, sub := range tt.wantStdout {
				if !strings.Contains(out, sub) {
					t.Errorf("expected stdout to contain %q, got:\n%s", sub, out)
				}
			}

			if tt.wantJSONKey != "" {
				var parsed map[string]any
				if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
					t.Fatalf("expected valid JSON output, got error %v in:\n%s", err, out)
				}
				if _, ok := parsed[tt.wantJSONKey]; !ok {
					t.Errorf("expected JSON key %q in %v", tt.wantJSONKey, parsed)
				}
			}
		})
	}
}

func TestCLIDefaultGlobalScopeAndEnvOverrides(t *testing.T) {
	t.Run("default install without flags uses global scope (<HomeDir>/.claude)", func(t *testing.T) {
		fakeHome := t.TempDir()
		t.Setenv("PCS_INSTALL_SCOPE", "")
		t.Setenv("CLAUDE_CONFIG_DIR", "")

		var stdout, stderr bytes.Buffer
		code := Run([]string{"install", "--json"}, Env{
			Stdout:  &stdout,
			Stderr:  &stderr,
			HomeDir: fakeHome,
			WorkDir: fakeHome,
		})
		if code != 0 {
			t.Fatalf("install exit code = %d, stderr: %s", code, stderr.String())
		}
		var rep map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
			t.Fatal(err)
		}
		if rep["scope"] != "global" {
			t.Errorf("expected default scope=global, got %v", rep["scope"])
		}
		wantTarget := filepath.Join(fakeHome, ".claude")
		if rep["target_dir"] != wantTarget {
			t.Errorf("target_dir = %v, want %s", rep["target_dir"], wantTarget)
		}
	})

	t.Run("default global install respects CLAUDE_CONFIG_DIR when set", func(t *testing.T) {
		fakeHome := t.TempDir()
		customConfig := filepath.Join(fakeHome, "isolated-claude")
		t.Setenv("PCS_INSTALL_SCOPE", "")
		t.Setenv("CLAUDE_CONFIG_DIR", customConfig)

		var stdout, stderr bytes.Buffer
		code := Run([]string{"install", "--json"}, Env{
			Stdout:  &stdout,
			Stderr:  &stderr,
			HomeDir: fakeHome,
			WorkDir: fakeHome,
		})
		if code != 0 {
			t.Fatalf("install exit code = %d, stderr: %s", code, stderr.String())
		}
		var rep map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
			t.Fatal(err)
		}
		if rep["scope"] != "global" {
			t.Errorf("expected scope=global, got %v", rep["scope"])
		}
		if rep["target_dir"] != customConfig {
			t.Errorf("target_dir = %v, want %s", rep["target_dir"], customConfig)
		}
	})

	t.Run("PCS_INSTALL_SCOPE overrides default scope when --scope is omitted", func(t *testing.T) {
		fakeHome := t.TempDir()
		t.Setenv("PCS_INSTALL_SCOPE", "profile")
		t.Setenv("CLAUDE_CONFIG_DIR", "")

		var stdout, stderr bytes.Buffer
		code := Run([]string{"install", "--dry-run", "--json"}, Env{
			Stdout:  &stdout,
			Stderr:  &stderr,
			HomeDir: fakeHome,
			WorkDir: fakeHome,
		})
		if code != 0 {
			t.Fatalf("install exit code = %d, stderr: %s", code, stderr.String())
		}
		var rep map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
			t.Fatal(err)
		}
		if rep["scope"] != "profile" {
			t.Errorf("expected scope=profile from PCS_INSTALL_SCOPE, got %v", rep["scope"])
		}
	})
}
