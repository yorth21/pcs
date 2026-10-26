package installer

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// fakeClaudeEnv switches the test binary into fake-claude mode: when a launcher
// runs a copy of this binary as "claude", TestMain prints CLAUDE_CONFIG_DIR and
// then every argument it received, one per line, and exits.
const fakeClaudeEnv = "PCS_TEST_FAKE_CLAUDE"

func TestMain(m *testing.M) {
	if os.Getenv(fakeClaudeEnv) == "1" {
		fmt.Println("CLAUDE_CONFIG_DIR=" + os.Getenv("CLAUDE_CONFIG_DIR"))
		for _, a := range os.Args[1:] {
			fmt.Println(a)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// installFakeClaude copies the running test binary into a fresh directory as
// the "claude" executable and returns that directory.
func installFakeClaude(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	dir := t.TempDir()
	name := "claude"
	if runtime.GOOS == "windows" {
		name = "claude.exe"
	}
	src, err := os.Open(self)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

// installLauncherForExec installs a profile with the given launcher args into an
// isolated home whose PATH holds only the fake claude (plus the directories of
// any extra tools the launcher syntax needs). It returns the launcher directory,
// profile directory, and shared MCP path.
func installLauncherForExec(t *testing.T, launcherArgs string, extraTools ...string) (launcherDir, profileDir, sharedMCP string) {
	t.Helper()
	toolDirs := make([]string, 0, len(extraTools))
	for _, tool := range extraTools {
		p, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
		toolDirs = append(toolDirs, filepath.Dir(p))
	}
	home := isolateEnv(t)
	fakeDir := installFakeClaude(t)
	t.Setenv("PATH", strings.Join(append([]string{fakeDir}, toolDirs...), string(os.PathListSeparator)))
	t.Setenv(fakeClaudeEnv, "1")

	// The apostrophe, percent sign, and spaces exercise path quoting in every launcher syntax.
	profileDir = filepath.Join(home, "O'Reilly 100% profile", ".claude-pcs")
	launcherDir = filepath.Join(home, "bin with spaces")
	if _, err := Install(Options{
		Scope:        ScopeProfile,
		TargetDir:    profileDir,
		HomeDir:      home,
		LauncherDir:  launcherDir,
		LauncherArgs: launcherArgs,
	}); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	return launcherDir, profileDir, filepath.Join(profileDir, "shared-mcp.json")
}

func parseFakeClaudeOutput(t *testing.T, out []byte) (configDir string, argv []string) {
	t.Helper()
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "CLAUDE_CONFIG_DIR=") {
		t.Fatalf("unexpected fake claude output:\n%s", out)
	}
	return strings.TrimPrefix(lines[0], "CLAUDE_CONFIG_DIR="), lines[1:]
}

// launcherArgsWithShellMetacharacters exercises $, backtick, single quote,
// percent, caret, ampersand, pipe, and spaces inside grouped launcher args.
const launcherArgsWithShellMetacharacters = `--append-system-prompt "$HOME ` + "`echo pwned`" + ` it's 100% ^done & more|less" --verbose`

var wantLauncherArgs = []string{"--append-system-prompt", "$HOME `echo pwned` it's 100% ^done & more|less", "--verbose"}

func assertLauncherArgv(t *testing.T, out []byte, profileDir, sharedMCP string) {
	t.Helper()
	configDir, argv := parseFakeClaudeOutput(t, out)
	if configDir != profileDir {
		t.Errorf("CLAUDE_CONFIG_DIR = %q, want %q", configDir, profileDir)
	}
	want := append(append([]string{}, wantLauncherArgs...), "hello world", "--resume", "x", "--mcp-config", sharedMCP)
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("claude argv mismatch\ngot:  %q\nwant: %q", argv, want)
	}
}

func TestPOSIXLauncherPassesArgsLiterallyWithMCPConfigLast(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX launcher execution test runs on non-Windows only")
	}
	launcherDir, profileDir, sharedMCP := installLauncherForExec(t, launcherArgsWithShellMetacharacters, "sh", "env")

	out, err := exec.Command(filepath.Join(launcherDir, "claude-pcs"), "hello world", "--resume", "x").CombinedOutput()
	if err != nil {
		t.Fatalf("run POSIX launcher: %v\n%s", err, out)
	}
	assertLauncherArgv(t, out, profileDir, sharedMCP)
}

func TestCmdLauncherPassesArgsLiterallyWithMCPConfigLast(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cmd launcher execution test runs on Windows only")
	}
	launcherDir, profileDir, sharedMCP := installLauncherForExec(t, launcherArgsWithShellMetacharacters, "cmd")

	out, err := exec.Command(filepath.Join(launcherDir, "claude-pcs.cmd"), "hello world", "--resume", "x").CombinedOutput()
	if err != nil {
		t.Fatalf("run cmd launcher: %v\n%s", err, out)
	}
	assertLauncherArgv(t, out, profileDir, sharedMCP)
}

func TestPowerShellLauncherPassesArgsLiterallyWithMCPConfigLast(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell launcher execution test runs on Windows only")
	}
	shell := ""
	for _, candidate := range []string{"pwsh", "powershell"} {
		if _, err := exec.LookPath(candidate); err == nil {
			shell = candidate
			break
		}
	}
	if shell == "" {
		t.Skip("neither pwsh nor powershell is available")
	}
	launcherDir, profileDir, sharedMCP := installLauncherForExec(t, launcherArgsWithShellMetacharacters, shell)

	out, err := exec.Command(shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-File", filepath.Join(launcherDir, "claude-pcs.ps1"), "hello world", "--resume", "x").CombinedOutput()
	if err != nil {
		t.Fatalf("run PowerShell launcher: %v\n%s", err, out)
	}
	assertLauncherArgv(t, out, profileDir, sharedMCP)
}

