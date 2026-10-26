package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// TestCmdLauncherPassesArgsLiterallyWithMCPConfigLast runs the .cmd launcher the
// way PowerShell does: cmd.exe /s /c with the whole command line wrapped in an
// extra pair of quotes. Starting a .cmd directly through CreateProcess (as
// exec.Command does) lets cmd.exe strip the first and last quote of the line,
// which splits a launcher path containing spaces.
func TestCmdLauncherPassesArgsLiterallyWithMCPConfigLast(t *testing.T) {
	launcherDir, profileDir, sharedMCP := installLauncherForExec(t, launcherArgsWithShellMetacharacters, "cmd")

	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	launcher := filepath.Join(launcherDir, "claude-pcs.cmd")
	cmd := exec.Command(comspec)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: `cmd.exe /d /s /c ""` + launcher + `" "hello world" --resume x"`,
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run cmd launcher: %v\n%s", err, out)
	}
	assertLauncherArgv(t, out, profileDir, sharedMCP)
}
