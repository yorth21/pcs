package flow

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextTransitions(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		done      []string
		status    map[string]string
		wantKind  string
		wantPhase string
		wantAgent string
		wantText  string
	}{
		{name: "audit starts with explore", kind: KindAudit, wantKind: TransitionDelegate, wantPhase: "explore", wantAgent: "pcs-explorer", wantText: "Delegate to subagent pcs-explorer for feature demo"},
		{name: "audit after explore", kind: KindAudit, done: []string{"explore"}, wantKind: TransitionDelegate, wantPhase: "audit", wantAgent: "pcs-auditor", wantText: "or pcs-verifier"},
		{name: "audit complete", kind: KindAudit, done: []string{"explore", "audit"}, wantKind: TransitionClose, wantText: "pcs flow close"},
		{name: "t2 middle", kind: KindT2, done: []string{"explore", "strategy"}, wantKind: TransitionDelegate, wantPhase: "architecture", wantAgent: "pcs-architect"},
		{name: "t3 blocked phase", kind: KindT3, status: map[string]string{"explore": StatusBlocked}, wantKind: TransitionDelegate, wantPhase: "explore", wantText: "is blocked"},
		{name: "unverified phase", kind: KindT2, status: map[string]string{"explore": StatusUnverified}, wantKind: TransitionDelegate, wantPhase: "explore", wantText: "unverified"},
		{name: "inline closes with budget note", kind: KindInline, wantKind: TransitionClose, wantText: "Inline flow demo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := New("demo", tc.kind)
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range tc.done {
				f.Phases[f.PhaseIndex(d)].Status = StatusDone
			}
			for p, st := range tc.status {
				f.Phases[f.PhaseIndex(p)].Status = st
			}
			tr := f.Next()
			if tr.Schema != TransitionSchema || tr.Kind != tc.wantKind || tr.Phase != tc.wantPhase {
				t.Fatalf("transition = %+v", tr)
			}
			if tc.wantAgent != "" && tr.Agent != tc.wantAgent {
				t.Fatalf("agent = %q, want %q", tr.Agent, tc.wantAgent)
			}
			if !strings.Contains(tr.Instruction, tc.wantText) {
				t.Fatalf("instruction %q lacks %q", tr.Instruction, tc.wantText)
			}
			if tc.kind == KindInline && !strings.Contains(tr.Note, "Inline budget") {
				t.Fatalf("inline note missing: %+v", tr)
			}
		})
	}
	if tr := NoFlowTransition(); tr.Kind != TransitionStart || tr.Command != StartCommand {
		t.Fatalf("no-flow transition = %+v", tr)
	}
}

func TestValidateFeature(t *testing.T) {
	for _, ok := range []string{"demo", "credit-simulator", "a1-b2"} {
		if err := ValidateFeature(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Demo", "a_b", "a--b", "-a", "a-", "../x", "a b"} {
		if err := ValidateFeature(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestStoreLifecycle(t *testing.T) {
	root := t.TempDir()
	s := Store{Root: root}
	if f, err := s.Active(); err != nil || f != nil {
		t.Fatalf("Active() = %v, %v", f, err)
	}
	if _, err := s.Start("demo", KindAudit); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start("other", KindT2); err == nil || !strings.Contains(err.Error(), "already active") {
		t.Fatalf("second start err = %v", err)
	}
	if _, err := s.Close(false, ""); err == nil || !strings.Contains(err.Error(), "pending explore, audit") {
		t.Fatalf("close incomplete err = %v", err)
	}
	if _, err := s.Close(true, "  "); err == nil {
		t.Fatal("abandon without reason accepted")
	}
	if _, err := s.Record("explore", StatusDone, "a1", "", SourceManual); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Record("nope", StatusDone, "", "", SourceManual); err == nil {
		t.Fatal("unknown phase accepted")
	}
	f, err := s.Record("audit", StatusDone, "a2", "done", SourceHook)
	if err != nil {
		t.Fatal(err)
	}
	if f.Phases[0].Source != SourceManual || f.Phases[1].AgentID != "a2" || f.Phases[1].RecordedAt == "" {
		t.Fatalf("records = %+v", f.Phases)
	}
	if _, err := s.Close(false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".pcs", "active")); !os.IsNotExist(err) {
		t.Fatalf("active pointer still present: %v", err)
	}
	// A closed flow no longer blocks a new start.
	if _, err := s.Start("other", KindT2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Close(true, "user cancelled"); err != nil {
		t.Fatal(err)
	}
	closed, err := s.Load("other")
	if err != nil || closed.AbandonedReason != "user cancelled" || closed.ClosedAt == "" {
		t.Fatalf("abandoned flow = %+v, %v", closed, err)
	}
}

func TestResolveRoot(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	deep := filepath.Join(repo, "src", "features", "x")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := ResolveRoot(deep); got != deep {
		t.Fatalf("no markers: root = %q, want %q", got, deep)
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := ResolveRoot(deep); got != repo {
		t.Fatalf("git: root = %q, want %q", got, repo)
	}
	front := filepath.Join(repo, "src")
	if err := os.Mkdir(filepath.Join(front, ".pcs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := ResolveRoot(deep); got != front {
		t.Fatalf(".pcs: root = %q, want %q", got, front)
	}
}

func TestRunCLI(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) (int, string, string) {
		var out, errb bytes.Buffer
		code := RunCLI(append(args, "--cwd", root), &out, &errb, "")
		return code, out.String(), errb.String()
	}
	code, out, _ := run("status", "--json")
	if code != 0 || !strings.Contains(out, `"kind": "start"`) {
		t.Fatalf("status without flow: %d %s", code, out)
	}
	if code, _, errs := run("start", "--feature", "Bad Name", "--kind", "audit"); code != 1 || !strings.Contains(errs, "kebab-case") {
		t.Fatalf("bad feature: %d %s", code, errs)
	}
	if code, _, _ := run("start", "--feature", "demo", "--kind", "audit", "--json"); code != 0 {
		t.Fatal("start failed")
	}
	_, out, _ = run("next", "--json")
	var tr Transition
	if err := json.Unmarshal([]byte(out), &tr); err != nil || tr.Kind != TransitionDelegate || tr.Agent != "pcs-explorer" {
		t.Fatalf("next = %s (%v)", out, err)
	}
	if code, _, errs := run("record", "--phase", "explore", "--status", "unverified"); code != 1 || !strings.Contains(errs, "done or blocked") {
		t.Fatalf("record bad status: %d %s", code, errs)
	}
	if code, _, _ := run("record", "--phase", "explore", "--status", "done", "--agent-id", "x1"); code != 0 {
		t.Fatal("record failed")
	}
	if code, _, errs := run("close"); code != 1 || !strings.Contains(errs, "pending audit") {
		t.Fatalf("close incomplete: %d %s", code, errs)
	}
	if code, out, _ := run("close", "--abandon", "--reason", "user cancelled", "--json"); code != 0 || !strings.Contains(out, "abandoned demo") {
		t.Fatalf("abandon: %d %s", code, out)
	}
	if code, _, errs := run("close"); code != 1 || !strings.Contains(errs, "pcs flow start") {
		t.Fatalf("close without flow: %d %s", code, errs)
	}
}
