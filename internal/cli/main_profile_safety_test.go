package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mainRoots lists the main Claude profile locations that profile-scoped
// commands must never modify.
func mainRoots(home string) []string {
	return []string{filepath.Join(home, ".claude"), filepath.Join(home, ".claude.json"), filepath.Join(home, "CLAUDE.md"), filepath.Join(home, "settings.json")}
}

func installProfileOnly(t *testing.T, home string) string {
	t.Helper()
	seedMain(t, home)
	if code, _, stderr := runCLI(t, home, "install", "--scope", "profile", "--write-mcp-config", "--write-hook"); code != 0 {
		t.Fatalf("install --scope profile exit %d: %s", code, stderr)
	}
	return filepath.Join(home, ".claude-pcs")
}

func TestInstallProfileRefusesHomeAndItsAncestors(t *testing.T) {
	tests := []struct {
		name string
		dir  func(home string) string
	}{
		{name: "home directory", dir: func(home string) string { return home }},
		{name: "parent of the home directory", dir: func(home string) string { return filepath.Dir(home) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := isolateCLIEnv(t)
			home := filepath.Join(root, "users", "me")
			seedMain(t, home)
			before := snapshotFiles(t, root)

			code, _, stderr := runCLI(t, home, "install", "--profile-dir", tt.dir(home))
			if code == 0 {
				t.Fatalf("install --profile-dir %s exit 0, want refusal", tt.dir(home))
			}
			if !strings.Contains(stderr, "refusing profile target") {
				t.Errorf("stderr = %q, want a profile target refusal", stderr)
			}
			assertSameFiles(t, "isolated root", before, snapshotFiles(t, root))
		})
	}
}

func TestUninstallWithoutScopeTargetsOnlyExistingProfileInstall(t *testing.T) {
	home := isolateCLIEnv(t)
	profileDir := installProfileOnly(t, home)
	before := snapshotFiles(t, mainRoots(home)...)

	code, _, stderr := runCLI(t, home, "uninstall")
	if code != 0 {
		t.Fatalf("plain uninstall exit %d: %s", code, stderr)
	}
	assertSameFiles(t, "main profile", before, snapshotFiles(t, mainRoots(home)...))
	if _, err := os.Stat(filepath.Join(profileDir, ".pcs-state.json")); !os.IsNotExist(err) {
		t.Errorf("profile .pcs-state.json still present after plain uninstall (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(profileDir, "agents", "pcs-explorer.md")); !os.IsNotExist(err) {
		t.Errorf("profile pcs agents still present after plain uninstall (err=%v)", err)
	}
}

func TestUninstallWithoutScopeRefusesWithoutInstallOrWithSeveral(t *testing.T) {
	t.Run("no install", func(t *testing.T) {
		home := isolateCLIEnv(t)
		seedMain(t, home)
		before := snapshotFiles(t, home)
		code, _, stderr := runCLI(t, home, "uninstall")
		if code == 0 {
			t.Fatalf("plain uninstall without installs exit 0, want refusal")
		}
		if !strings.Contains(stderr, "no pcs installation found") {
			t.Errorf("stderr = %q, want 'no pcs installation found'", stderr)
		}
		assertSameFiles(t, "home", before, snapshotFiles(t, home))
	})
	t.Run("global and profile installs", func(t *testing.T) {
		home := isolateCLIEnv(t)
		for _, scope := range []string{"global", "profile"} {
			if code, _, stderr := runCLI(t, home, "install", "--scope", scope); code != 0 {
				t.Fatalf("install --scope %s exit %d: %s", scope, code, stderr)
			}
		}
		before := snapshotFiles(t, home)
		code, _, stderr := runCLI(t, home, "uninstall")
		if code == 0 {
			t.Fatalf("plain uninstall with two installs exit 0, want refusal")
		}
		if !strings.Contains(stderr, "--scope") {
			t.Errorf("stderr = %q, want a request for --scope", stderr)
		}
		assertSameFiles(t, "home", before, snapshotFiles(t, home))
	})
}

func TestUninstallForceRequiresExplicitScope(t *testing.T) {
	tests := []struct {
		name     string
		envScope string
	}{
		{name: "no scope at all"},
		{name: "PCS_INSTALL_SCOPE only", envScope: "global"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateCLIEnv(t)
			installProfileOnly(t, home)
			t.Setenv("PCS_INSTALL_SCOPE", tt.envScope)
			before := snapshotFiles(t, home)

			code, _, stderr := runCLI(t, home, "uninstall", "--remove-mcp", "--force")
			if code == 0 {
				t.Fatalf("uninstall --remove-mcp --force without --scope exit 0, want refusal")
			}
			if !strings.Contains(stderr, "--force requires") {
				t.Errorf("stderr = %q, want an explicit-scope refusal for --force", stderr)
			}
			assertSameFiles(t, "home", before, snapshotFiles(t, home))
		})
	}
}

func TestInstallWithoutScopeRefusesWhenOnlyAProfileInstallExists(t *testing.T) {
	home := isolateCLIEnv(t)
	installProfileOnly(t, home)
	before := snapshotFiles(t, home)

	code, _, stderr := runCLI(t, home, "install")
	if code == 0 {
		t.Fatalf("plain install with a profile-only install exit 0, want refusal")
	}
	for _, want := range []string{"--scope global", "--scope profile"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
	}
	assertSameFiles(t, "home", before, snapshotFiles(t, home))

	if code, _, stderr := runCLI(t, home, "install", "--scope", "profile"); code != 0 {
		t.Fatalf("install --scope profile (re-install) exit %d: %s", code, stderr)
	}
}

func TestInstallWithoutScopeDefaultsToGlobalWhenNothingIsInstalled(t *testing.T) {
	home := isolateCLIEnv(t)
	if code, _, stderr := runCLI(t, home, "install"); code != 0 {
		t.Fatalf("plain install exit %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", ".pcs-state.json")); err != nil {
		t.Errorf("plain install did not create the global install: %v", err)
	}
}
