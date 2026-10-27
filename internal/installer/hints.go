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

// profileClaudeCommand renders a copy-pasteable command that runs claude with
// args and CLAUDE_CONFIG_DIR set to profileDir, in goos's default shell
// (PowerShell on Windows, POSIX sh elsewhere). args must be shell-safe words.
func profileClaudeCommand(goos, profileDir string, args []string) string {
	if goos == "windows" {
		return fmt.Sprintf("$env:CLAUDE_CONFIG_DIR = %s; claude %s", powerShellQuote(profileDir), strings.Join(args, " "))
	}
	return fmt.Sprintf("CLAUDE_CONFIG_DIR=%s claude %s", posixQuote(profileDir), strings.Join(args, " "))
}

// launcherCommand renders the manual equivalent of the claude-pcs launcher.
func launcherCommand(goos, profileDir, sharedMCPPath string) string {
	if goos == "windows" {
		return fmt.Sprintf("$env:CLAUDE_CONFIG_DIR=%s; claude --mcp-config %s", powerShellQuote(profileDir), powerShellQuote(sharedMCPPath))
	}
	return fmt.Sprintf("CLAUDE_CONFIG_DIR=%s claude --mcp-config %s", posixQuote(profileDir), posixQuote(sharedMCPPath))
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
		var cmds []string
		for _, args := range requiredPluginSteps() {
			cmds = append(cmds, profileClaudeCommand(goos, profileDir, args))
		}
		steps = append(steps, "To install plugins into this profile, re-run 'pcs sync' with --with-plugins, or run: "+strings.Join(cmds, " ; then "))
	}
	if !launcherOnPath {
		steps = append(steps, pathHint(goos, launcherDir))
	}
	return steps
}
