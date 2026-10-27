package hook

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yorth21/pcs/internal/flow"
)

// TestAwaitLetsTheTurnEnd walks one interactive blocking question: the Stop
// hook blocks, `pcs flow await` lets the turn end, the user's next prompt
// clears the mark, and Stop blocks again.
func TestAwaitLetsTheTurnEnd(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindT2)
	store := flow.Store{Root: root}
	steps := []struct {
		name      string
		do        func(t *testing.T) string
		wantBlock bool
		wantEmpty bool
		wantCtx   string
		awaiting  bool
	}{
		{name: "stop blocks with the escape", do: func(t *testing.T) string {
			return runHook(t, "stop", map[string]any{"cwd": root, "session_id": "A"})
		}, wantBlock: true},
		{name: "await sets the flag", do: func(t *testing.T) string {
			if _, err := store.Await("A", "Table or cards?"); err != nil {
				t.Fatal(err)
			}
			return ""
		}, wantEmpty: true, awaiting: true},
		{name: "stop does not block while awaiting", do: func(t *testing.T) string {
			return runHook(t, "stop", map[string]any{"cwd": root, "session_id": "A"})
		}, wantEmpty: true, awaiting: true},
		{name: "foreign prompt keeps the flag", do: func(t *testing.T) string {
			return runHook(t, "user-prompt-submit", map[string]any{"cwd": root, "session_id": "B", "prompt": "hi"})
		}, wantCtx: "from another session", awaiting: true},
		{name: "user prompt clears the flag", do: func(t *testing.T) string {
			return runHook(t, "user-prompt-submit", map[string]any{"cwd": root, "session_id": "A", "prompt": "Table"})
		}, wantCtx: AnsweredContext + " PCD flow demo (t2) is active"},
		{name: "stop blocks again", do: func(t *testing.T) string {
			return runHook(t, "stop", map[string]any{"cwd": root, "session_id": "A"})
		}, wantBlock: true},
	}
	for _, st := range steps {
		out := st.do(t)
		switch {
		case st.wantBlock:
			var d stopDecision
			if err := json.Unmarshal([]byte(out), &d); err != nil || d.Decision != "block" || !strings.Contains(d.Reason, AwaitEscape) {
				t.Fatalf("%s: stop = %q", st.name, out)
			}
		case st.wantEmpty:
			if out != "" {
				t.Fatalf("%s: output = %q", st.name, out)
			}
		case st.wantCtx != "":
			var d struct {
				HookSpecificOutput promptContext `json:"hookSpecificOutput"`
			}
			if err := json.Unmarshal([]byte(out), &d); err != nil || !strings.Contains(d.HookSpecificOutput.AdditionalContext, st.wantCtx) {
				t.Fatalf("%s: context = %q", st.name, out)
			}
		}
		f, err := store.Active()
		if err != nil || f == nil {
			t.Fatalf("%s: active = %v, %v", st.name, f, err)
		}
		if (f.AwaitingUser != nil) != st.awaiting {
			t.Fatalf("%s: awaiting = %+v, want %v", st.name, f.AwaitingUser, st.awaiting)
		}
	}
}

func TestPromptWithoutAwaitHasNoAnsweredContext(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindAudit)
	out := runHook(t, "user-prompt-submit", map[string]any{"cwd": root, "session_id": "A", "prompt": "go"})
	if strings.Contains(out, "answered your blocking question") {
		t.Fatalf("context = %q", out)
	}
}

func TestMainThreadAwaitAllowed(t *testing.T) {
	root := newProject(t)
	startFlow(t, root, flow.KindAudit)
	out := runHook(t, "pre-tool-use", map[string]any{"cwd": root, "session_id": "A", "tool_name": "Bash",
		"tool_input": map[string]any{"command": `pcs flow await --question "Table or cards?"`}})
	if got, reason := decision(t, out); got != "allow" {
		t.Fatalf("await = %s %s", got, reason)
	}
}
