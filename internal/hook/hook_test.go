package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yorth21/pcs/internal/flow"
)

func newProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func startFlow(t *testing.T, root, kind string) {
	t.Helper()
	if _, err := (flow.Store{Root: root}).Start("demo", kind); err != nil {
		t.Fatal(err)
	}
}

func runHook(t *testing.T, event string, in map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := Run(event, bytes.NewReader(raw), &out); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	return out.String()
}

func decision(t *testing.T, out string) (string, string) {
	t.Helper()
	if strings.TrimSpace(out) == "" {
		return "allow", ""
	}
	var d struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("bad output %q: %v", out, err)
	}
	if d.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Fatalf("hookEventName = %q", d.HookSpecificOutput.HookEventName)
	}
	return d.HookSpecificOutput.PermissionDecision, d.HookSpecificOutput.PermissionDecisionReason
}

func TestPreToolUseDecisions(t *testing.T) {
	tests := []struct {
		name       string
		kind       string // "" = no flow
		done       []string
		tool       string
		input      map[string]any
		agentID    string
		want       string
		reasonPart string
	}{
		{name: "subagent read allowed", tool: "Grep", input: map[string]any{"pattern": "x", "path": "src"}, agentID: "a1", want: "allow"},
		{name: "main grep without flow denied", tool: "Grep", input: map[string]any{"pattern": "x", "path": "src"}, want: "deny", reasonPart: "pcs flow start --feature"},
		{name: "main read of AGENTS.md allowed", tool: "Read", input: map[string]any{"file_path": "src/AGENTS.md"}, want: "allow"},
		{name: "main read under pcs/ allowed", tool: "Read", input: map[string]any{"file_path": "pcs/specs/demo.md"}, want: "allow"},
		{name: "main read outside root allowed", tool: "Read", input: map[string]any{"file_path": filepath.Join(os.TempDir(), "elsewhere.txt")}, want: "allow"},
		{name: "glob without path counts as cwd", tool: "Glob", input: map[string]any{"pattern": "**/*.tsx"}, want: "deny"},
		{name: "audit flow main read denied with delegation", kind: flow.KindAudit, tool: "Read", input: map[string]any{"file_path": "src/a.tsx"}, want: "deny", reasonPart: "Delegate to subagent pcs-explorer"},
		{name: "agent without flow denied", tool: "Agent", input: map[string]any{"subagent_type": "pcs-explorer"}, want: "deny", reasonPart: "pcs flow start"},
		{name: "generic Explore agent without flow denied", tool: "Agent", input: map[string]any{"subagent_type": "Explore"}, want: "deny", reasonPart: NonPCSAgentReason},
		{name: "generic agent without flow points to flow start", tool: "Task", input: map[string]any{"subagent_type": "general-purpose"}, want: "deny", reasonPart: "pcs flow start"},
		{name: "generic agent in audit flow points to next phase", kind: flow.KindAudit, tool: "Agent", input: map[string]any{"subagent_type": "general-purpose"}, want: "deny", reasonPart: "Delegate to subagent pcs-explorer"},
		{name: "omitted subagent type denied", kind: flow.KindAudit, tool: "Agent", input: map[string]any{"prompt": "map the feature"}, want: "deny", reasonPart: "(default)"},
		{name: "fork agent denied", kind: flow.KindT2, tool: "Agent", input: map[string]any{"subagent_type": "fork"}, want: "deny", reasonPart: NonPCSAgentReason},
		{name: "claude-code-guide allowed", tool: "Agent", input: map[string]any{"subagent_type": "claude-code-guide"}, want: "allow"},
		{name: "statusline-setup allowed", kind: flow.KindAudit, tool: "Task", input: map[string]any{"subagent_type": "statusline-setup"}, want: "allow"},
		{name: "generic agent from a subagent allowed", tool: "Agent", input: map[string]any{"subagent_type": "Explore"}, agentID: "a1", want: "allow"},
		{name: "first phase agent allowed", kind: flow.KindAudit, tool: "Agent", input: map[string]any{"subagent_type": "pcs-explorer"}, want: "allow"},
		{name: "out of order agent denied", kind: flow.KindAudit, tool: "Task", input: map[string]any{"subagent_type": "pcs-verifier"}, want: "deny", reasonPart: "before phase explore"},
		{name: "auditor after explore allowed", kind: flow.KindAudit, done: []string{"explore"}, tool: "Agent", input: map[string]any{"subagent_type": "pcs-auditor"}, want: "allow"},
		{name: "builder before compose denied", kind: flow.KindT2, done: []string{"explore", "strategy"}, tool: "Agent", input: map[string]any{"subagent_type": "pcs-builder"}, want: "deny", reasonPart: "phase compose"},
		{name: "agent outside flow phases denied", kind: flow.KindAudit, tool: "Agent", input: map[string]any{"subagent_type": "pcs-builder"}, want: "deny", reasonPart: "has no phase"},
		{name: "edit without flow denied", tool: "Edit", input: map[string]any{"file_path": "src/a.tsx"}, want: "deny"},
		{name: "write spec under pcs/ allowed", tool: "Write", input: map[string]any{"file_path": "pcs/specs/demo.md"}, want: "allow"},
		{name: "write flow state denied", kind: flow.KindAudit, tool: "Write", input: map[string]any{"file_path": ".pcs/flows/demo.json"}, want: "deny", reasonPart: "CLI-owned"},
		{name: "t2 main edit denied", kind: flow.KindT2, tool: "Edit", input: map[string]any{"file_path": "src/a.tsx"}, want: "deny", reasonPart: "delegate"},
		{name: "bash cat without flow denied", tool: "Bash", input: map[string]any{"command": "cat src/a.tsx"}, want: "deny"},
		{name: "bash pcs allowed", tool: "Bash", input: map[string]any{"command": "pcs flow start --feature demo --kind audit"}, want: "allow"},
		{name: "bash cat in audit denied", kind: flow.KindAudit, tool: "Bash", input: map[string]any{"command": "cd src && cat a.tsx"}, want: "deny", reasonPart: "pcs-explorer"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := newProject(t)
			if tc.kind != "" {
				startFlow(t, root, tc.kind)
				for _, p := range tc.done {
					if _, err := (flow.Store{Root: root}).Record(p, flow.StatusDone, "", "", flow.SourceManual); err != nil {
						t.Fatal(err)
					}
				}
			}
			out := runHook(t, "pre-tool-use", map[string]any{
				"cwd": root, "hook_event_name": "PreToolUse", "tool_name": tc.tool,
				"tool_input": tc.input, "agent_id": tc.agentID,
			})
			got, reason := decision(t, out)
			if got != tc.want {
				t.Fatalf("decision = %s (%s), want %s", got, reason, tc.want)
			}
			if !strings.Contains(reason, tc.reasonPart) {
				t.Fatalf("reason %q lacks %q", reason, tc.reasonPart)
			}
		})
	}
}

