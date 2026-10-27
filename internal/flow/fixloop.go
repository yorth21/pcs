package flow

import (
	"fmt"
	"strings"
	"time"
)

// Build-flow phase names used by the verify -> fix loop.
const (
	PhaseCompose = "compose"
	PhaseApply   = "apply"
)

// MaxFixRounds is the number of automatic verify -> fix rounds a t2/t3 flow
// runs before the coordinator must ask the user.
const MaxFixRounds = 3

// InFlightTTL is how long an in-flight delegation keeps the Stop hook from
// blocking; older entries are ignored and pruned.
const InFlightTTL = 30 * time.Minute

// InFlight is one pcs-* delegation launched from the main thread that has not
// stopped yet.
type InFlight struct {
	AgentType string `json:"agent_type"`
	Phase     string `json:"phase"`
	StartedAt string `json:"started_at"`
}

// live reports whether e started less than InFlightTTL ago. An unparseable
// timestamp is never live.
func (e InFlight) live() bool {
	t, err := time.Parse(time.RFC3339, e.StartedAt)
	return err == nil && Now().Sub(t) < InFlightTTL
}

// LiveInFlight returns the in-flight delegations younger than InFlightTTL,
// oldest first.
func (f *Flow) LiveInFlight() []InFlight {
	if f == nil {
		return nil
	}
	var out []InFlight
	for _, e := range f.InFlight {
		if e.live() {
			out = append(out, e)
		}
	}
	return out
}

// HasLiveInFlight reports whether a delegation of f is still running.
func (f *Flow) HasLiveInFlight() bool { return len(f.LiveInFlight()) > 0 }

// Running returns the live in-flight delegation of agentType for phase.
func (f *Flow) Running(agentType, phase string) (InFlight, bool) {
	for _, e := range f.LiveInFlight() {
		if e.AgentType == agentType && e.Phase == phase {
			return e, true
		}
	}
	return InFlight{}, false
}

// StartDelegation prunes expired in-flight entries of f, records agentType as
// running for phase, and saves f.
func (s Store) StartDelegation(f *Flow, agentType, phase string) error {
	f.InFlight = append(f.LiveInFlight(), InFlight{AgentType: agentType, Phase: phase, StartedAt: Now().Format(time.RFC3339)})
	return s.Save(f)
}

// finishDelegation removes the oldest in-flight entry of agentType, live or
// expired, and prunes the expired ones.
func (f *Flow) finishDelegation(agentType string) {
	out := []InFlight{}
	removed := false
	for _, e := range f.InFlight {
		if !removed && e.AgentType == agentType {
			removed = true
			continue
		}
		if e.live() {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		out = nil
	}
	f.InFlight = out
}

// WaitInstruction tells the coordinator a delegation is still running.
func WaitInstruction(e InFlight) string {
	return fmt.Sprintf("%s is running (phase %s, started %s); wait for its completion notification, do not relaunch it. Its Return Envelope status is recorded automatically when it stops.", e.AgentType, e.Phase, e.StartedAt)
}

func waitTransition(f *Flow, e InFlight) Transition {
	return Transition{Schema: TransitionSchema, Kind: TransitionWait, Phase: e.Phase, Agent: e.AgentType,
		Instruction: WaitInstruction(e) + " Feature " + f.Feature + " (" + f.Kind + ")."}
}

// fixLoop reports whether f runs the verify -> fix loop and returns the
// indexes of its apply and verify phases.
func (f *Flow) fixLoop() (apply, verify int, ok bool) {
	if f.Kind != KindT2 && f.Kind != KindT3 {
		return -1, -1, false
	}
	apply, verify = f.PhaseIndex(PhaseApply), f.PhaseIndex(PhaseVerify)
	return apply, verify, apply >= 0 && verify >= 0 && len(f.Phases[apply].Agents) > 0
}

// applyFixRound counts an apply run recorded while verify is blocked as one
// fix round. Only a successful run (envelope success or done, or a manual
// done) resets verify to pending, keeping its run history, so the verifier
// runs again; any other result consumes the round and leaves verify blocked.
func (f *Flow) applyFixRound(phase string, run Run) {
	applyIdx, verifyIdx, ok := f.fixLoop()
	if !ok || phase != f.Phases[applyIdx].Name || f.Phases[verifyIdx].Status != StatusBlocked {
		return
	}
	f.FixRounds++
	if !fixSucceeded(run) {
		return
	}
	v := &f.Phases[verifyIdx]
	v.Status = StatusPending
	v.AgentID, v.EnvelopeStatus, v.Source, v.Fingerprint, v.RecordedAt = "", "", "", "", ""
	v.Note = fmt.Sprintf("fix round %d applied; verify runs again", f.FixRounds)
}

// fixSucceeded reports whether an apply run fixed the findings: envelope
// success or done, or a manual done without an envelope.
func fixSucceeded(run Run) bool {
	return run.Status == StatusDone && (run.EnvelopeStatus == "" || run.EnvelopeStatus == "success" || run.EnvelopeStatus == "done")
}

// fixTransition returns the next step of a blocked verify: the next fix round
// while rounds remain, else a request to ask the user.
func (f *Flow) fixTransition() (Transition, bool) {
	applyIdx, verifyIdx, ok := f.fixLoop()
	if !ok {
		return Transition{}, false
	}
	builder := f.Phases[applyIdx].Agents[0]
	spec := "pcs/specs/" + f.Feature + ".md"
	if f.FixRounds >= MaxFixRounds {
		return Transition{
			Schema:  TransitionSchema,
			Kind:    TransitionAwait,
			Phase:   PhaseVerify,
			Command: AwaitCommand,
			Instruction: fmt.Sprintf("Verify of %s is still blocked after %d/%d fix rounds. Present the remaining blocking findings from the Verification Report in %s to the user, run `%s`, and end the turn. If the user asks for more fixes, delegate to subagent %s in fix mode with the Agent tool, subagent_type %q (verify re-runs automatically after it); if the user cancels, run %s.",
				f.Feature, f.FixRounds, MaxFixRounds, spec, AwaitCommand, builder, builder, AbandonCommand),
		}, true
	}
	t := Transition{
		Schema: TransitionSchema,
		Kind:   TransitionDelegate,
		Phase:  f.Phases[applyIdx].Name,
		Agent:  builder,
		Instruction: fmt.Sprintf("Fix round %d/%d of %s: delegate to subagent %s in fix mode with the Agent tool, subagent_type %q; pass the verifier's blocking findings from the Verification Report in %s; verify re-runs automatically after it.",
			f.FixRounds+1, MaxFixRounds, f.Feature, builder, builder, spec),
	}
	t.Note = f.Phases[verifyIdx].Note
	if runs := f.Phases[applyIdx].Runs; f.FixRounds > 0 && len(runs) > 0 {
		if last := runs[len(runs)-1]; !fixSucceeded(last) {
			env := last.EnvelopeStatus
			if env == "" {
				env = "missing"
			}
			t.Note = strings.TrimSpace(fmt.Sprintf("The previous fix attempt ended with Return Envelope status %s; verify stays blocked (%s). %s", env, f.Phases[verifyIdx].EnvelopeStatus, t.Note))
		}
	}
	return t, true
}
