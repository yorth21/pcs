package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLISubcommands(t *testing.T) {
	fakeHome := isolateCLIEnv(t)
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
			wantStdout:   []string{"Product Composition System (pcs)", "skill-registry refresh", "--write-mcp-config", "--write-hook"},
		},
		{
			name:         "version command text output",
			args:         []string{"version"},
			wantExitCode: 0,
			wantStdout:   []string{"pcs version ", "(commit ", ", built "},
		},
		{
			name:         "version command json output",
			args:         []string{"version", "--json"},
			wantExitCode: 0,
			wantJSONKey:  "version",
		},
		{
			name:         "install dry-run json output",
			args:         []string{"install", "--scope", "profile", "--profile-dir", profileDir, "--dry-run", "--json"},
			wantExitCode: 0,
			wantJSONKey:  "actions",
		},
		{
			name:         "install profile scope succeeds with --write-mcp-config and --write-hook",
			args:         []string{"install", "--scope", "profile", "--profile-dir", profileDir, "--persona", "gentleman", "--write-mcp-config", "--write-hook"},
			wantExitCode: 0,
			wantStdout:   []string{"pcs install completed", "claude-pcs"},
		},
		{
			name:         "status command reports installed true",
			args:         []string{"status", "--scope", "profile", "--profile-dir", profileDir},
			wantExitCode: 0,
			wantStdout:   []string{"Installed:  true", "pcs:orchestrator: true"},
		},
		{
			name:         "sync command refreshes installed profile and supports flag overrides",
			args:         []string{"sync", "--scope", "profile", "--profile-dir", profileDir, "--persona", "neutral", "--mcp-guidance=false", "--json"},
			wantExitCode: 0,
			wantJSONKey:  "operation",
		},
		{
			name:         "status after sync reflects overridden persona and removed mcp-guidance markers",
			args:         []string{"status", "--scope", "profile", "--profile-dir", profileDir},
			wantExitCode: 0,
			wantStdout:   []string{"Persona:    neutral", "pcs:codegraph-guidance: false", "pcs:engram-protocol: false"},
		},
		{
			name:         "doctor command verifies installed target dir",
			args:         []string{"doctor", "--target-dir", profileDir},
			wantExitCode: 0,
			wantStdout:   []string{"Verdict: HEALTHY"},
		},
		{
			name:         "skill-registry refresh generates .atl/pcs-skill-registry.md",
			args:         []string{"skill-registry", "refresh", "--cwd", fakeHome, "--skills-dir", filepath.Join(profileDir, "skills")},
			wantExitCode: 0,
			wantStdout:   []string{"pcs-skill-registry.md", "2 skills"},
		},
		{
			name:         "uninstall command removes pcs artifacts cleanly",
			args:         []string{"uninstall", "--scope", "profile", "--profile-dir", profileDir},
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
	t.Run("default install without flags uses global scope (<HomeDir>/.claude) and does not touch .claude.json", func(t *testing.T) {
		fakeHome := isolateCLIEnv(t)
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
		if _, err := os.Stat(filepath.Join(fakeHome, ".claude.json")); !os.IsNotExist(err) {
			t.Errorf("expected default install not to create .claude.json without --write-mcp-config")
		}
	})

	t.Run("default global install respects CLAUDE_CONFIG_DIR when set", func(t *testing.T) {
		fakeHome := isolateCLIEnv(t)
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
		fakeHome := isolateCLIEnv(t)
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

func TestCLICodeGraphInit(t *testing.T) {
	fakeHome := isolateCLIEnv(t)
	validRepo := filepath.Join(fakeHome, "projects", "frontend-app")
	if err := os.MkdirAll(validRepo, 0o755); err != nil {
		t.Fatal(err)
	}

	type codeGraphInitCase struct {
		name         string
		args         []string
		wantExitCode int
		wantStderr   string
		wantStdout   string
	}

	tests := []codeGraphInitCase{
		{
			name:         "succeeds on valid project root and invokes codegraph init",
			args:         []string{"codegraph", "init", "--cwd", validRepo},
			wantExitCode: 0,
			wantStdout:   "Initialized CodeGraph index",
		},
		{
			name:         "succeeds with --json output on valid project root",
			args:         []string{"codegraph", "init", "--cwd", validRepo, "--json"},
			wantExitCode: 0,
			wantStdout:   `"status": "initialized"`,
		},
		{
			name:         "refuses to initialize in user home directory",
			args:         []string{"codegraph", "init", "--cwd", fakeHome},
			wantExitCode: 1,
			wantStderr:   "cannot initialize CodeGraph in user home directory",
		},
		{
			name:         "refuses to initialize at filesystem root",
			args:         []string{"codegraph", "init", "--cwd", "/"},
			wantExitCode: 1,
			wantStderr:   "cannot initialize CodeGraph at filesystem root",
		},
		{
			name:         "refuses to initialize in system temporary directory os.TempDir()",
			args:         []string{"codegraph", "init", "--cwd", os.TempDir()},
			wantExitCode: 1,
			wantStderr:   "cannot initialize CodeGraph in temporary directory",
		},
		{
			name:         "refuses to initialize in non-existent directory",
			args:         []string{"codegraph", "init", "--cwd", filepath.Join(fakeHome, "missing-dir")},
			wantExitCode: 1,
			wantStderr:   "does not exist",
		},
	}

	if runtime.GOOS != "windows" {
		// The Unix temporary roots are only meaningful off Windows; os.TempDir()
		// above covers the platform temporary directory everywhere.
		for _, dir := range []string{"/tmp", "/var/tmp"} {
			tests = append(tests, codeGraphInitCase{
				name:         "refuses to initialize in " + dir,
				args:         []string{"codegraph", "init", "--cwd", dir},
				wantExitCode: 1,
				wantStderr:   "cannot initialize CodeGraph in temporary directory",
			})
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, Env{
				Stdout:  &stdout,
				Stderr:  &stderr,
				HomeDir: fakeHome,
				WorkDir: validRepo,
				ExecCommand: func(name string, cmdArgs ...string) ([]byte, error) {
					return []byte("ok: " + name + " " + strings.Join(cmdArgs, " ")), nil
				},
			})
			if code != tt.wantExitCode {
				t.Fatalf("Run(%v) exit = %d, want %d\nstdout: %s\nstderr: %s",
					tt.args, code, tt.wantExitCode, stdout.String(), stderr.String())
			}
			if tt.wantStdout != "" && !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want substring %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestVersionJSONIncludesBuildMetadata(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"version", "--json"}, Env{Stdout: &stdout, Stderr: &stderr}); code != 0 {
		t.Fatalf("pcs version --json exited %d: %s", code, stderr.String())
	}
	var parsed map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("decode %q: %v", stdout.String(), err)
	}
	for _, key := range []string{"name", "version", "commit", "date"} {
		if parsed[key] == "" {
			t.Errorf("pcs version --json missing %q: %v", key, parsed)
		}
	}
}
