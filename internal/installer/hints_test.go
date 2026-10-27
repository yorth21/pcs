package installer

import (
	"errors"
	"path/filepath"
	"regexp"
	"runtime"
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

// assertNoPersistentConfigDir fails when hint can leave CLAUDE_CONFIG_DIR set
// in the caller's shell: every assignment must be scoped to one command
// (POSIX prefix), to a cmd /c child process, or to a PowerShell block that
// restores the previous value in finally.
func assertNoPersistentConfigDir(t *testing.T, goos, hint string) {
	t.Helper()
	if strings.Contains(hint, "export CLAUDE_CONFIG_DIR") || strings.Contains(hint, "setx") {
		t.Errorf("%s hint exports CLAUDE_CONFIG_DIR persistently: %q", goos, hint)
	}
	if goos == "windows" {
		psAssign := regexp.MustCompile(`\$env:CLAUDE_CONFIG_DIR\s*=\s*'`)
		psScoped := regexp.MustCompile(`& \{ \$prev = \$env:CLAUDE_CONFIG_DIR; \$env:CLAUDE_CONFIG_DIR = '(?:[^']|'')*'; try \{ [^{}]* \} finally \{ \$env:CLAUDE_CONFIG_DIR = \$prev \} \}`)
		if a, s := len(psAssign.FindAllString(hint, -1)), len(psScoped.FindAllString(hint, -1)); a != s {
			t.Errorf("windows hint has %d PowerShell CLAUDE_CONFIG_DIR assignments but only %d scoped blocks: %q", a, s, hint)
		}
		cmdSet := regexp.MustCompile(`(?i)set "?CLAUDE_CONFIG_DIR=`)
		cmdScoped := regexp.MustCompile(`cmd /s /c "set "CLAUDE_CONFIG_DIR=[^"]*" && `)
		if a, s := len(cmdSet.FindAllString(hint, -1)), len(cmdScoped.FindAllString(hint, -1)); a != s {
			t.Errorf("windows hint has %d cmd.exe CLAUDE_CONFIG_DIR assignments but only %d inside cmd /s /c: %q", a, s, hint)
		}
		return
	}
	assign := regexp.MustCompile(`CLAUDE_CONFIG_DIR=`)
	prefix := regexp.MustCompile(`CLAUDE_CONFIG_DIR='(?:[^']|'\\'')*' claude `)
	if a, s := len(assign.FindAllString(hint, -1)), len(prefix.FindAllString(hint, -1)); a != s {
		t.Errorf("%s hint has %d CLAUDE_CONFIG_DIR assignments but only %d single-command prefixes: %q", goos, a, s, hint)
	}
}

func TestProfileClaudeCommandScopesTheConfigDir(t *testing.T) {
	steps := requiredPluginSteps()
	tests := []struct {
		name       string
		goos       string
		profileDir string
		want       []string
	}{
		{
			name:       "windows PowerShell restores the previous value and cmd.exe uses a child process",
			goos:       "windows",
			profileDir: `C:\Users\O'Brien\.claude-pcs`,
			want: []string{
				`& { $prev = $env:CLAUDE_CONFIG_DIR; $env:CLAUDE_CONFIG_DIR = 'C:\Users\O''Brien\.claude-pcs'; try { claude plugin marketplace add ` + EngramMarketplaceSource + `; claude plugin install engram@engram } finally { $env:CLAUDE_CONFIG_DIR = $prev } }`,
				`cmd /s /c "set "CLAUDE_CONFIG_DIR=C:\Users\O'Brien\.claude-pcs" && claude plugin marketplace add ` + EngramMarketplaceSource + ` & claude plugin install engram@engram"`,
			},
		},
		{
			name:       "linux prefixes every command",
			goos:       "linux",
			profileDir: "/home/o'brien/.claude-pcs",
			want: []string{
				`CLAUDE_CONFIG_DIR='/home/o'\''brien/.claude-pcs' claude plugin marketplace add ` + EngramMarketplaceSource + `; CLAUDE_CONFIG_DIR='/home/o'\''brien/.claude-pcs' claude plugin install engram@engram`,
			},
		},
		{
			name:       "darwin uses the posix form",
			goos:       "darwin",
			profileDir: "/Users/me/pcs profile",
			want:       []string{`CLAUDE_CONFIG_DIR='/Users/me/pcs profile' claude plugin install engram@engram`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := profileClaudeCommand(tt.goos, tt.profileDir, steps...)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("profileClaudeCommand(%s) = %q, want it to contain %q", tt.goos, got, w)
				}
			}
			assertNoPersistentConfigDir(t, tt.goos, got)
		})
	}
}