func TestInlineBudget(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindInline)
	read := func(tool string, input map[string]any) (string, string) {
		return decision(t, runHook(t, "pre-tool-use", map[string]any{"cwd": root, "tool_name": tool, "tool_input": input}))
	}
	for i := 0; i < flow.InlineReadBudget; i++ {
		if d, r := read("Read", map[string]any{"file_path": "src/a.tsx"}); d != "allow" {
			t.Fatalf("read %d denied: %s", i, r)
		}
	}
	if d, r := read("Bash", map[string]any{"command": "grep -r x src"}); d != "deny" || !strings.Contains(r, "inline budget exhausted") {
		t.Fatalf("fourth read = %s %s", d, r)
	}
	// Exempt reads never count.
	if d, _ := read("Read", map[string]any{"file_path": "README.md"}); d != "allow" {
		t.Fatal("README read denied")
	}
	if d, r := read("Edit", map[string]any{"file_path": "src/a.tsx"}); d != "allow" {
		t.Fatalf("first edit denied: %s", r)
	}
	if d, _ := read("MultiEdit", map[string]any{"file_path": "src/a.tsx"}); d != "allow" {
		t.Fatal("same-file edit denied")
	}
	if d, r := read("Write", map[string]any{"file_path": "src/b.tsx"}); d != "deny" || !strings.Contains(r, "inline budget exhausted") {
		t.Fatalf("second file = %s %s", d, r)
	}
	f, err := (flow.Store{Root: root}).Active()
	if err != nil || f.InlineReads != 3 || len(f.InlineFiles) != 1 || f.InlineFiles[0] != "src/a.tsx" {
		t.Fatalf("flow = %+v, %v", f, err)
	}
}

func TestStopHook(t *testing.T) {
	root := newProject(t)
	if out := runHook(t, "stop", map[string]any{"cwd": root}); out != "" {
		t.Fatalf("no flow: %q", out)
	}
	startFlow(t, root, flow.KindAudit)
	out := runHook(t, "stop", map[string]any{"cwd": root})
	var d stopDecision
	if err := json.Unmarshal([]byte(out), &d); err != nil || d.Decision != "block" ||
		!strings.Contains(d.Reason, "PCD flow demo (audit) is incomplete: pending explore, audit") ||
		!strings.Contains(d.Reason, "pcs flow close --abandon") {
		t.Fatalf("incomplete stop = %q", out)
	}
	if out := runHook(t, "stop", map[string]any{"cwd": root, "stop_hook_active": true}); out != "" {
		t.Fatalf("stop_hook_active: %q", out)
	}
	s := flow.Store{Root: root}
	s.Record("explore", flow.StatusDone, "", "", flow.SourceManual)
	s.Record("audit", flow.StatusDone, "", "", flow.SourceManual)
	out = runHook(t, "stop", map[string]any{"cwd": root})
	if !strings.Contains(out, `"block"`) || !strings.Contains(out, "pcs flow close") {
		t.Fatalf("complete stop = %q", out)
	}
	if out := runHook(t, "stop", map[string]any{"cwd": root, "stop_hook_active": true}); out != "" {
		t.Fatalf("complete with stop_hook_active: %q", out)
	}
}

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "agent.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func assistant(text string) string {
	b, _ := json.Marshal(map[string]any{
		"type":    "assistant",
		"message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": text}}},
	})
	return string(b)
}

