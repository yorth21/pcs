package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yorth21/pcs/internal/mcp"
)

const mainStatusLine = `{"statusLine": {"type": "command", "command": "echo main"}}` + "\n"

// seedMainStatusLine writes a main-profile settings.json carrying a statusLine.
func seedMainStatusLine(t *testing.T, home string) {
	t.Helper()
	if err := writeFileCreatingDirs(filepath.Join(home, ".claude", "settings.json"), []byte(mainStatusLine), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readSettings decodes the profile settings.json (an empty map when missing).
func readSettings(t *testing.T, profileDir string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(profileDir, "settings.json"))
	if os.IsNotExist(err) {
		return map[string]any{}
	}
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode settings.json: %v", err)
	}
	return m
}

func writeSettings(t *testing.T, profileDir string, m map[string]any) {
	t.Helper()
	if err := writeJSONFile(filepath.Join(profileDir, "settings.json"), m); err != nil {
		t.Fatal(err)
	}
}

func mustReadState(t *testing.T, dir string) *State {
	t.Helper()
	s, err := readState(filepath.Join(dir, StateFileName))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	return s
}

func hasAction(rep *Report, op string) bool {
	for _, a := range rep.Actions {
		if a.Operation == op {
			return true
		}
	}
	return false
}

func TestSyncHonorsPersistedNoStatusLine(t *testing.T) {
	home := isolateEnv(t)
	seedMainStatusLine(t, home)
	profileDir := filepath.Join(home, ".claude-pcs")
	base := Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin")}

	install := base
	install.NoStatusLine = true
	if _, err := Install(install); err != nil {
		t.Fatalf("Install(--no-statusline) error: %v", err)
	}
	if !mustReadState(t, profileDir).NoStatusLine {
		t.Fatalf("expected no_statusline to be persisted in %s", StateFileName)
	}

	if _, err := Sync(base); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}
	if _, has := readSettings(t, profileDir)["statusLine"]; has {
		t.Fatalf("sync re-copied statusLine despite persisted --no-statusline")
	}
	if !mustReadState(t, profileDir).NoStatusLine {
		t.Fatalf("sync dropped the persisted no_statusline flag")
	}

	// An explicit --no-statusline=false on the command line overrides the persisted value.
	override := base
	override.FlagOverrides = map[string]bool{"no-statusline": true}
	if _, err := Sync(override); err != nil {
		t.Fatalf("Sync(--no-statusline=false) error: %v", err)
	}
	if _, has := readSettings(t, profileDir)["statusLine"]; !has {
		t.Fatalf("expected explicit --no-statusline=false to copy statusLine")
	}
	if mustReadState(t, profileDir).NoStatusLine {
		t.Fatalf("expected explicit override to persist no_statusline=false")
	}
}

func TestSyncHonorsPersistedWithPlugins(t *testing.T) {
	home := isolateEnv(t)
	profileDir := filepath.Join(home, ".claude-pcs")
	base := Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin")}

	install := base
	install.WithPlugins = true
	if _, err := Install(install); err != nil {
		t.Fatalf("Install(--with-plugins) error: %v", err)
	}
	if _, err := Sync(base); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}
	if !mustReadState(t, profileDir).WithPlugins {
		t.Fatalf("sync dropped the persisted with_plugins flag")
	}

	override := base
	override.FlagOverrides = map[string]bool{"with-plugins": true}
	if _, err := Sync(override); err != nil {
		t.Fatalf("Sync(--with-plugins=false) error: %v", err)
	}
	if mustReadState(t, profileDir).WithPlugins {
		t.Fatalf("expected explicit --with-plugins=false to persist with_plugins=false")
	}
}

