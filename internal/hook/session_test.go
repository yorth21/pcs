package hook

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yorth21/pcs/internal/flow"
)

func owner(t *testing.T, root string) string {
	t.Helper()
	f, err := (flow.Store{Root: root}).Active()
	if err != nil || f == nil {
		t.Fatalf("active = %v, %v", f, err)
	}
	return f.OwnerSessionID
}

func TestFirstEventClaimsFlow(t *testing.T) {
	events := []struct {
		event string
		in    map[string]any
	}{
		{"user-prompt-submit", map[string]any{"prompt": "hi"}},
		{"pre-tool-use", map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "pcs flow next"}}},
		{"stop", map[string]any{}},
		{"stop", map[string]any{"stop_hook_active": true}},
		{"subagent-stop", map[string]any{"agent_type": "Explore"}},
	}
	for _, ev := range events {
		t.Run(ev.event, func(t *testing.T) {
			root := newProject(t)
			startFlow(t, root, flow.KindAudit)
			ev.in["cwd"] = root
			ev.in["session_id"] = "A"
			runHook(t, ev.event, ev.in)
			if got := owner(t, root); got != "A" {
				t.Fatalf("owner = %q, want A", got)
			}
			// A later event from another session never steals the flow.
			ev.in["session_id"] = "B"
			runHook(t, ev.event, ev.in)
			if got := owner(t, root); got != "A" {
				t.Fatalf("owner after B = %q, want A", got)
			}
		})
	}
}

func TestStopBySession(t *testing.T) {
	tests := []struct {
		name      string
		session   string
		wantBlock bool
	}{
		{name: "owning session blocks", session: "A", wantBlock: true},
		{name: "foreign session does not block", session: "B"},
		{name: "unknown session blocks (legacy payload)", session: "", wantBlock: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := newProject(t)
			startFlow(t, root, flow.KindAudit)
			runHook(t, "stop", map[string]any{"cwd": root, "session_id": "A"}) // claim
			out := runHook(t, "stop", map[string]any{"cwd": root, "session_id": tc.session})
			if !tc.wantBlock {
				if out != "" {
					t.Fatalf("foreign stop printed %q", out)
				}
				return
			}
			var d stopDecision
			if err := json.Unmarshal([]byte(out), &d); err != nil || d.Decision != "block" || !strings.Contains(d.Reason, "is incomplete") {
				t.Fatalf("stop = %q", out)
			}
		})
	}
}

func TestForeignSessionPreToolUse(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		input   map[string]any
		agentID string
		want    string
	}{
		{name: "grep denied", tool: "Grep", input: map[string]any{"pattern": "x", "path": "src"}, want: "deny"},
		{name: "edit denied", tool: "Edit", input: map[string]any{"file_path": "src/a.tsx"}, want: "deny"},
		{name: "pcs agent denied", tool: "Agent", input: map[string]any{"subagent_type": "pcs-explorer"}, want: "deny"},
		{name: "generic agent denied", tool: "Agent", input: map[string]any{"subagent_type": "Explore"}, want: "deny"},
		{name: "bash flow adopt allowed", tool: "Bash", input: map[string]any{"command": "pcs flow adopt"}, want: "allow"},
		{name: "bash flow start allowed", tool: "Bash", input: map[string]any{"command": "pcs flow start --feature mine --kind audit"}, want: "allow"},
		{name: "subagent read allowed", tool: "Read", input: map[string]any{"file_path": "src/a.tsx"}, agentID: "a1", want: "allow"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := newProject(t)
			startFlow(t, root, flow.KindInline)
			runHook(t, "stop", map[string]any{"cwd": root, "session_id": "A"}) // claim
			out := runHook(t, "pre-tool-use", map[string]any{
				"cwd": root, "session_id": "B", "tool_name": tc.tool, "tool_input": tc.input, "agent_id": tc.agentID,
			})
			got, reason := decision(t, out)
			if got != tc.want {
				t.Fatalf("decision = %s (%s), want %s", got, reason, tc.want)
			}
			if tc.want == "deny" {
				for _, part := range []string{"An unfinished PCD flow `demo` from another session is active", "pcs flow adopt", `pcs flow close --abandon --reason "..."`, "then start yours"} {
					if !strings.Contains(reason, part) {
						t.Fatalf("reason %q lacks %q", reason, part)
					}
				}
			}
			// The inline flow budget of session A is never spent by B.
			if f, _ := (flow.Store{Root: root}).Active(); f.InlineReads != 0 || len(f.InlineFiles) != 0 {
				t.Fatalf("foreign session spent budget: %+v", f)
			}
		})
	}
	// The owning session keeps its inline budget.
	root := newProject(t)
	startFlow(t, root, flow.KindInline)
	out := runHook(t, "pre-tool-use", map[string]any{"cwd": root, "session_id": "A", "tool_name": "Read", "tool_input": map[string]any{"file_path": "src/a.tsx"}})
	if got, reason := decision(t, out); got != "allow" {
		t.Fatalf("owner read = %s %s", got, reason)
	}
}

