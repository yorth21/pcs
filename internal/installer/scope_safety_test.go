package installer

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// isolateEnv points every home/config environment variable at a fresh temporary
// home so tests can never read or mutate the real user profile.
func isolateEnv(t *testing.T) string {
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

// snapshotTree returns path -> content for every regular file under root
// (or the single file root). Missing roots yield an empty snapshot.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := make(map[string]string)
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
		if d.Type()&fs.ModeSymlink != 0 {
			target, linkErr := os.Readlink(p)
			if linkErr != nil {
				return linkErr
			}
			snap[p] = "<symlink:" + target + ">"
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
	return snap
}

func assertSnapshotEqual(t *testing.T, label string, before, after map[string]string) {
	t.Helper()
	for p, want := range before {
		got, ok := after[p]
		if !ok {
			t.Errorf("%s: %s was removed", label, p)
			continue
		}
		if got != want {
			t.Errorf("%s: %s changed\nbefore: %q\nafter:  %q", label, p, want, got)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			t.Errorf("%s: unexpected new path %s", label, p)
		}
	}
}

func seedMainProfile(t *testing.T, home string) {
	t.Helper()
	files := map[string]string{
		filepath.Join(home, ".claude", "CLAUDE.md"):                  "# Main profile instructions\n",
		filepath.Join(home, ".claude", "settings.json"):              "{\n  \"enabledPlugins\": {\n    \"engram@engram\": true\n  }\n}\n",
		filepath.Join(home, ".claude", "agents", "custom.md"):        "# Custom agent\n",
		filepath.Join(home, ".claude", "skills", "mine", "SKILL.md"): "# Mine\n",
		filepath.Join(home, ".claude.json"):                          "{\n  \"mcpServers\": {}\n}\n",
	}
	for p, c := range files {
		if err := writeFileCreatingDirs(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProfileScopeIgnoresClaudeConfigDir(t *testing.T) {
	home := isolateEnv(t)
	seedMainProfile(t, home)
	mainDir := filepath.Join(home, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", mainDir)

	beforeMain := snapshotTree(t, mainDir)
	beforeMainJSON := snapshotTree(t, filepath.Join(home, ".claude.json"))

	rep, err := Install(Options{Scope: ScopeProfile, HomeDir: home, WithMCP: true})
	if err != nil {
		t.Fatalf("Install(profile) error: %v", err)
	}
	wantTarget := filepath.Join(home, ".claude-pcs")
	if rep.TargetDir != wantTarget {
		t.Fatalf("profile TargetDir = %q, want %q (CLAUDE_CONFIG_DIR must be ignored)", rep.TargetDir, wantTarget)
	}
	assertSnapshotEqual(t, "main ~/.claude", beforeMain, snapshotTree(t, mainDir))
	assertSnapshotEqual(t, "main ~/.claude.json", beforeMainJSON, snapshotTree(t, filepath.Join(home, ".claude.json")))
}

func TestProfileDirEqualToMainConfigDirIsRefused(t *testing.T) {
	tests := []struct {
		name string
		// opts builds the install options given the isolated home.
		opts func(t *testing.T, home string) Options
	}{
		{
			name: "profile-dir equals ~/.claude",
			opts: func(t *testing.T, home string) Options {
				return Options{Scope: ScopeProfile, HomeDir: home, ProfileDir: filepath.Join(home, ".claude")}
			},
		},
		{
			name: "profile-dir equals ~/.claude with trailing separator and dot segments",
			opts: func(t *testing.T, home string) Options {
				return Options{Scope: ScopeProfile, HomeDir: home, ProfileDir: filepath.Join(home, "x", "..", ".claude") + string(filepath.Separator)}
			},
		},
		{
			name: "target-dir equals ~/.claude in profile scope",
			opts: func(t *testing.T, home string) Options {
				return Options{Scope: ScopeProfile, HomeDir: home, TargetDir: filepath.Join(home, ".claude")}
			},
		},
		{
			name: "profile-dir is a symlink to ~/.claude",
			opts: func(t *testing.T, home string) Options {
				link := filepath.Join(home, "link-to-main")
				if err := os.Symlink(filepath.Join(home, ".claude"), link); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return Options{Scope: ScopeProfile, HomeDir: home, ProfileDir: link}
			},
		},
		{
			name: "profile-dir equals CLAUDE_CONFIG_DIR main target",
			opts: func(t *testing.T, home string) Options {
				custom := filepath.Join(home, "custom-main")
				if err := os.MkdirAll(custom, 0o755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("CLAUDE_CONFIG_DIR", custom)
				return Options{Scope: ScopeProfile, HomeDir: home, ProfileDir: custom}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateEnv(t)
			seedMainProfile(t, home)
			opts := tt.opts(t, home)
			before := snapshotTree(t, home)

			if _, err := Install(opts); err == nil {
				t.Fatalf("Install(%+v) succeeded, want refusal", opts)
			}
			if _, err := Uninstall(opts); err == nil {
				t.Fatalf("Uninstall(%+v) succeeded, want refusal", opts)
			}
			assertSnapshotEqual(t, "home", before, snapshotTree(t, home))
		})
	}
}

func TestProfileScopeAllowsClaudeConfigDirPointingAtExistingPCSProfile(t *testing.T) {
	home := isolateEnv(t)
	profileDir := filepath.Join(home, ".claude-pcs")
	if _, err := Install(Options{Scope: ScopeProfile, HomeDir: home}); err != nil {
		t.Fatalf("Install(profile) error: %v", err)
	}
	// Inside a claude-pcs session the launcher sets CLAUDE_CONFIG_DIR to the profile itself.
	t.Setenv("CLAUDE_CONFIG_DIR", profileDir)
	rep, err := Sync(Options{Scope: ScopeProfile, HomeDir: home})
	if err != nil {
		t.Fatalf("Sync(profile) inside a profile session error: %v", err)
	}
	if rep.TargetDir != profileDir {
		t.Errorf("TargetDir = %q, want %q", rep.TargetDir, profileDir)
	}
}

func TestSyncRefusesWithoutStateFile(t *testing.T) {
	for _, scope := range []string{ScopeGlobal, ScopeProfile} {
		t.Run(scope, func(t *testing.T) {
			home := isolateEnv(t)
			seedMainProfile(t, home)
			before := snapshotTree(t, home)

			_, err := Sync(Options{Scope: scope, HomeDir: home})
			if !errors.Is(err, ErrNotInstalled) {
				t.Fatalf("Sync(%s) error = %v, want ErrNotInstalled", scope, err)
			}
			assertSnapshotEqual(t, "home", before, snapshotTree(t, home))
		})
	}
}

func TestDiscoverInstalls(t *testing.T) {
	t.Run("no installs", func(t *testing.T) {
		home := isolateEnv(t)
		found, err := DiscoverInstalls(Options{HomeDir: home})
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 0 {
			t.Fatalf("found = %+v, want none", found)
		}
	})

	t.Run("profile only", func(t *testing.T) {
		home := isolateEnv(t)
		if _, err := Install(Options{Scope: ScopeProfile, HomeDir: home}); err != nil {
			t.Fatal(err)
		}
		found, err := DiscoverInstalls(Options{HomeDir: home})
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 1 || found[0].Scope != ScopeProfile || found[0].TargetDir != filepath.Join(home, ".claude-pcs") {
			t.Fatalf("found = %+v, want only the profile install", found)
		}
	})

	t.Run("global and profile", func(t *testing.T) {
		home := isolateEnv(t)
		for _, scope := range []string{ScopeGlobal, ScopeProfile} {
			if _, err := Install(Options{Scope: scope, HomeDir: home}); err != nil {
				t.Fatal(err)
			}
		}
		found, err := DiscoverInstalls(Options{HomeDir: home})
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 2 {
			t.Fatalf("found = %+v, want two installs", found)
		}
	})

	t.Run("CLAUDE_CONFIG_DIR pointing at the profile is reported once", func(t *testing.T) {
		home := isolateEnv(t)
		if _, err := Install(Options{Scope: ScopeProfile, HomeDir: home}); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude-pcs"))
		found, err := DiscoverInstalls(Options{HomeDir: home})
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 1 || found[0].Scope != ScopeProfile {
			t.Fatalf("found = %+v, want one profile install", found)
		}
	})
}

func TestProfileUninstallWithoutStateLeavesSettingsUntouched(t *testing.T) {
	home := isolateEnv(t)
	profileDir := filepath.Join(home, ".claude-pcs")
	settings := "{\n  \"enabledPlugins\": {\n    \"engram@engram\": true\n  },\n  \"permissions\": {\n    \"allow\": [\n      \"mcp__engram__*\",\n      \"mcp__context7__*\"\n    ]\n  }\n}\n"
	claudeJSON := "{\n  \"mcpServers\": {\n    \"codegraph\": {},\n    \"context7\": {},\n    \"engram\": {}\n  }\n}\n"
	if err := writeFileCreatingDirs(filepath.Join(profileDir, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeFileCreatingDirs(filepath.Join(profileDir, ".claude.json"), []byte(claudeJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	userBin := filepath.Join(home, "user-bin")
	if err := writeFileCreatingDirs(filepath.Join(userBin, "claude-pcs"), []byte("#!/bin/sh\necho user-owned\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, profileDir)
	beforeBin := snapshotTree(t, userBin)

	if _, err := Uninstall(Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: userBin}); err != nil {
		t.Fatalf("Uninstall(profile, no state) error: %v", err)
	}
	assertSnapshotEqual(t, "profile", before, snapshotTree(t, profileDir))
	assertSnapshotEqual(t, "user launcher dir", beforeBin, snapshotTree(t, userBin))
}

func TestProfileTargetOverlappingMainProfileIsRefused(t *testing.T) {
	// relocate replaces the regular file at path with a symlink to a copy of it
	// under dir, so the main profile file is physically inside dir.
	relocate := func(t *testing.T, path, dir string) {
		t.Helper()
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		moved := filepath.Join(dir, filepath.Base(path))
		if err := writeFileCreatingDirs(moved, content, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(moved, path); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	tests := []struct {
		name string
		// opts builds the install options given the isolated root and the home
		// directory nested inside it.
		opts func(t *testing.T, root, home string) Options
	}{
		{
			name: "profile-dir equals the home directory",
			opts: func(t *testing.T, root, home string) Options {
				return Options{Scope: ScopeProfile, HomeDir: home, ProfileDir: home}
			},
		},
		{
			name: "profile-dir equals the home directory with a trailing separator",
			opts: func(t *testing.T, root, home string) Options {
				return Options{Scope: ScopeProfile, HomeDir: home, ProfileDir: home + string(filepath.Separator)}
			},
		},
		{
			name: "profile-dir is an ancestor of the home directory",
			opts: func(t *testing.T, root, home string) Options {
				return Options{Scope: ScopeProfile, HomeDir: home, ProfileDir: filepath.Dir(home)}
			},
		},
		{
			name: "profile-dir is an ancestor of the CLAUDE_CONFIG_DIR main target",
			opts: func(t *testing.T, root, home string) Options {
				custom := filepath.Join(home, "config", "main")
				if err := os.MkdirAll(custom, 0o755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("CLAUDE_CONFIG_DIR", custom)
				return Options{Scope: ScopeProfile, HomeDir: home, ProfileDir: filepath.Join(home, "config")}
			},
		},
		{
			name: "profile .claude.json is the main ~/.claude.json through a symlink",
			opts: func(t *testing.T, root, home string) Options {
				dotfiles := filepath.Join(home, "dotfiles")
				relocate(t, filepath.Join(home, ".claude.json"), dotfiles)
				return Options{Scope: ScopeProfile, HomeDir: home, ProfileDir: dotfiles}
			},
		},
		{
			name: "profile settings.json is the main settings.json through a symlink",
			opts: func(t *testing.T, root, home string) Options {
				dotfiles := filepath.Join(home, "dotfiles")
				relocate(t, filepath.Join(home, ".claude", "settings.json"), dotfiles)
				return Options{Scope: ScopeProfile, HomeDir: home, ProfileDir: dotfiles}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := isolateEnv(t)
			home := filepath.Join(root, "users", "me")
			seedMainProfile(t, home)
			opts := tt.opts(t, root, home)
			before := snapshotTree(t, root)

			if _, err := Install(opts); err == nil {
				t.Errorf("Install(%+v) succeeded, want refusal", opts)
			}
			if _, err := Sync(opts); err == nil {
				t.Errorf("Sync(%+v) succeeded, want refusal", opts)
			}
			if _, err := Uninstall(opts); err == nil {
				t.Errorf("Uninstall(%+v) succeeded, want refusal", opts)
			}
			assertSnapshotEqual(t, "isolated root", before, snapshotTree(t, root))
		})
	}
}

func TestProfileTargetBesideMainProfileIsAllowed(t *testing.T) {
	root := isolateEnv(t)
	home := filepath.Join(root, "users", "me")
	seedMainProfile(t, home)
	for _, dir := range []string{filepath.Join(home, ".claude-pcs"), filepath.Join(home, "profiles", "pcs")} {
		if _, err := Install(Options{Scope: ScopeProfile, HomeDir: home, ProfileDir: dir, LauncherDir: filepath.Join(root, "bin")}); err != nil {
			t.Errorf("Install(profile-dir %s) error: %v", dir, err)
		}
	}
}
