package hook

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/yorth21/pcs/internal/flow"
)

// observedExplorerEnding is the exact ending of a real pcs-explorer final
// message that an earlier parser recorded as unverified.
const observedExplorerEnding = "## Key Learnings\n1. ...\n**Status**: success\n**Skill Resolution**: paths-injected\n"

func TestParseEnvelopeStatus(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"observed explorer ending", "Summary of the map.\n\n" + observedExplorerEnding, "success"},
		{"bold key", "**Status**: success", "success"},
		{"bullet with colon inside bold", "- **Status:** partial", "partial"},
		{"plain", "Status: blocked", "blocked"},
		{"bold key and value", "**Status**: **success**", "success"},
		{"uppercase value", "**Status**: SUCCESS", "success"},
		{"underscore emphasis", "__Status__: _partial_", "partial"},
		{"blockquote", "> **Status**: done", "done"},
		{"indented star bullet", "  * Status: blocked", "blocked"},
		{"last match wins", "Status: blocked\n...\n**Status**: success", "success"},
		{"unknown value ignored", "**Status**: success\n| Status: pending |", "success"},
		{"no status", "I stopped without an envelope.", ""},
		{"status word in prose", "The status of the form is unclear: success is not shown.", ""},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseEnvelopeStatus(tc.text); got != tc.want {
				t.Fatalf("ParseEnvelopeStatus(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func subagentStopPayload(root, agentType, agentID, message, transcript string) map[string]any {
	return map[string]any{
		"cwd": root, "hook_event_name": "SubagentStop", "agent_id": agentID, "agent_type": agentType,
		"last_assistant_message": message, "agent_transcript_path": transcript,
	}
}

func TestSubagentStopPrefersLastAssistantMessage(t *testing.T) {
	tests := []struct {
		name, message string
		transcript    []string
		want          string
		wantEnvelope  string
	}{
		{name: "observed message without transcript", message: "Mapped the feature.\n" + observedExplorerEnding, want: flow.StatusDone, wantEnvelope: "success"},
		{name: "message beats transcript", message: "**Status**: success", transcript: []string{assistant("**Status**: blocked")}, want: flow.StatusDone, wantEnvelope: "success"},
		{name: "transcript fallback when message has no status", message: "no envelope", transcript: []string{assistant("- **Status:** partial")}, want: flow.StatusDone, wantEnvelope: "partial"},
		{name: "neither has status", message: "no envelope", want: flow.StatusUnverified},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := newProject(t)
			startFlow(t, root, flow.KindAudit)
			transcript := ""
			if tc.transcript != nil {
				transcript = writeTranscript(t, tc.transcript...)
			}
			runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-explorer", "ag-1", tc.message, transcript))
			f, _ := (flow.Store{Root: root}).Active()
			if p := f.Phases[0]; p.Status != tc.want || p.EnvelopeStatus != tc.wantEnvelope {
				t.Fatalf("phase = %+v", p)
			}
		})
	}
}

func TestSubagentStopDuplicateRunDoesNotDowngrade(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindAudit)
	runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-explorer", "ag-1", observedExplorerEnding, ""))
	runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-explorer", "ag-2", "Duplicate run with no envelope.", ""))
	f, _ := (flow.Store{Root: root}).Active()
	p := f.Phases[0]
	if p.Status != flow.StatusDone || p.AgentID != "ag-1" || len(p.Runs) != 2 || p.Runs[1].AgentID != "ag-2" || p.Runs[1].Status != flow.StatusUnverified {
		t.Fatalf("phase = %+v", p)
	}
	if next := f.Next(); next.Phase != "audit" {
		t.Fatalf("next = %+v", next)
	}
}

func TestSubagentStopUnverifiedThenRelaunch(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindAudit)
	runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-explorer", "ag-1", "no envelope", ""))
	f, _ := (flow.Store{Root: root}).Active()
	if next := f.Next(); !strings.Contains(next.Instruction, flow.UnverifiedRunInstruction("pcs-explorer")) {
		t.Fatalf("next after unverified = %+v", next)
	}
	runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-explorer", "ag-2", "**Status**: blocked", ""))
	runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-explorer", "ag-3", "**Status**: success", ""))
	f, _ = (flow.Store{Root: root}).Active()
	if p := f.Phases[0]; p.Status != flow.StatusDone || p.AgentID != "ag-3" || len(p.Runs) != 3 {
		t.Fatalf("phase = %+v", p)
	}
}

