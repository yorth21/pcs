package flow

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"
)

// fixedClock sets Now to a controllable time for the test.
func fixedClock(t *testing.T, start time.Time) *time.Time {
	t.Helper()
	now := start
	prev := Now
	Now = func() time.Time { return now }
	t.Cleanup(func() { Now = prev })
	return &now
}

func TestBuildPhaseOrder(t *testing.T) {
	for _, kind := range []string{KindT2, KindT3} {
		f, err := New("demo", kind)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, p := range f.Phases {
			names = append(names, p.Name)
		}
		want := []string{"explore", "strategy", "compose", "apply", "verify"}
		if !slices.Equal(names, want) {
			t.Fatalf("%s phases = %v, want %v", kind, names, want)
		}
		if got := f.Phases[f.PhaseIndex(PhaseCompose)].Agents; !slices.Equal(got, []string{"pcs-architect"}) {
			t.Fatalf("compose agents = %v", got)
		}
	}
}

// TestLegacyArchitecturePhase keeps flows persisted with the old phase name
// working: phases are stored per file and never migrated.
func TestLegacyArchitecturePhase(t *testing.T) {
	s := Store{Root: t.TempDir()}
	f, err := s.Start("demo", KindT2)
	if err != nil {
		t.Fatal(err)
	}
	f.Phases[2].Name = "architecture"
	if err := s.Save(f); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"explore", "strategy"} {
		if _, err := s.Record(p, StatusDone, "", "", SourceManual); err != nil {
			t.Fatal(err)
		}
	}
	f, _ = s.Active()
	if tr := f.Next(); tr.Phase != "architecture" || tr.Agent != "pcs-architect" {
		t.Fatalf("legacy next = %+v", tr)
	}
	if idx := f.PhaseForAgent("pcs-architect"); idx != 2 {
		t.Fatalf("legacy architect phase index = %d", idx)
	}
}

// blockedVerifyFlow starts a t2 flow with every phase before verify done and
// verify blocked by a partial verifier run.
func blockedVerifyFlow(t *testing.T) Store {
	t.Helper()
	s := Store{Root: t.TempDir()}
	if _, err := s.Start("demo", KindT2); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"explore", "strategy", "compose", "apply"} {
		if _, err := s.Record(p, StatusDone, "", "", SourceManual); err != nil {
			t.Fatal(err)
		}
	}
	verifierRun(t, s, "partial")
	return s
}

