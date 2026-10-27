package installer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	// EngramMarketplaceSource is the GitHub repository that hosts the engram
	// plugin marketplace; it must be added to a profile before its plugin.
	EngramMarketplaceSource = "Gentleman-Programming/engram"
	// EngramProjectURL documents how to install the engram binary.
	EngramProjectURL = "https://github.com/" + EngramMarketplaceSource
	// EngramPlugin is the plugin identifier installed by --with-plugins.
	EngramPlugin = "engram@engram"
)

// runClaude runs the claude CLI with extra environment entries appended to the
// current environment and returns its combined output. Tests replace it so the
// real claude is never invoked.
var runClaude = func(extraEnv []string, args ...string) ([]byte, error) {
	cmd := exec.Command("claude", args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	return cmd.CombinedOutput()
}

// requiredPluginSteps returns the claude CLI argument lists that install the
// required plugins into a profile, in order: add the marketplace, then install.
func requiredPluginSteps() [][]string {
	return [][]string{
		{"plugin", "marketplace", "add", EngramMarketplaceSource},
		{"plugin", "install", EngramPlugin},
	}
}

// installRequiredPlugins runs every required plugin step with CLAUDE_CONFIG_DIR
// set to profileDir. A failed marketplace add is harmless when the final
// install succeeds (the marketplace may already exist), so failures are
// reported only when the plugin install itself fails: one warning per failed
// step, each with the exact manual command for goos.
func installRequiredPlugins(goos, profileDir string) []string {
	env := []string{"CLAUDE_CONFIG_DIR=" + profileDir}
	steps := requiredPluginSteps()
	var failures []string
	installFailed := false
	for i, args := range steps {
		out, err := runClaude(env, args...)
		if err == nil {
			continue
		}
		if i == len(steps)-1 {
			installFailed = true
		}
		reason := err.Error()
		if detail := strings.TrimSpace(string(out)); detail != "" {
			reason += ": " + detail
		}
		failures = append(failures, fmt.Sprintf("'claude %s' failed (%s); run it manually: %s",
			strings.Join(args, " "), reason, profileClaudeCommand(goos, profileDir, args)))
	}
	if !installFailed {
		return nil
	}
	return failures
}

// profileClaudeCommand renders one copy-pasteable command line that runs claude
// once per args list (in order) with CLAUDE_CONFIG_DIR scoped to profileDir, so
// the variable never stays set in the caller's shell. Every args list must hold
// shell-safe words.
//
//   - POSIX sh: each command gets its own CLAUDE_CONFIG_DIR='...' prefix.
//   - Windows: a PowerShell block that restores the previous value in finally,
//     plus the cmd.exe equivalent running in a cmd /s /c child process. The
//     quoted set "VAR=value" form keeps trailing spaces out of the value;
//     Windows paths cannot contain the double quote that would end it.
func profileClaudeCommand(goos, profileDir string, steps ...[]string) string {
	cmds := make([]string, len(steps))
	for i, args := range steps {
		cmds[i] = "claude " + strings.Join(args, " ")
	}
	if goos == "windows" {
		powerShell := fmt.Sprintf("& { $prev = $env:CLAUDE_CONFIG_DIR; $env:CLAUDE_CONFIG_DIR = %s; try { %s } finally { $env:CLAUDE_CONFIG_DIR = $prev } }",
			powerShellQuote(profileDir), strings.Join(cmds, "; "))
		cmdExe := fmt.Sprintf(`cmd /s /c "set "CLAUDE_CONFIG_DIR=%s" && %s"`, profileDir, strings.Join(cmds, " & "))
		return "PowerShell: " + powerShell + ", or cmd.exe: " + cmdExe
	}
	prefixed := make([]string, len(cmds))
	for i, c := range cmds {
		prefixed[i] = "CLAUDE_CONFIG_DIR=" + posixQuote(profileDir) + " " + c
	}
	return strings.Join(prefixed, "; ")
}

// PluginInstallCommand renders the manual command, in goos's shell syntax,
// that installs the required plugins into the profile at profileDir without
// leaving CLAUDE_CONFIG_DIR set in the caller's shell.
func PluginInstallCommand(goos, profileDir string) string {
	return profileClaudeCommand(goos, profileDir, requiredPluginSteps()...)
}

// launchHint returns the command that starts an isolated profile session: the
// claude-pcs launcher itself, which scopes CLAUDE_CONFIG_DIR and passes the
// stored launcher arguments and shared-mcp.json. When launcherDir is not on
// PATH it is the launcher's absolute path, quoted for goos's default shell
// (on Windows, PowerShell running the cmd launcher, which no execution policy
// blocks).
func launchHint(goos, launcherDir string, onPath bool) string {
	if onPath || launcherDir == "" {
		return "claude-pcs"
	}
	if goos == "windows" {
		return "& " + powerShellQuote(strings.TrimRight(launcherDir, `\/`)+`\claude-pcs.cmd`)
	}
	return posixQuote(strings.TrimRight(launcherDir, "/") + "/claude-pcs")
}

// pathHint returns the instruction that puts dir on PATH. On Windows it appends
// dir to the user PATH only, through .NET in PowerShell: setx would truncate
// PATH at 1024 characters and copy the system PATH into the user PATH.
func pathHint(goos, dir string) string {
	if goos == "windows" {
		return fmt.Sprintf("Add launcher dir to your user PATH (PowerShell), then open a new terminal: $p = [Environment]::GetEnvironmentVariable('Path', 'User'); [Environment]::SetEnvironmentVariable('Path', \"$p;\" + %s, 'User')", powerShellQuote(dir))
	}
	return fmt.Sprintf("Add launcher dir to PATH: export PATH=%s:\"$PATH\"", posixQuote(dir))
}

// profileNextSteps returns the next steps printed after a profile install or
// sync, using goos's shell syntax for every command.
func profileNextSteps(goos, profileDir, launcherDir string, launcherOnPath, withPlugins bool) []string {
	steps := []string{"Profile ready! Run 'claude-pcs' and type '/login' to authenticate this profile."}
	if !withPlugins {
		steps = append(steps, "To install plugins into this profile, re-run 'pcs sync' with --with-plugins, or run: "+PluginInstallCommand(goos, profileDir))
	}
	if !launcherOnPath {
		steps = append(steps, pathHint(goos, launcherDir))
	}
	return steps
}