func TestSubagentStopEnvelope(t *testing.T) {
	envelope := func(status string) string {
		return "**Status**: " + status + "\n**Executive Summary**: x\n**Next Recommended**: none\n\n## Key Learnings\n1. Something factual was learned here."
	}
	tests := []struct {
		name         string
		lines        []string
		wantStatus   string
		wantEnvelope string
	}{
		{name: "done", lines: []string{assistant("working"), assistant(envelope("done"))}, wantStatus: flow.StatusDone, wantEnvelope: "done"},
		{name: "success", lines: []string{assistant(envelope("success"))}, wantStatus: flow.StatusDone, wantEnvelope: "success"},
		{name: "partial", lines: []string{assistant(envelope("partial"))}, wantStatus: flow.StatusDone, wantEnvelope: "partial"},
		{name: "blocked", lines: []string{assistant(envelope("blocked"))}, wantStatus: flow.StatusBlocked, wantEnvelope: "blocked"},
		{name: "last message wins", lines: []string{assistant(envelope("done")), assistant("I stopped without an envelope.")}, wantStatus: flow.StatusUnverified},
		{name: "missing envelope", lines: []string{`{"type":"user","message":{"role":"user","content":"hi"}}`, assistant("no envelope")}, wantStatus: flow.StatusUnverified},
		{name: "string content", lines: []string{`{"type":"assistant","message":{"role":"assistant","content":"Status: blocked"}}`}, wantStatus: flow.StatusBlocked, wantEnvelope: "blocked"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := newProject(t)
			startFlow(t, root, flow.KindAudit)
			out := runHook(t, "subagent-stop", map[string]any{
				"cwd": root, "agent_id": "ag-1", "agent_type": "pcs-explorer",
				"agent_transcript_path": writeTranscript(t, tc.lines...),
			})
			if out != "" {
				t.Fatalf("subagent-stop printed %q", out)
			}
			f, _ := (flow.Store{Root: root}).Active()
			p := f.Phases[0]
			if p.Status != tc.wantStatus || p.EnvelopeStatus != tc.wantEnvelope || p.AgentID != "ag-1" || p.Source != flow.SourceHook || p.RecordedAt == "" {
				t.Fatalf("phase = %+v", p)
			}
		})
	}
}

func TestSubagentStopAuditPhaseMapping(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindAudit)
	runHook(t, "subagent-stop", map[string]any{"cwd": root, "agent_type": "pcs-verifier",
		"agent_transcript_path": writeTranscript(t, assistant("**Status**: done"))})
	f, _ := (flow.Store{Root: root}).Active()
	if f.Phases[1].Name != "audit" || f.Phases[1].Status != flow.StatusDone || f.Phases[0].Status != flow.StatusPending {
		t.Fatalf("phases = %+v", f.Phases)
	}
	// Non-pcs agents are ignored.
	runHook(t, "subagent-stop", map[string]any{"cwd": root, "agent_type": "Explore"})
	if f2, _ := (flow.Store{Root: root}).Active(); f2.Phases[0].Status != flow.StatusPending {
		t.Fatal("non-pcs agent recorded")
	}
}