func TestForeignSessionPromptNotice(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindT2)
	runHook(t, "user-prompt-submit", map[string]any{"cwd": root, "session_id": "A"})
	out := runHook(t, "user-prompt-submit", map[string]any{"cwd": root, "session_id": "B"})
	var d struct {
		HookSpecificOutput promptContext `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatal(err)
	}
	ctx := d.HookSpecificOutput.AdditionalContext
	if !strings.HasPrefix(ctx, NoFlowRule) || !strings.Contains(ctx, "An unfinished PCD flow `demo` from another session is active") || strings.Contains(ctx, "is active: pending") {
		t.Fatalf("foreign context = %q", ctx)
	}
}

func TestForeignSubagentStopNotRecorded(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindAudit)
	runHook(t, "stop", map[string]any{"cwd": root, "session_id": "A"})
	runHook(t, "subagent-stop", map[string]any{"cwd": root, "session_id": "B", "agent_type": "pcs-explorer", "last_assistant_message": "**Status**: success"})
	if f, _ := (flow.Store{Root: root}).Active(); f.Phases[0].Status != flow.StatusPending {
		t.Fatalf("foreign run recorded: %+v", f.Phases[0])
	}
	runHook(t, "subagent-stop", map[string]any{"cwd": root, "session_id": "A", "agent_type": "pcs-explorer", "last_assistant_message": "**Status**: success"})
	if f, _ := (flow.Store{Root: root}).Active(); f.Phases[0].Status != flow.StatusDone {
		t.Fatalf("owner run not recorded: %+v", f.Phases[0])
	}
}

func TestAdoptThenClaim(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindAudit)
	runHook(t, "stop", map[string]any{"cwd": root, "session_id": "A"})
	if out := runHook(t, "stop", map[string]any{"cwd": root, "session_id": "B"}); out != "" {
		t.Fatalf("foreign stop before adopt = %q", out)
	}
	if _, err := (flow.Store{Root: root}).Adopt(); err != nil {
		t.Fatal(err)
	}
	// The next event of B claims the flow; from then on B owns it and A is foreign.
	out := runHook(t, "pre-tool-use", map[string]any{"cwd": root, "session_id": "B", "tool_name": "Agent", "tool_input": map[string]any{"subagent_type": "pcs-explorer"}})
	if got, reason := decision(t, out); got != "allow" {
		t.Fatalf("adopted delegation = %s %s", got, reason)
	}
	if got := owner(t, root); got != "B" {
		t.Fatalf("owner = %q, want B", got)
	}
	if out := runHook(t, "stop", map[string]any{"cwd": root, "session_id": "B"}); !strings.Contains(out, `"block"`) {
		t.Fatalf("adopting session stop = %q", out)
	}
	if out := runHook(t, "stop", map[string]any{"cwd": root, "session_id": "A"}); out != "" {
		t.Fatalf("previous owner stop = %q", out)
	}
}
