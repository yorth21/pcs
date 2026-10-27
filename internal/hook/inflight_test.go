package hook

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yorth21/pcs/internal/flow"
)

// TestInFlightDelegation walks one background delegation: the allowed Agent
// launch is recorded, a duplicate launch is denied, Stop lets the turn end
// while the subagent runs, SubagentStop clears it, and Stop blocks again.
func TestInFlightDelegation(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindAudit)
	store := flow.Store{Root: root}
	launch := func(tool string) (string, string) {
		return decision(t, runHook(t, "pre-tool-use", map[string]any{"cwd": root, "session_id": "A", "tool_name": tool,
			"tool_input": map[string]any{"subagent_type": "pcs-explorer", "run_in_background": true}}))
	}
	stopBlocks := func() bool {
		out := runHook(t, "stop", map[string]any{"cwd": root, "session_id": "A"})
		if out == "" {
			return false
		}
		var d stopDecision
		if err := json.Unmarshal([]byte(out), &d); err != nil || d.Decision != "block" {
			t.Fatalf("stop = %q", out)
		}
		return true
	}

	if got, reason := launch("Agent"); got != "allow" {
		t.Fatalf("first launch = %s %s", got, reason)
	}
	f, _ := store.Active()
	if len(f.InFlight) != 1 || f.InFlight[0].AgentType != "pcs-explorer" || f.InFlight[0].Phase != "explore" || f.InFlight[0].StartedAt == "" {
		t.Fatalf("in flight = %+v", f.InFlight)
	}
	for _, tool := range []string{"Agent", "Task"} {
		if got, reason := launch(tool); got != "deny" || !strings.Contains(reason, "already running") || !strings.Contains(reason, "Wait for its completion") {
			t.Fatalf("duplicate %s launch = %s %s", tool, got, reason)
		}
	}
	if f, _ := store.Active(); len(f.InFlight) != 1 {
		t.Fatalf("denied launch recorded: %+v", f.InFlight)
	}
	if stopBlocks() {
		t.Fatal("stop blocked while the delegation is in flight")
	}
	if tr := mustActive(t, store).Next(); tr.Kind != flow.TransitionWait {
		t.Fatalf("next while running = %+v", tr)
	}
	// SubagentStop clears the entry whatever the envelope.
	runHook(t, "subagent-stop", map[string]any{"cwd": root, "session_id": "A", "agent_id": "x1", "agent_type": "pcs-explorer",
		"last_assistant_message": "I stopped without an envelope."})
	if f := mustActive(t, store); len(f.InFlight) != 0 {
		t.Fatalf("in flight after stop = %+v", f.InFlight)
	}
	if !stopBlocks() {
		t.Fatal("stop did not block after the delegation finished")
	}
}

func TestInFlightExpires(t *testing.T) {
	now := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	prev := flow.Now
	flow.Now = func() time.Time { return now }
	t.Cleanup(func() { flow.Now = prev })
	root := newProject(t)
	startFlow(t, root, flow.KindT2)
	if got, reason := decision(t, runHook(t, "pre-tool-use", map[string]any{"cwd": root, "session_id": "A", "tool_name": "Agent",
		"tool_input": map[string]any{"subagent_type": "pcs-explorer"}})); got != "allow" {
		t.Fatalf("launch = %s %s", got, reason)
	}
	if out := runHook(t, "stop", map[string]any{"cwd": root, "session_id": "A"}); out != "" {
		t.Fatalf("stop while running = %q", out)
	}
	now = now.Add(flow.InFlightTTL + time.Second)
	if out := runHook(t, "stop", map[string]any{"cwd": root, "session_id": "A"}); !strings.Contains(out, `"block"`) {
		t.Fatalf("stop after TTL = %q", out)
	}
	// An expired entry no longer denies a relaunch.
	if got, reason := decision(t, runHook(t, "pre-tool-use", map[string]any{"cwd": root, "session_id": "A", "tool_name": "Agent",
		"tool_input": map[string]any{"subagent_type": "pcs-explorer"}})); got != "allow" {
		t.Fatalf("relaunch after TTL = %s %s", got, reason)
	}
	if f := mustActive(t, flow.Store{Root: root}); len(f.InFlight) != 1 {
		t.Fatalf("expired entry not pruned: %+v", f.InFlight)
	}
}

// TestFixRoundThroughHooks drives one fix round through SubagentStop.
func TestFixRoundThroughHooks(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindT2)
	s := flow.Store{Root: root}
	for _, p := range []string{"explore", "strategy", "compose", "apply"} {
		if _, err := s.Record(p, flow.StatusDone, "", "", flow.SourceManual); err != nil {
			t.Fatal(err)
		}
	}
	runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-verifier", "v1", "**Status**: partial", ""))
	if tr := mustActive(t, s).Next(); tr.Agent != "pcs-builder" || !strings.Contains(tr.Instruction, "Fix round 1/3") {
		t.Fatalf("next after partial verify = %+v", tr)
	}
	if got, reason := decision(t, runHook(t, "pre-tool-use", map[string]any{"cwd": root, "tool_name": "Agent",
		"tool_input": map[string]any{"subagent_type": "pcs-builder"}})); got != "allow" {
		t.Fatalf("fix launch = %s %s", got, reason)
	}
	runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-builder", "b2", "**Status**: success", ""))
	f := mustActive(t, s)
	if f.FixRounds != 1 || f.Phases[f.PhaseIndex(flow.PhaseVerify)].Status != flow.StatusPending || len(f.InFlight) != 0 {
		t.Fatalf("after fix: rounds %d, phases %+v, in flight %+v", f.FixRounds, f.Phases, f.InFlight)
	}
	if tr := f.Next(); tr.Agent != "pcs-verifier" {
		t.Fatalf("next after fix = %+v", tr)
	}
}

func mustActive(t *testing.T, s flow.Store) *flow.Flow {
	t.Helper()
	f, err := s.Active()
	if err != nil || f == nil {
		t.Fatalf("active = %v, %v", f, err)
	}
	return f
}
