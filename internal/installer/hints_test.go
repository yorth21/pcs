package installer

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPathHintIsPerOS(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		dir     string
		want    []string
		notWant []string
	}{
		{
			name: "windows appends only to the user PATH via PowerShell",
			goos: "windows",
			dir:  `C:\Users\O'Brien\AppData\Local\pcs\bin`,
			want: []string{
				"$p = [Environment]::GetEnvironmentVariable('Path', 'User')",
				"[Environment]::SetEnvironmentVariable('Path', \"$p;\" + 'C:\\Users\\O''Brien\\AppData\\Local\\pcs\\bin', 'User')",
			},
			notWant: []string{"setx", "%PATH%"},
		},
		{
			name:    "posix exports a single-quoted directory",
			goos:    "linux",
			dir:     "/home/o'brien/.local/bin",
			want:    []string{`export PATH='/home/o'\''brien/.local/bin':"$PATH"`},
			notWant: []string{"setx", "[Environment]"},
		},
		{
			name: "darwin uses the posix form",
			goos: "darwin",
			dir:  "/Users/me/bin with space",
			want: []string{`export PATH='/Users/me/bin with space':"$PATH"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathHint(tt.goos, tt.dir)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("pathHint(%s) = %q, want it to contain %q", tt.goos, got, w)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(got, nw) {
					t.Errorf("pathHint(%s) = %q, must not contain %q", tt.goos, got, nw)
				}
			}
		})
	}
}

func TestProfileClaudeCommandIsPerOS(t *testing.T) {
	args := []string{"plugin", "install", EngramPlugin}
	if got, want := profileClaudeCommand("windows", `C:\p'x`, args), `$env:CLAUDE_CONFIG_DIR = 'C:\p''x'; claude plugin install engram@engram`; got != want {
		t.Errorf("windows command = %q, want %q", got, want)
	}
	if got, want := profileClaudeCommand("linux", "/p'x", args), `CLAUDE_CONFIG_DIR='/p'\''x' claude plugin install engram@engram`; got != want {
		t.Errorf("posix command = %q, want %q", got, want)
	}
}

func TestProfileNextStepsArePerOS(t *testing.T) {
	win := strings.Join(profileNextSteps("windows", `C:\prof`, `C:\bin`, false, false), "\n")
	for _, w := range []string{"$env:CLAUDE_CONFIG_DIR = 'C:\\prof'; claude plugin marketplace add " + EngramMarketplaceSource, "claude plugin install engram@engram", "[Environment]::SetEnvironmentVariable"} {
		if !strings.Contains(win, w) {
			t.Errorf("windows next steps missing %q:\n%s", w, win)
		}
	}
	if strings.Contains(win, "CLAUDE_CONFIG_DIR=") || strings.Contains(win, "setx") {
		t.Errorf("windows next steps contain POSIX or setx syntax:\n%s", win)
	}

	posix := strings.Join(profileNextSteps("linux", "/prof", "/bin", true, false), "\n")
	if !strings.Contains(posix, "CLAUDE_CONFIG_DIR='/prof' claude plugin marketplace add "+EngramMarketplaceSource) {
		t.Errorf("posix next steps missing marketplace command:\n%s", posix)
	}
	if strings.Contains(posix, "export PATH") {
		t.Errorf("posix next steps suggest a PATH change although the launcher dir is on PATH:\n%s", posix)
	}

	if withPlugins := strings.Join(profileNextSteps("linux", "/prof", "/bin", true, true), "\n"); strings.Contains(withPlugins, "plugin install") {
		t.Errorf("next steps repeat the plugin commands although --with-plugins ran them:\n%s", withPlugins)
	}
}

type claudeCall struct {
	env  []string
	args []string
}

// fakeClaude replaces runClaude for the test; fail maps a joined argument list
// to the error it returns.
func fakeClaude(t *testing.T, fail map[string]error) *[]claudeCall {
	t.Helper()
	var calls []claudeCall
	orig := runClaude
	runClaude = func(env []string, args ...string) ([]byte, error) {
		calls = append(calls, claudeCall{env: slices.Clone(env), args: slices.Clone(args)})
		if err := fail[strings.Join(args, " ")]; err != nil {
			return []byte("boom output\n"), err
		}
		return nil, nil
	}
	t.Cleanup(func() { runClaude = orig })
	return &calls
}

func TestWithPluginsAddsMarketplaceFirstAndSurfacesFailures(t *testing.T) {
	addCmd := "plugin marketplace add " + EngramMarketplaceSource
	installCmd := "plugin install " + EngramPlugin
	tests := []struct {
		name         string
		fail         map[string]error
		wantWarnings []string
	}{
		{name: "both steps succeed", fail: nil},
		{
			name:         "marketplace already present: add fails but install succeeds",
			fail:         map[string]error{addCmd: errors.New("exit status 1")},
			wantWarnings: nil,
		},
		{
			name: "install fails: every failed step is reported with its manual command",
			fail: map[string]error{addCmd: errors.New("exit status 1"), installCmd: errors.New("exit status 2")},
			wantWarnings: []string{
				"claude " + addCmd,
				"claude " + installCmd,
				"boom output",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateEnv(t)
			calls := fakeClaude(t, tt.fail)
			profileDir := filepath.Join(home, ".claude-pcs")
			rep, err := Install(Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin"), WithPlugins: true})
			if err != nil {
				t.Fatalf("Install(--with-plugins) error: %v", err)
			}
			if len(*calls) != 2 {
				t.Fatalf("expected 2 claude calls, got %+v", *calls)
			}
			if got := strings.Join((*calls)[0].args, " "); got != addCmd {
				t.Errorf("first claude call = %q, want %q", got, addCmd)
			}
			if got := strings.Join((*calls)[1].args, " "); got != installCmd {
				t.Errorf("second claude call = %q, want %q", got, installCmd)
			}
			for _, c := range *calls {
				if !slices.Contains(c.env, "CLAUDE_CONFIG_DIR="+profileDir) {
					t.Errorf("claude call %v ran without the profile env, got %v", c.args, c.env)
				}
			}
			warnings := strings.Join(rep.Warnings, "\n")
			if len(tt.wantWarnings) == 0 && len(rep.Warnings) != 0 {
				t.Errorf("expected no warnings, got %v", rep.Warnings)
			}
			for _, w := range tt.wantWarnings {
				if !strings.Contains(warnings, w) {
					t.Errorf("warnings missing %q:\n%s", w, warnings)
				}
			}
			if len(tt.wantWarnings) > 0 && !strings.Contains(warnings, "CLAUDE_CONFIG_DIR") {
				t.Errorf("warnings lack the exact manual command:\n%s", warnings)
			}
		})
	}
}

func TestWithPluginsDryRunDoesNotRunClaude(t *testing.T) {
	home := isolateEnv(t)
	calls := fakeClaude(t, nil)
	rep, err := Install(Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: filepath.Join(home, "bin"), WithPlugins: true, DryRun: true})
	if err != nil {
		t.Fatalf("Install(--dry-run) error: %v", err)
	}
	if len(*calls) != 0 {
		t.Errorf("dry-run invoked claude: %+v", *calls)
	}
	if !hasAction(rep, "install-plugins") {
		t.Errorf("expected planned install-plugins action, got %+v", rep.Actions)
	}
}
