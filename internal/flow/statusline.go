package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// StatusLineSeparator joins the base status line and the pcs flow segment.
const StatusLineSeparator = " │ "

// StatusLineBaseTimeout bounds the wrapped base status line command.
const StatusLineBaseTimeout = 2 * time.Second

// StatusLineInput is the subset of the Claude Code statusLine stdin JSON that
// pcs reads.
type StatusLineInput struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
}

// Segment returns the one-line status of the active flow, e.g.
// "pcs:checkout t2 ▸ apply 4/5" or "pcs:x audit ▸ awaiting user", or "" when
// no flow is active. A flow owned by another session is marked as such.
func Segment(f *Flow, session string) string {
	if f == nil || f.Closed() {
		return ""
	}
	head := "pcs:" + f.Feature + " " + f.Kind + " ▸ "
	switch {
	case f.ForeignTo(session):
		return head + "other session"
	case f.AwaitingUser != nil:
		return head + "awaiting user"
	case f.Kind == KindInline:
		return head + fmt.Sprintf("inline reads %d/%d", f.InlineReads, InlineReadBudget)
	}
	for i, p := range f.Phases {
		if p.Status == StatusDone {
			continue
		}
		s := fmt.Sprintf("%s%s %d/%d", head, p.Name, i+1, len(f.Phases))
		if p.Status != StatusPending {
			s += " " + p.Status
		}
		return s
	}
	return head + "close"
}

// StatusLineSegment resolves the pcs root for cwd and returns the segment of
// its active flow. Every error reads as no segment.
func StatusLineSegment(cwd, session string) string {
	f, err := Store{Root: ResolveRoot(cwd)}.Active()
	if err != nil {
		return ""
	}
	return Segment(f, session)
}

// readPiped reads r fully unless it is an interactive terminal, so a status
// line command run by hand never waits for input.
func readPiped(r io.Reader) []byte {
	if r == nil {
		return nil
	}
	if f, ok := r.(*os.File); ok {
		if info, err := f.Stat(); err != nil || info.Mode()&os.ModeCharDevice != 0 {
			return nil
		}
	}
	b, _ := io.ReadAll(r)
	return b
}

// statusLineTarget returns the directory and session a status line describes:
// --cwd when given, else workspace.current_dir or cwd from stdin, else
// workDir.
func statusLineTarget(flagCwd string, raw []byte, workDir string) (string, string) {
	var in StatusLineInput
	_ = json.Unmarshal(raw, &in)
	dir := strings.TrimSpace(flagCwd)
	for _, c := range []string{in.Workspace.CurrentDir, in.Cwd, workDir} {
		if dir == "" {
			dir = strings.TrimSpace(c)
		}
	}
	return dir, in.SessionID
}

// RunStatusLine executes `pcs statusline`: it prints the active flow segment
// for the cwd's root, or nothing. It never fails.
func RunStatusLine(args []string, stdin io.Reader, stdout io.Writer, workDir string) (code int) {
	defer func() {
		if recover() != nil {
			code = 0
		}
	}()
	fs := flag.NewFlagSet("statusline", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cwd := fs.String("cwd", "", "Project directory (default: the statusLine stdin JSON, then the working directory)")
	_ = fs.Parse(args)
	dir, session := statusLineTarget(*cwd, readPiped(stdin), workDir)
	if seg := StatusLineSegment(dir, session); seg != "" {
		fmt.Fprintln(stdout, seg)
	}
	return 0
}

// StatusLineBaseField is the .pcs-state.json field holding the base status
// line command that `pcs statusline-wrap` runs.
const StatusLineBaseField = "flow_statusline_base_command"

// ReadStatusLineBase returns the base status line command recorded in the
// pcs state file at path, or "".
func ReadStatusLineBase(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var st map[string]json.RawMessage
	if json.Unmarshal(b, &st) != nil {
		return ""
	}
	var cmd string
	_ = json.Unmarshal(st[StatusLineBaseField], &cmd)
	return strings.TrimSpace(cmd)
}

// ShellCommand builds the command that runs a status line command string the
// way Claude Code does: Git Bash (or sh) when available, cmd.exe otherwise on
// Windows. Tests replace it.
var ShellCommand = func(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS != "windows" {
		return exec.CommandContext(ctx, "sh", "-c", command)
	}
	if bash := windowsBash(); bash != "" {
		return exec.CommandContext(ctx, bash, "-c", command)
	}
	return exec.CommandContext(ctx, "cmd", "/C", command)
}

// windowsBash returns Git Bash: $CLAUDE_CODE_GIT_BASH_PATH, else bash on PATH
// unless it is the WSL launcher under the Windows system directory.
func windowsBash() string {
	if p := strings.TrimSpace(os.Getenv("CLAUDE_CODE_GIT_BASH_PATH")); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	p, err := exec.LookPath("bash")
	if err != nil {
		return ""
	}
	if sys := os.Getenv("SystemRoot"); sys != "" {
		if rel, err := filepath.Rel(sys, p); err == nil && !strings.HasPrefix(rel, "..") {
			return ""
		}
	}
	return p
}

// RunBaseStatusLine runs command with stdin and returns its stdout without
// trailing newlines, killing it after timeout. A failing or timed-out command
// still returns whatever it printed.
func RunBaseStatusLine(command string, stdin []byte, timeout time.Duration) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := ShellCommand(ctx, command)
	cmd.Stdin = bytes.NewReader(stdin)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 500 * time.Millisecond
	_ = cmd.Run()
	return strings.TrimRight(out.String(), "\r\n")
}

// RunStatusLineWrap executes `pcs statusline-wrap --state <path>`: it reads
// stdin once, runs the base command recorded in the state file with the same
// stdin (2s timeout), and prints the base output followed by
// StatusLineSeparator and the flow segment when one exists. It never fails.
func RunStatusLineWrap(args []string, stdin io.Reader, stdout io.Writer, workDir string) (code int) {
	defer func() {
		if recover() != nil {
			code = 0
		}
	}()
	fs := flag.NewFlagSet("statusline-wrap", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	statePath := fs.String("state", "", "pcs state file holding "+StatusLineBaseField)
	base := fs.String("base", "", "Base status line command (overrides --state)")
	cwd := fs.String("cwd", "", "Project directory (default: the statusLine stdin JSON, then the working directory)")
	_ = fs.Parse(args)
	raw := readPiped(stdin)
	command := strings.TrimSpace(*base)
	if command == "" && strings.TrimSpace(*statePath) != "" {
		command = ReadStatusLineBase(*statePath)
	}
	out := RunBaseStatusLine(command, raw, StatusLineBaseTimeout)
	dir, session := statusLineTarget(*cwd, raw, workDir)
	seg := StatusLineSegment(dir, session)
	switch {
	case out != "" && seg != "":
		out += StatusLineSeparator + seg
	case out == "":
		out = seg
	}
	if out != "" {
		fmt.Fprintln(stdout, out)
	}
	return 0
}