func TestSplitLauncherArgs(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{name: "empty input yields no args", in: "   ", want: nil},
		{name: "whitespace separates words", in: " --model  opus\t--verbose ", want: []string{"--model", "opus", "--verbose"}},
		{name: "double quotes group words", in: `--append-system-prompt "be brief"`, want: []string{"--append-system-prompt", "be brief"}},
		{name: "single quotes group words and keep double quotes", in: `--x 'say "hi"'`, want: []string{"--x", `say "hi"`}},
		{name: "double quotes keep single quotes", in: `"it's"`, want: []string{"it's"}},
		{name: "quotes join with adjacent text", in: `--add-dir="C:\My Dir"`, want: []string{`--add-dir=C:\My Dir`}},
		{name: "backslashes are literal", in: `C:\tools\bin`, want: []string{`C:\tools\bin`}},
		{name: "empty quotes yield an empty arg", in: `--x ""`, want: []string{"--x", ""}},
		{name: "shell metacharacters stay literal", in: "$HOME `id` %PATH% ^", want: []string{"$HOME", "`id`", "%PATH%", "^"}},
		{name: "unterminated double quote is an error", in: `--x "abc`, wantErr: true},
		{name: "unterminated single quote is an error", in: `--x 'abc`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitLauncherArgs(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SplitLauncherArgs(%q) = %q, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("SplitLauncherArgs(%q) error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitLauncherArgs(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLauncherQuotingHelpers(t *testing.T) {
	tests := []struct {
		name  string
		quote func(string) string
		in    string
		want  string
	}{
		{name: "posix plain", quote: posixQuote, in: "/home/u/.claude-pcs", want: `'/home/u/.claude-pcs'`},
		{name: "posix empty", quote: posixQuote, in: "", want: `''`},
		{name: "posix single quote", quote: posixQuote, in: "it's", want: `'it'\''s'`},
		{name: "posix dollar and backtick stay literal", quote: posixQuote, in: "$HOME `id`", want: "'$HOME `id`'"},
		{name: "powershell single quote", quote: powerShellQuote, in: "O'Reilly", want: `'O''Reilly'`},
		{name: "powershell typographic quotes doubled", quote: powerShellQuote, in: "a\u2019b", want: "'a\u2019\u2019b'"},
		{name: "powershell dollar stays literal", quote: powerShellQuote, in: "$env:X", want: `'$env:X'`},
		{name: "cmd plain path", quote: cmdQuote, in: `C:\Users\u\.claude-pcs`, want: `"C:\Users\u\.claude-pcs"`},
		{name: "cmd percent doubled", quote: cmdQuote, in: "100%PATH%", want: `"100%%PATH%%"`},
		{name: "cmd caret ampersand literal inside quotes", quote: cmdQuote, in: "a^b&c", want: `"a^b&c"`},
		{name: "cmd embedded double quotes escaped for program and cmd", quote: cmdQuote, in: `say "hi"`, want: `"say \"hi\^"^"`},
		{name: "cmd metacharacters caret-escaped once outside cmd quotes", quote: cmdQuote, in: `a&"b&c^`, want: `"a&\"b^&c^^^"`},
		{name: "cmd backslashes before quote doubled plus escape", quote: cmdQuote, in: `a\"b`, want: `"a\\\"b^"`},
		{name: "cmd trailing backslashes doubled", quote: cmdQuote, in: `C:\dir\`, want: `"C:\dir\\"`},
		{name: "cmd empty", quote: cmdQuote, in: "", want: `""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.quote(tt.in); got != tt.want {
				t.Errorf("quote(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestRenderLaunchersPlaceMCPConfigLast(t *testing.T) {
	scripts := renderLaunchers("/p/.claude-pcs", "/p/.claude-pcs/shared-mcp.json", []string{"--verbose"})
	checks := []struct {
		name, script, userArgs, mcp string
	}{
		{"posix", scripts.posix, `"$@"`, `--mcp-config '/p/.claude-pcs/shared-mcp.json'`},
		{"cmd", scripts.cmd, "%*", `--mcp-config "/p/.claude-pcs/shared-mcp.json"`},
		{"powershell", scripts.powerShell, "@args", `--mcp-config '/p/.claude-pcs/shared-mcp.json'`},
	}
	for _, c := range checks {
		userIdx := strings.Index(c.script, c.userArgs)
		mcpIdx := strings.Index(c.script, c.mcp)
		if userIdx < 0 || mcpIdx < 0 || mcpIdx < userIdx {
			t.Errorf("%s launcher must pass user args before %s:\n%s", c.name, c.mcp, c.script)
		}
		if !strings.Contains(c.script, "--verbose") {
			t.Errorf("%s launcher missing launcher arg:\n%s", c.name, c.script)
		}
	}
}

func TestInstallRejectsUnbalancedLauncherArgs(t *testing.T) {
	home := isolateEnv(t)
	profileDir := filepath.Join(home, ".claude-pcs")
	_, err := Install(Options{Scope: ScopeProfile, TargetDir: profileDir, HomeDir: home, LauncherArgs: `--x "open`})
	if err == nil || !strings.Contains(err.Error(), "launcher-args") {
		t.Fatalf("Install() error = %v, want launcher-args parse error", err)
	}
	if _, statErr := os.Stat(profileDir); !os.IsNotExist(statErr) {
		t.Errorf("expected no profile directory after rejected launcher args, stat err = %v", statErr)
	}
}