func verifierRun(t *testing.T, s Store, envelope string) *Flow {
	t.Helper()
	status, note := StatusFromEnvelope(PhaseVerify, envelope)
	f, err := s.RecordRun(PhaseVerify, Run{AgentType: "pcs-verifier", Status: status, EnvelopeStatus: envelope, Source: SourceHook, Note: note})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func builderRun(t *testing.T, s Store, envelope string) *Flow {
	t.Helper()
	status, note := StatusFromEnvelope(PhaseApply, envelope)
	f, err := s.RecordRun(PhaseApply, Run{AgentType: "pcs-builder", Status: status, EnvelopeStatus: envelope, Source: SourceHook, Note: note})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFixLoop(t *testing.T) {
	s := blockedVerifyFlow(t)
	f, _ := s.Active()
	for round := 1; round <= MaxFixRounds; round++ {
		tr := f.Next()
		if tr.Kind != TransitionDelegate || tr.Phase != PhaseApply || tr.Agent != "pcs-builder" {
			t.Fatalf("round %d: next = %+v", round, tr)
		}
		for _, part := range []string{"Fix round " + itoa(round) + "/3 of demo", "fix mode", "pcs/specs/demo.md", "verify re-runs automatically"} {
			if !strings.Contains(tr.Instruction, part) {
				t.Fatalf("round %d: instruction %q lacks %q", round, tr.Instruction, part)
			}
		}
		f = builderRun(t, s, "success")
		v := f.Phases[f.PhaseIndex(PhaseVerify)]
		if f.FixRounds != round || v.Status != StatusPending {
			t.Fatalf("round %d: fix rounds %d, verify %+v", round, f.FixRounds, v)
		}
		if len(v.Runs) != round {
			t.Fatalf("round %d: verify run history = %d, want %d", round, len(v.Runs), round)
		}
		if tr := f.Next(); tr.Kind != TransitionDelegate || tr.Agent != "pcs-verifier" {
			t.Fatalf("round %d: next after fix = %+v", round, tr)
		}
		f = verifierRun(t, s, "blocked")
	}
	tr := f.Next()
	if tr.Kind != TransitionAwait || tr.Command != AwaitCommand || tr.Phase != PhaseVerify {
		t.Fatalf("after cap: next = %+v", tr)
	}
	for _, part := range []string{"3/3 fix rounds", "blocking findings", "pcs flow await", "pcs-builder", AbandonCommand} {
		if !strings.Contains(tr.Instruction, part) {
			t.Fatalf("cap instruction %q lacks %q", tr.Instruction, part)
		}
	}
	// The user asks for more work: one more builder run still re-verifies.
	f = builderRun(t, s, "success")
	if f.FixRounds != MaxFixRounds+1 || f.Phases[f.PhaseIndex(PhaseVerify)].Status != StatusPending {
		t.Fatalf("extra round: %+v", f)
	}
	f = verifierRun(t, s, "success")
	if !f.Complete() {
		t.Fatalf("flow not complete: %v", f.Pending())
	}
	// A builder run after verify passed is not a fix round.
	f = builderRun(t, s, "success")
	if f.FixRounds != MaxFixRounds+1 || f.Phases[f.PhaseIndex(PhaseVerify)].Status != StatusDone {
		t.Fatalf("builder after done verify: %+v", f)
	}
}

func TestFixLoopBuilderNotSuccess(t *testing.T) {
	for _, envelope := range []string{"partial", "blocked", ""} {
		t.Run("envelope "+envelope, func(t *testing.T) {
			s := blockedVerifyFlow(t)
			f := builderRun(t, s, envelope)
			if f.FixRounds != 1 {
				t.Fatalf("fix rounds = %d, want 1 (the attempt is consumed)", f.FixRounds)
			}
			if v := f.Phases[f.PhaseIndex(PhaseVerify)]; v.Status != StatusBlocked {
				t.Fatalf("verify = %+v, want blocked", v)
			}
			if a := f.Phases[f.PhaseIndex(PhaseApply)]; a.Status != StatusDone {
				t.Fatalf("apply regressed: %+v", a)
			}
			tr := f.Next()
			if tr.Agent != "pcs-builder" || !strings.Contains(tr.Instruction, "Fix round 2/3") || !strings.Contains(tr.Note, "previous fix attempt") {
				t.Fatalf("next = %+v", tr)
			}
		})
	}
}

func TestInFlight(t *testing.T) {
	now := fixedClock(t, time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC))
	s := Store{Root: t.TempDir()}
	f, err := s.Start("demo", KindAudit)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.StartDelegation(f, "pcs-explorer", "explore"); err != nil {
		t.Fatal(err)
	}
	f, _ = s.Active()
	if !f.HasLiveInFlight() {
		t.Fatal("in-flight entry not persisted")
	}
	if _, ok := f.Running("pcs-explorer", "explore"); !ok {
		t.Fatal("Running(pcs-explorer, explore) = false")
	}
	tr := f.Next()
	if tr.Kind != TransitionWait || tr.Agent != "pcs-explorer" || !strings.Contains(tr.Instruction, "do not relaunch") {
		t.Fatalf("next while running = %+v", tr)
	}
	// The entry expires after the TTL.
	*now = now.Add(InFlightTTL + time.Minute)
	if f.HasLiveInFlight() {
		t.Fatal("expired entry still live")
	}
	if tr := f.Next(); tr.Kind != TransitionDelegate {
		t.Fatalf("next after TTL = %+v", tr)
	}
	// Starting another delegation prunes the expired one.
	if err := s.StartDelegation(f, "pcs-explorer", "explore"); err != nil {
		t.Fatal(err)
	}
	if err := s.StartDelegation(f, "pcs-explorer", "explore"); err != nil {
		t.Fatal(err)
	}
	f, _ = s.Active()
	if len(f.InFlight) != 2 {
		t.Fatalf("in flight = %+v, want 2 live entries", f.InFlight)
	}
	// A recorded run removes the oldest matching entry, whatever its status.
	f, err = s.RecordRun("explore", Run{AgentType: "pcs-explorer", Status: StatusBlocked, EnvelopeStatus: "blocked", Source: SourceHook})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.InFlight) != 1 {
		t.Fatalf("in flight after one stop = %+v", f.InFlight)
	}
	f, _ = s.RecordRun("explore", Run{AgentType: "pcs-explorer", Status: StatusDone, EnvelopeStatus: "success", Source: SourceHook})
	if len(f.InFlight) != 0 || f.HasLiveInFlight() {
		t.Fatalf("in flight after both stops = %+v", f.InFlight)
	}
}

func itoa(n int) string { return string(rune('0' + n)) }

func TestSegmentFixAndRunning(t *testing.T) {
	fixedClock(t, time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC))
	s := blockedVerifyFlow(t)
	f := builderRun(t, s, "success")
	if got := Segment(f, ""); got != "pcs:demo t2 ▸ verify 5/5 fix 1/3" {
		t.Fatalf("segment = %q", got)
	}
	if err := s.StartDelegation(f, "pcs-verifier", PhaseVerify); err != nil {
		t.Fatal(err)
	}
	if got := Segment(f, ""); got != "pcs:demo t2 ▸ verify 5/5 running fix 1/3" {
		t.Fatalf("running segment = %q", got)
	}
}

func TestStatusShowsFixRoundsAndRunning(t *testing.T) {
	s := blockedVerifyFlow(t)
	f := builderRun(t, s, "success")
	if err := s.StartDelegation(f, "pcs-verifier", PhaseVerify); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := RunCLI([]string{"status", "--cwd", s.Root}, &out, &errb, ""); code != 0 {
		t.Fatalf("status: %d %s", code, errb.String())
	}
	for _, part := range []string{"fix rounds: 1/3", "running: pcs-verifier (phase verify)", "Next: pcs-verifier is running"} {
		if !strings.Contains(out.String(), part) {
			t.Fatalf("status output lacks %q:\n%s", part, out.String())
		}
	}
	out.Reset()
	RunCLI([]string{"next", "--json", "--cwd", s.Root}, &out, &errb, "")
	if !strings.Contains(out.String(), `"kind": "wait"`) {
		t.Fatalf("next --json = %s", out.String())
	}
}