func TestLaunchHintIsTheLauncher(t *testing.T) {
	tests := []struct {
		name   string
		goos   string
		dir    string
		onPath bool
		want   string
	}{
		{name: "windows launcher on PATH", goos: "windows", dir: `C:\pcs\bin`, onPath: true, want: "claude-pcs"},
		{name: "windows launcher off PATH runs the cmd launcher by absolute path", goos: "windows", dir: `C:\Users\O'Brien\pcs\bin\`, want: `& 'C:\Users\O''Brien\pcs\bin\claude-pcs.cmd'`},
		{name: "linux launcher on PATH", goos: "linux", dir: "/home/me/.local/bin", onPath: true, want: "claude-pcs"},
		{name: "linux launcher off PATH", goos: "linux", dir: "/home/o'brien/bin/", want: `'/home/o'\''brien/bin/claude-pcs'`},
		{name: "darwin launcher off PATH", goos: "darwin", dir: "/Users/me/my bin", want: `'/Users/me/my bin/claude-pcs'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := launchHint(tt.goos, tt.dir, tt.onPath)
			if got != tt.want {
				t.Errorf("launchHint(%s, %q, %v) = %q, want %q", tt.goos, tt.dir, tt.onPath, got, tt.want)
			}
			if strings.Contains(got, "CLAUDE_CONFIG_DIR") {
				t.Errorf("launch hint sets CLAUDE_CONFIG_DIR: %q", got)
			}
		})
	}
}

func TestNoHintLeavesClaudeConfigDirSet(t *testing.T) {
	cases := []struct {
		goos, profileDir, launcherDir string
	}{
		{"windows", `C:\Users\me\.claude-pcs`, `C:\Users\me\AppData\Local\pcs\bin`},
		{"darwin", "/Users/me/.claude-pcs", "/Users/me/.local/bin"},
		{"linux", "/home/me/.claude-pcs", "/home/me/.local/bin"},
	}
	for _, c := range cases {
		t.Run(c.goos, func(t *testing.T) {
			var hints []string
			for _, onPath := range []bool{true, false} {
				hints = append(hints, launchHint(c.goos, c.launcherDir, onPath))
				for _, withPlugins := range []bool{true, false} {
					hints = append(hints, profileNextSteps(c.goos, c.profileDir, c.launcherDir, onPath, withPlugins)...)
				}
			}
			for _, args := range requiredPluginSteps() {
				hints = append(hints, profileClaudeCommand(c.goos, c.profileDir, args))
			}
			hints = append(hints, PluginInstallCommand(c.goos, c.profileDir))
			for _, h := range hints {
				assertNoPersistentConfigDir(t, c.goos, h)
			}
		})
	}
}

func TestProfileNextStepsArePerOS(t *testing.T) {
	win := strings.Join(profileNextSteps("windows", `C:\prof`, `C:\bin`, false, false), "\n")
	for _, w := range []string{"$env:CLAUDE_CONFIG_DIR = 'C:\\prof'; try { claude plugin marketplace add " + EngramMarketplaceSource, "claude plugin install engram@engram", "[Environment]::SetEnvironmentVariable"} {
		if !strings.Contains(win, w) {
			t.Errorf("windows next steps missing %q:\n%s", w, win)
		}
	}
	if strings.Contains(win, "CLAUDE_CONFIG_DIR='") || strings.Contains(win, "setx") {
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

func TestProfileInstallReportsTheLauncherAsTheLaunchCommand(t *testing.T) {
	home := isolateEnv(t)
	launcherDir := filepath.Join(home, "bin")
	rep, err := Install(Options{Scope: ScopeProfile, HomeDir: home, LauncherDir: launcherDir, LauncherArgs: "--model opus"})
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if rep.LauncherOnPath {
		t.Fatalf("isolated launcher dir %q unexpectedly on PATH", launcherDir)
	}
	if want := launchHint(runtime.GOOS, launcherDir, false); rep.LauncherCmd != want {
		t.Errorf("LauncherCmd = %q, want %q", rep.LauncherCmd, want)
	}
	if strings.Contains(rep.LauncherCmd, "CLAUDE_CONFIG_DIR") || strings.Contains(rep.LauncherCmd, "--mcp-config") {
		t.Errorf("LauncherCmd bypasses the launcher: %q", rep.LauncherCmd)
	}
}