func TestSubagentStopVerifyAcceptance(t *testing.T) {
	tests := []struct {
		envelope, want string
		wantNote       bool
	}{
		{"success", flow.StatusDone, false},
		{"partial", flow.StatusBlocked, true},
		{"blocked", flow.StatusBlocked, false},
	}
	for _, tc := range tests {
		t.Run(tc.envelope, func(t *testing.T) {
			root := newProject(t)
			startFlow(t, root, flow.KindT2)
			s := flow.Store{Root: root}
			for _, p := range []string{"explore", "strategy", "compose", "apply"} {
				if _, err := s.Record(p, flow.StatusDone, "", "", flow.SourceManual); err != nil {
					t.Fatal(err)
				}
			}
			runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-verifier", "v1", "**Status**: "+tc.envelope, ""))
			f, _ := s.Active()
			p := f.Phases[f.PhaseIndex(flow.PhaseVerify)]
			if p.Status != tc.want || (p.Note != "") != tc.wantNote {
				t.Fatalf("verify phase = %+v", p)
			}
		})
	}
}

func TestManualFlowRecordDenied(t *testing.T) {
	tests := []struct {
		cmd      string
		wantDeny bool
	}{
		{"pcs flow record --phase explore --status done", true},
		{"C:/Users/x/AppData/Local/pcs/bin/pcs.exe flow record --phase explore --status done", true},
		{`"C:\Program Files\pcs\pcs.exe" flow record --phase audit --status done`, true},
		{"pcs flow next && pcs flow record --phase explore --status done", true},
		{`sh -c "pcs flow record --phase explore --status done"`, true},
		{"PCS FLOW RECORD --phase explore --status done", true},
		{"pcs flow next", false},
		{"pcs flow status --json", false},
		{"pcs flow close", false},
		{"echo mypcs flow record", false},
	}
	for _, tc := range tests {
		t.Run(tc.cmd, func(t *testing.T) {
			if got := FlowRecordCommand(tc.cmd); got != tc.wantDeny {
				t.Fatalf("FlowRecordCommand(%q) = %v, want %v", tc.cmd, got, tc.wantDeny)
			}
			root := newProject(t)
			startFlow(t, root, flow.KindAudit)
			got, reason := decision(t, runHook(t, "pre-tool-use", map[string]any{
				"cwd": root, "tool_name": "Bash", "tool_input": map[string]any{"command": tc.cmd},
			}))
			if tc.wantDeny && (got != "deny" || reason != ManualRecordReason) {
				t.Fatalf("decision = %s (%s), want deny with the manual-record reason", got, reason)
			}
			if !tc.wantDeny && reason == ManualRecordReason {
				t.Fatalf("denied as manual record: %s", tc.cmd)
			}
		})
	}
	// Subagents may not self-report phases either, but their other Bash
	// commands stay ungated.
	root := newProject(t)
	startFlow(t, root, flow.KindAudit)
	for _, cmd := range []string{"pcs flow record --phase explore --status done", `bash -c "C:/tools/pcs.exe flow record --phase audit --status done"`} {
		if got, reason := decision(t, runHook(t, "pre-tool-use", map[string]any{
			"cwd": root, "tool_name": "Bash", "agent_id": "a1", "agent_type": "pcs-explorer",
			"tool_input": map[string]any{"command": cmd},
		})); got != "deny" || reason != ManualRecordReason {
			t.Fatalf("subagent %q decision = %s (%s), want deny", cmd, got, reason)
		}
	}
	if got, _ := decision(t, runHook(t, "pre-tool-use", map[string]any{
		"cwd": root, "tool_name": "Bash", "agent_id": "a1",
		"tool_input": map[string]any{"command": "cat src/app.tsx"},
	})); got != "allow" {
		t.Fatalf("subagent cat decision = %s, want allow", got)
	}
	// Subagents cannot rewrite CLI-owned flow state either.
	if got, _ := decision(t, runHook(t, "pre-tool-use", map[string]any{
		"cwd": root, "tool_name": "Write", "agent_id": "a1",
		"tool_input": map[string]any{"file_path": filepath.Join(root, ".pcs", "flows", "demo.json")},
	})); got != "deny" {
		t.Fatalf("subagent .pcs write decision = %s, want deny", got)
	}
}