func TestLegacyStateWithoutFlagFieldsKeepsCurrentBehavior(t *testing.T) {
	home := isolateEnv(t)
	seedMainStatusLine(t, home)
	profileDir := filepath.Join(home, ".claude-pcs")
	opts := Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin")}
	if _, err := Install(opts); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	// Rewrite the state as a legacy file: no flag fields and no statusLine digest.
	statePath := filepath.Join(profileDir, StateFileName)
	b, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"no_statusline", "with_plugins", "status_line_sha256"} {
		delete(raw, k)
	}
	if raw["added_status_line"] != true {
		t.Fatalf("expected install to record added_status_line, got %v", raw["added_status_line"])
	}
	if err := writeJSONFile(statePath, raw); err != nil {
		t.Fatal(err)
	}

	// Legacy sync still copies a missing statusLine: added_status_line proves the install did not opt out.
	settings := readSettings(t, profileDir)
	delete(settings, "statusLine")
	writeSettings(t, profileDir, settings)
	if _, err := Sync(opts); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}
	if _, has := readSettings(t, profileDir)["statusLine"]; !has {
		t.Fatalf("expected legacy sync to copy statusLine")
	}

	// Legacy uninstall (no digest) removes the statusLine still equal to the main profile's.
	raw2 := map[string]any{}
	b, _ = os.ReadFile(statePath)
	_ = json.Unmarshal(b, &raw2)
	delete(raw2, "status_line_sha256")
	if err := writeJSONFile(statePath, raw2); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(opts); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if _, has := readSettings(t, profileDir)["statusLine"]; has {
		t.Fatalf("expected legacy uninstall to remove the unchanged statusLine")
	}
}

func TestUninstallStatusLineOwnership(t *testing.T) {
	tests := []struct {
		name       string
		edit       bool
		wantKept   bool
		wantAction string
	}{
		{name: "unchanged statusLine is removed", edit: false, wantKept: false, wantAction: "remove-statusline"},
		{name: "user-edited statusLine is kept and reported", edit: true, wantKept: true, wantAction: "keep-statusline"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateEnv(t)
			seedMainStatusLine(t, home)
			profileDir := filepath.Join(home, ".claude-pcs")
			opts := Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin")}
			if _, err := Install(opts); err != nil {
				t.Fatalf("Install() error: %v", err)
			}
			if mustReadState(t, profileDir).StatusLineSHA256 == "" {
				t.Fatalf("expected install to record the statusLine digest")
			}
			if tt.edit {
				settings := readSettings(t, profileDir)
				settings["statusLine"] = map[string]any{"type": "command", "command": "echo mine"}
				writeSettings(t, profileDir, settings)
			}

			rep, err := Uninstall(opts)
			if err != nil {
				t.Fatalf("Uninstall() error: %v", err)
			}
			_, has := readSettings(t, profileDir)["statusLine"]
			if has != tt.wantKept {
				t.Fatalf("statusLine present after uninstall = %v, want %v", has, tt.wantKept)
			}
			if !hasAction(rep, tt.wantAction) {
				t.Fatalf("expected %q action in uninstall report, got %+v", tt.wantAction, rep.Actions)
			}
			if tt.wantKept && !strings.Contains(strings.Join(rep.Warnings, "\n"), "statusLine") {
				t.Fatalf("expected a warning explaining the kept statusLine, got %v", rep.Warnings)
			}
		})
	}
}

func TestUninstallKeepsPrePCSBackups(t *testing.T) {
	home := isolateEnv(t)
	globalDir := filepath.Join(home, ".claude")
	claudeMD := filepath.Join(globalDir, "CLAUDE.md")
	claudeJSON := filepath.Join(home, ".claude.json")
	settingsJSON := filepath.Join(globalDir, "settings.json")
	seed := map[string]string{
		claudeMD:     "# user notes\n",
		claudeJSON:   `{"mcpServers": {"mine": {"command": "x"}}}` + "\n",
		settingsJSON: `{"theme": "dark"}` + "\n",
	}
	for p, c := range seed {
		if err := writeFileCreatingDirs(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	opts := Options{Scope: ScopeGlobal, TargetDir: globalDir, HomeDir: home, WithMCP: true}
	if _, err := Install(opts); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	rep, err := Uninstall(Options{Scope: ScopeGlobal, TargetDir: globalDir, HomeDir: home, RemoveMCP: true})
	if err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	for p, original := range seed {
		bak := p + mcp.PrePCSBackupSuffix
		got, err := os.ReadFile(bak)
		if err != nil {
			t.Fatalf("expected %s to be kept after uninstall: %v", bak, err)
		}
		if string(got) != original {
			t.Errorf("%s content = %q, want %q", bak, got, original)
		}
		found := false
		for _, a := range rep.Actions {
			if a.Operation == "keep-backup" && a.Path == bak {
				found = true
			}
		}
		if !found {
			t.Errorf("expected keep-backup action for %s in uninstall report", bak)
		}
	}
}