func TestUserPromptSubmit(t *testing.T) {
	root := newProject(t)
	out := runHook(t, "user-prompt-submit", map[string]any{"cwd": root, "prompt": "review the UI"})
	var d struct {
		HookSpecificOutput promptContext `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil || d.HookSpecificOutput.HookEventName != "UserPromptSubmit" || d.HookSpecificOutput.AdditionalContext != NoFlowRule {
		t.Fatalf("no flow = %q", out)
	}
	if len(NoFlowRule) >= 600 {
		t.Fatalf("rule is %d chars", len(NoFlowRule))
	}
	startFlow(t, root, flow.KindT2)
	out = runHook(t, "user-prompt-submit", map[string]any{"cwd": root})
	if !strings.Contains(out, "PCD flow demo (t2) is active") || !strings.Contains(out, "pcs-explorer") {
		t.Fatalf("active = %q", out)
	}
}

func TestCwdGitignore(t *testing.T) {
	for _, event := range []string{"user-prompt-submit", "subagent-stop"} {
		t.Run(event, func(t *testing.T) {
			root := newProject(t)
			front := filepath.Join(root, "src")
			ignore := filepath.Join(front, ".pcs", ".gitignore")

			// Without <cwd>/.pcs nothing is created.
			runHook(t, event, map[string]any{"cwd": front})
			if _, err := os.Stat(filepath.Join(front, ".pcs")); !os.IsNotExist(err) {
				t.Fatalf(".pcs created without one: %v", err)
			}

			// A bare .pcs/playwright under the cwd gets a "*" .gitignore.
			if err := os.MkdirAll(filepath.Join(front, ".pcs", "playwright"), 0o755); err != nil {
				t.Fatal(err)
			}
			runHook(t, event, map[string]any{"cwd": front})
			if b, err := os.ReadFile(ignore); err != nil || string(b) != "*\n" {
				t.Fatalf(".gitignore = %q, %v", b, err)
			}

			// An existing .gitignore is kept.
			if err := os.WriteFile(ignore, []byte("custom\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			runHook(t, event, map[string]any{"cwd": front})
			if b, _ := os.ReadFile(ignore); string(b) != "custom\n" {
				t.Fatalf(".gitignore overwritten: %q", b)
			}
		})
	}
	// Other events never touch it.
	root := newProject(t)
	front := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(front, ".pcs", "playwright"), 0o755); err != nil {
		t.Fatal(err)
	}
	runHook(t, "pre-tool-use", map[string]any{"cwd": front, "tool_name": "Read", "tool_input": map[string]any{"file_path": "x"}})
	if _, err := os.Stat(filepath.Join(front, ".pcs", ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("pre-tool-use wrote .gitignore: %v", err)
	}
}

func TestFailOpen(t *testing.T) {
	root := newProject(t)
	if err := os.Mkdir(filepath.Join(root, ".pcs"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	var out bytes.Buffer
	if code := Run("pre-tool-use", strings.NewReader("{not json"), &out); code != 0 || out.Len() != 0 {
		t.Fatalf("malformed stdin: code=%d out=%q", code, out.String())
	}
	b, err := os.ReadFile(filepath.Join(root, ".pcs", "hook-errors.log"))
	if err != nil || !strings.Contains(string(b), "pre-tool-use: parse stdin") {
		t.Fatalf("error log = %q, %v", b, err)
	}
	// A corrupt flow state fails open too.
	os.MkdirAll(filepath.Join(root, ".pcs", "flows"), 0o755)
	os.WriteFile(filepath.Join(root, ".pcs", "active"), []byte("demo\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".pcs", "flows", "demo.json"), []byte("{"), 0o644)
	if o := runHook(t, "stop", map[string]any{"cwd": root}); o != "" {
		t.Fatalf("corrupt state printed %q", o)
	}
}

func TestBashAllowed(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"cat src/a.tsx", false},
		{"sed -n 1,20p src/a.tsx", false},
		{"grep -rn foo src", false},
		{"find src -name '*.tsx'", false},
		{"wc -l src/a.tsx", false},
		{"ls src", false},
		{"git status && pcs flow next", true},
		{"cd src && cat x", false},
		{"cd src && git diff", true},
		{"git status | cat", false},
		{"git log --oneline | head -5", false},
		{"C:/Users/x/AppData/Local/pcs/bin/pcs.exe flow status --json", true},
		{`"/usr/local/bin/pcs" hook stop`, true},
		{"git -C sub status", true},
		{"git grep foo", false},
		{"git commit -m \"fix: a; cat b\"", true},
		{"npm run dev", true},
		{"npm view react", false},
		{"pnpm test", true},
		{"npx playwright test", true},
		{"node --version", true},
		{"node script.js", false},
		{"curl -s http://localhost:5173", true},
		{"go test ./...", true},
		{"go doc fmt", false},
		{"dotnet build", true},
		{"echo hello", true},
		{"echo $(cat src/a.tsx)", false},
		{"echo `cat x`", false},
		{"echo x > src/a.tsx", false},
		{"npm run build 2>&1", true},
		{"go build ./... > /dev/null 2>&1", true},
		{"pwd; which pcs; command -v git", true},
		{"command ls", false},
		{"sleep 2 && kill 123", true},
		{"mkdir -p pcs/specs", true},
		{"test -f a && true", true},
		{"[ -d src ]", true},
		{"FOO=1 go test ./...", true},
		{"xargs cat < files.txt", false},
		{"head -5 a", false},
		{"", false},
		{"echo 'unterminated", false},
		{"gh pr view 1", true},
		{"codegraph status", true},
		{"engram search x", true},
	}
	for _, tc := range tests {
		if got := BashAllowed(tc.cmd); got != tc.want {
			t.Errorf("BashAllowed(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}
