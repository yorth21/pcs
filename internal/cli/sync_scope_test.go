package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateCLIEnv points every home/config environment variable at a fresh
// temporary home so CLI tests can never touch the real user profile.
func isolateCLIEnv(t *testing.T) string {
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
	// PATH holds only an empty directory, so no real claude (or other tool) is found.
	t.Setenv("PATH", t.TempDir())
	return home
}

func runCLI(t *testing.T, home string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, Env{Stdout: &stdout, Stderr: &stderr, HomeDir: home, WorkDir: home})
	return code, stdout.String(), stderr.String()
}

func snapshotFiles(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	snap := make(map[string]string)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				snap[p+string(filepath.Separator)] = "<dir>"
				return nil
			}
			b, readErr := os.ReadFile(p)
			if readErr != nil {
				return readErr
			}
			snap[p] = string(b)
			return nil
		})
		if err != nil {
			t.Fatalf("snapshot %s: %v", root, err)
		}
	}
	return snap
}

func assertSameFiles(t *testing.T, label string, before, after map[string]string) {
	t.Helper()
	for p, want := range before {
		got, ok := after[p]
		if !ok {
			t.Errorf("%s: %s was removed", label, p)
		} else if got != want {
			t.Errorf("%s: %s changed\nbefore: %q\nafter:  %q", label, p, want, got)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			t.Errorf("%s: unexpected new path %s", label, p)
		}
	}
}

func seedMain(t *testing.T, home string) {
	t.Helper()
	files := map[string]string{
		filepath.Join(home, ".claude", "CLAUDE.md"):                  "# Main profile instructions\n",
		filepath.Join(home, ".claude", "settings.json"):              "{\n  \"theme\": \"dark\"\n}\n",
		filepath.Join(home, ".claude", "agents", "custom.md"):        "# Custom agent\n",
		filepath.Join(home, ".claude", "skills", "mine", "SKILL.md"): "# Mine\n",
		filepath.Join(home, ".claude.json"):                          "{\n  \"mcpServers\": {}\n}\n",
	}
	for p, c := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSyncWithoutScopeTargetsOnlyExistingProfileInstall(t *testing.T) {
	home := isolateCLIEnv(t)
	seedMain(t, home)
	profileDir := filepath.Join(home, ".claude-pcs")

	if code, _, stderr := runCLI(t, home, "install", "--scope", "profile", "--write-mcp-config", "--write-hook"); code != 0 {
		t.Fatalf("install --scope profile exit %d: %s", code, stderr)
	}
	mainRoots := []string{filepath.Join(home, ".claude"), filepath.Join(home, ".claude.json")}
	before := snapshotFiles(t, mainRoots...)

	code, stdout, stderr := runCLI(t, home, "sync", "--json")
	if code != 0 {
		t.Fatalf("plain sync exit %d: %s", code, stderr)
	}
	var rep map[string]any
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if rep["scope"] != "profile" || rep["target_dir"] != profileDir {
		t.Errorf("sync targeted scope=%v dir=%v, want profile %s", rep["scope"], rep["target_dir"], profileDir)
	}
	assertSameFiles(t, "main profile", before, snapshotFiles(t, mainRoots...))
}

func TestSyncWithoutInstallFailsAndWritesNothing(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "plain sync", args: []string{"sync"}},
		{name: "explicit global scope", args: []string{"sync", "--scope", "global"}},
		{name: "explicit profile scope", args: []string{"sync", "--scope", "profile"}},
		{name: "profile-dir without scope", args: []string{"sync", "--profile-dir", "<home>/custom-profile"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateCLIEnv(t)
			seedMain(t, home)
			before := snapshotFiles(t, home)

			args := make([]string, len(tt.args))
			for i, a := range tt.args {
				args[i] = strings.ReplaceAll(a, "<home>", home)
			}
			code, _, stderr := runCLI(t, home, args...)
			if code == 0 {
				t.Fatalf("%v exit 0, want non-zero", tt.args)
			}
			if !strings.Contains(stderr, "no pcs installation found") {
				t.Errorf("stderr = %q, want a 'no pcs installation found' message", stderr)
			}
			assertSameFiles(t, "home", before, snapshotFiles(t, home))
		})
	}
}

func TestSyncWithMultipleInstallsRequiresScope(t *testing.T) {
	home := isolateCLIEnv(t)
	for _, scope := range []string{"global", "profile"} {
		if code, _, stderr := runCLI(t, home, "install", "--scope", scope); code != 0 {
			t.Fatalf("install --scope %s exit %d: %s", scope, code, stderr)
		}
	}
	before := snapshotFiles(t, home)

	code, _, stderr := runCLI(t, home, "sync")
	if code == 0 {
		t.Fatalf("plain sync with two installs exit 0, want non-zero")
	}
	if !strings.Contains(stderr, "--scope") || !strings.Contains(stderr, filepath.Join(home, ".claude-pcs")) {
		t.Errorf("stderr should list installs and ask for --scope, got: %s", stderr)
	}
	assertSameFiles(t, "home", before, snapshotFiles(t, home))

	if code, _, stderr := runCLI(t, home, "sync", "--scope", "profile"); code != 0 {
		t.Fatalf("sync --scope profile exit %d: %s", code, stderr)
	}
}

func TestInstallProfileRefusesMainConfigDir(t *testing.T) {
	home := isolateCLIEnv(t)
	seedMain(t, home)
	before := snapshotFiles(t, home)

	code, _, stderr := runCLI(t, home, "install", "--scope", "profile", "--profile-dir", filepath.Join(home, ".claude"))
	if code == 0 {
		t.Fatalf("install --profile-dir ~/.claude exit 0, want refusal")
	}
	if !strings.Contains(stderr, "main Claude config directory") {
		t.Errorf("stderr = %q, want main config dir refusal", stderr)
	}
	assertSameFiles(t, "home", before, snapshotFiles(t, home))
}

func TestUninstallRemoveMCPWithoutStateRequiresForce(t *testing.T) {
	home := isolateCLIEnv(t)
	seedMain(t, home)
	claudeJSON := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(claudeJSON, []byte("{\n  \"mcpServers\": {\n    \"engram\": {}\n  }\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, home)

	code, _, stderr := runCLI(t, home, "uninstall", "--remove-mcp")
	if code == 0 {
		t.Fatalf("uninstall --remove-mcp without state exit 0, want refusal")
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("stderr = %q, want a hint about --force", stderr)
	}
	assertSameFiles(t, "home", before, snapshotFiles(t, home))

	code, _, stderr = runCLI(t, home, "uninstall", "--remove-mcp", "--force")
	if code != 0 {
		t.Fatalf("uninstall --remove-mcp --force exit %d: %s", code, stderr)
	}
	if !strings.Contains(strings.ToLower(stderr), "warning") {
		t.Errorf("stderr = %q, want a warning for forced removal", stderr)
	}
	b, err := os.ReadFile(claudeJSON)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "engram") {
		t.Errorf("expected forced removal of engram, got:\n%s", b)
	}
}
