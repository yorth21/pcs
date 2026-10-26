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
