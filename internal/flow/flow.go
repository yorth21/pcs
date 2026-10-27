// Package flow implements the deterministic Product-Centered Development (PCD)
// flow state machine: the model proposes a flow, the CLI and hooks record which
// mandatory phases ran, and every step returns a typed next transition.
package flow

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Schema identifiers for persisted flows and emitted transitions.
const (
	Schema           = "pcs.flow/v1"
	TransitionSchema = "pcs.flow.transition/v1"
)

// Flow kinds.
const (
	KindAudit  = "audit"
	KindInline = "inline"
	KindT2     = "t2"
	KindT3     = "t3"
)

// Phase statuses.
const (
	StatusPending    = "pending"
	StatusDone       = "done"
	StatusBlocked    = "blocked"
	StatusUnverified = "unverified"
)

// Record sources.
const (
	SourceHook   = "hook"
	SourceManual = "manual"
)

// Transition kinds.
const (
	TransitionStart    = "start"
	TransitionDelegate = "delegate"
	TransitionClose    = "close"
)

// InlineReadBudget is the maximum number of counted main-thread source reads
// an inline (Tier 1) flow may perform.
const InlineReadBudget = 3

// InlineFileBudget is the maximum number of distinct project files an inline
// flow may edit from the main thread.
const InlineFileBudget = 1

// StartCommand is the command template the model runs when no flow is active.
const StartCommand = "pcs flow start --feature <name> --kind audit|inline|t2|t3"

// CloseCommand closes a complete flow.
const CloseCommand = "pcs flow close"

// AbandonCommand closes an incomplete flow after an explicit user cancellation.
const AbandonCommand = `pcs flow close --abandon --reason "..."`

// AdoptCommand hands the active flow to the session that runs it.
const AdoptCommand = "pcs flow adopt"

// StaleAfter is how long an active flow may go without a state write before
// `pcs flow start` auto-abandons it.
const StaleAfter = 12 * time.Hour

// StaleReason is the abandon reason recorded for an auto-abandoned stale flow.
const StaleReason = "stale: no activity for >12h"

// Now is the clock used for timestamps; tests replace it.
var Now = func() time.Time { return time.Now().UTC() }

// ErrNoActiveFlow is returned when an operation needs an active flow.
var ErrNoActiveFlow = errors.New("no active PCD flow")

// Phase is one mandatory, ordered step of a flow.
type Phase struct {
	Name           string   `json:"name"`
	Agents         []string `json:"agents"`
	Status         string   `json:"status"`
	AgentID        string   `json:"agent_id,omitempty"`
	RecordedAt     string   `json:"recorded_at,omitempty"`
	EnvelopeStatus string   `json:"envelope_status,omitempty"`
	Source         string   `json:"source,omitempty"`
	Note           string   `json:"note,omitempty"`
	// Runs lists every recorded result for this phase in order, so duplicate
	// or failed subagent runs stay visible after the phase is done.
	Runs []Run `json:"runs,omitempty"`
}

// Run is one recorded result for a phase.
type Run struct {
	AgentID        string `json:"agent_id,omitempty"`
	AgentType      string `json:"agent_type,omitempty"`
	Status         string `json:"status"`
	EnvelopeStatus string `json:"envelope_status,omitempty"`
	Source         string `json:"source,omitempty"`
	Note           string `json:"note,omitempty"`
	RecordedAt     string `json:"recorded_at"`
}

// PhaseVerify is the build-flow phase whose Severity 3-4 hard gate requires a
// clean Return Envelope.
const PhaseVerify = "verify"

// VerifyPartialNote explains why a partial verify result blocks the phase.
const VerifyPartialNote = "verify requires Return Envelope status success; partial means unresolved findings (Severity 3-4 hard gate). Fix them and relaunch the verifier."

// StatusFromEnvelope maps a Return Envelope status to the phase status it
// earns for phase, with a note when the mapping needs explanation. verify is
// done only on success (or the legacy done); every other phase accepts
// success, partial, or done.
func StatusFromEnvelope(phase, envelope string) (status, note string) {
	switch envelope {
	case "success", "done":
		return StatusDone, ""
	case "partial":
		if phase == PhaseVerify {
			return StatusBlocked, VerifyPartialNote
		}
		return StatusDone, ""
	case "blocked":
		return StatusBlocked, ""
	}
	return StatusUnverified, ""
}

// Flow is the persisted state of one feature flow.
type Flow struct {
	Schema          string   `json:"schema"`
	Feature         string   `json:"feature"`
	Kind            string   `json:"kind"`
	CreatedAt       string   `json:"created_at"`
	Phases          []Phase  `json:"phases"`
	InlineReads     int      `json:"inline_reads"`
	InlineFiles     []string `json:"inline_files"`
	ClosedAt        string   `json:"closed_at,omitempty"`
	AbandonedReason string   `json:"abandoned_reason,omitempty"`
	// OwnerSessionID is the Claude Code session bound to the flow. It is empty
	// until the first hook event of a session claims it, and again after adopt.
	OwnerSessionID string `json:"owner_session_id,omitempty"`
	ClaimedAt      string `json:"claimed_at,omitempty"`
	// UpdatedAt is refreshed on every state write.
	UpdatedAt string `json:"updated_at,omitempty"`
}

// ForeignTo reports whether the flow is owned by a session other than
// session. An unowned flow, or an unknown session, is never foreign.
func (f *Flow) ForeignTo(session string) bool {
	return f != nil && f.OwnerSessionID != "" && session != "" && f.OwnerSessionID != session
}

// ForeignNotice explains that the active flow belongs to another session and
// how to resume or discard it.
func (f *Flow) ForeignNotice() string {
	return fmt.Sprintf("An unfinished PCD flow `%s` from another session is active. Resume it with `%s` or discard it with `%s`, then start yours.", f.Feature, AdoptCommand, AbandonCommand)
}

// lastActivity returns the time of the last state write, falling back to the
// creation time for state files written before updated_at existed.
func (f *Flow) lastActivity() (time.Time, bool) {
	for _, ts := range []string{f.UpdatedAt, f.CreatedAt} {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Stale reports whether the flow has had no state write for more than
// StaleAfter.
func (f *Flow) Stale() bool {
	t, ok := f.lastActivity()
	return ok && Now().Sub(t) > StaleAfter
}

// Transition is the typed next step the model must take.
type Transition struct {
	Schema      string `json:"schema"`
	Kind        string `json:"kind"`
	Phase       string `json:"phase,omitempty"`
	Agent       string `json:"agent,omitempty"`
	Instruction string `json:"instruction"`
	Command     string `json:"command,omitempty"`
	Note        string `json:"note,omitempty"`
}

type phaseDef struct {
	name   string
	agents []string
}

var buildPhases = []phaseDef{
	{"explore", []string{"pcs-explorer"}},
	{"strategy", []string{"pcs-strategist"}},
	{"architecture", []string{"pcs-architect"}},
	{"apply", []string{"pcs-builder"}},
	{"verify", []string{"pcs-verifier"}},
}

var kindPhases = map[string][]phaseDef{
	KindAudit: {
		{"explore", []string{"pcs-explorer"}},
		{"audit", []string{"pcs-auditor", "pcs-verifier"}},
	},
	KindInline: nil,
	KindT2:     buildPhases,
	KindT3:     buildPhases,
}

// Kinds lists the supported flow kinds in display order.
var Kinds = []string{KindAudit, KindInline, KindT2, KindT3}

var featurePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateFeature reports whether name is a kebab-case feature name.
func ValidateFeature(name string) error {
	if len(name) > 80 || !featurePattern.MatchString(name) {
		return fmt.Errorf("invalid feature name %q: use kebab-case (lowercase letters, digits, single hyphens), at most 80 characters", name)
	}
	return nil
}

// New builds a fresh flow for feature and kind.
func New(feature, kind string) (*Flow, error) {
	if err := ValidateFeature(feature); err != nil {
		return nil, err
	}
	defs, ok := kindPhases[kind]
	if !ok {
		return nil, fmt.Errorf("invalid kind %q (supported: %s)", kind, strings.Join(Kinds, ", "))
	}
	f := &Flow{
		Schema:      Schema,
		Feature:     feature,
		Kind:        kind,
		CreatedAt:   Now().Format(time.RFC3339),
		Phases:      []Phase{},
		InlineFiles: []string{},
	}
	for _, d := range defs {
		f.Phases = append(f.Phases, Phase{Name: d.name, Agents: slices.Clone(d.agents), Status: StatusPending})
	}
	return f, nil
}

// Closed reports whether the flow was closed or abandoned.
func (f *Flow) Closed() bool { return f != nil && f.ClosedAt != "" }

// PhaseIndex returns the index of the named phase, or -1.
func (f *Flow) PhaseIndex(name string) int {
	for i, p := range f.Phases {
		if p.Name == name {
			return i
		}
	}
	return -1
}

// PhaseForAgent returns the index of the first phase served by agent, or -1.
func (f *Flow) PhaseForAgent(agent string) int {
	for i, p := range f.Phases {
		if slices.Contains(p.Agents, agent) {
			return i
		}
	}
	return -1
}

// MissingBefore lists the phases before idx that are not done, in order.
func (f *Flow) MissingBefore(idx int) []string {
	var out []string
	for i := 0; i < idx && i < len(f.Phases); i++ {
		if f.Phases[i].Status != StatusDone {
			out = append(out, f.Phases[i].Name)
		}
	}
	return out
}

// Pending lists every mandatory phase that is not done, in order.
func (f *Flow) Pending() []string {
	return f.MissingBefore(len(f.Phases))
}

// Complete reports whether every mandatory phase is done.
func (f *Flow) Complete() bool { return len(f.Pending()) == 0 }

// UnverifiedRunInstruction is the duplicate-run guard appended to a delegate
// instruction when a previous run of agent ended without a status line.
func UnverifiedRunInstruction(agent string) string {
	return fmt.Sprintf("The previous %s run ended without a Return Envelope Status line; relaunch it and require the final `**Status**:` line.", agent)
}

// lastUnverifiedAgent reports whether any recorded run of p was unverified and
// returns the agent type of the latest such run (possibly empty).
func (p Phase) lastUnverifiedAgent() (string, bool) {
	for i := len(p.Runs) - 1; i >= 0; i-- {
		if p.Runs[i].Status == StatusUnverified {
			return p.Runs[i].AgentType, true
		}
	}
	return "", false
}

// NoFlowTransition is the transition returned when no flow is active.
func NoFlowTransition() Transition {
	return Transition{
		Schema:      TransitionSchema,
		Kind:        TransitionStart,
		Command:     StartCommand,
		Instruction: "No PCD flow is active. Run `" + StartCommand + "` (audit = read-only review of existing UI; inline = Tier 1 single-file fix; t2/t3 = build), then follow `pcs flow next`.",
	}
}

// Next returns the typed next transition for f.
func (f *Flow) Next() Transition {
	if f == nil || f.Closed() {
		return NoFlowTransition()
	}
	for _, p := range f.Phases {
		if p.Status == StatusDone {
			continue
		}
		agent := p.Agents[0]
		alt := ""
		if len(p.Agents) > 1 {
			alt = " (or " + strings.Join(p.Agents[1:], ", ") + ")"
		}
		instr := fmt.Sprintf("Delegate to subagent %s%s for feature %s (phase %s of the %s flow) with the Agent tool, subagent_type %q. Its Return Envelope status is recorded automatically when it stops.",
			agent, alt, f.Feature, p.Name, f.Kind, agent)
		switch p.Status {
		case StatusBlocked:
			instr = fmt.Sprintf("Phase %s is blocked (Return Envelope status %q). Resolve the blocker, then re-delegate to subagent %s%s for feature %s, or ask the user and run %s if they cancel.",
				p.Name, p.EnvelopeStatus, agent, alt, f.Feature, AbandonCommand)
			if p.Note != "" {
				instr += " Note: " + p.Note
			}
		case StatusUnverified:
			instr = fmt.Sprintf("Phase %s is unverified (feature %s, subagent_type %q).", p.Name, f.Feature, agent)
		}
		if unverified, ok := p.lastUnverifiedAgent(); ok || p.Status == StatusUnverified {
			if unverified == "" {
				unverified = agent
			}
			instr += " " + UnverifiedRunInstruction(unverified)
		}
		return Transition{Schema: TransitionSchema, Kind: TransitionDelegate, Phase: p.Name, Agent: agent, Instruction: instr}
	}
	t := Transition{
		Schema:      TransitionSchema,
		Kind:        TransitionClose,
		Command:     CloseCommand,
		Instruction: fmt.Sprintf("All mandatory phases of %s (%s) are done. Run `%s`.", f.Feature, f.Kind, CloseCommand),
	}
	if f.Kind == KindInline {
		t.Instruction = fmt.Sprintf("Inline flow %s: work in the main thread, then run `%s`.", f.Feature, CloseCommand)
		t.Note = fmt.Sprintf("Inline budget: at most %d counted source reads (used %d) and %d edited project file (used %d). Escalate with %s and pcs flow start --kind t2|audit when it is not enough.",
			InlineReadBudget, f.InlineReads, InlineFileBudget, len(f.InlineFiles), AbandonCommand)
	}
	return t
}

// Store persists flows under <root>/.pcs.
type Store struct {
	Root string
}

func (s Store) dir() string              { return filepath.Join(s.Root, ".pcs") }
func (s Store) activePath() string       { return filepath.Join(s.dir(), "active") }
func (s Store) flowPath(f string) string { return filepath.Join(s.dir(), "flows", f+".json") }

// Load reads the flow for feature.
func (s Store) Load(feature string) (*Flow, error) {
	if err := ValidateFeature(feature); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.flowPath(feature))
	if err != nil {
		return nil, err
	}
	var f Flow
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("invalid flow state %s: %w", s.flowPath(feature), err)
	}
	if f.Schema != Schema {
		return nil, fmt.Errorf("unsupported flow schema %q in %s", f.Schema, s.flowPath(feature))
	}
	return &f, nil
}

// Save refreshes f.UpdatedAt and writes f atomically.
func (s Store) Save(f *Flow) error {
	f.UpdatedAt = Now().Format(time.RFC3339)
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.flowPath(f.Feature), append(b, '\n'))
}

// Active returns the active open flow, or nil when none is active. A pointer to
// a missing or closed flow reads as no active flow.
func (s Store) Active() (*Flow, error) {
	b, err := os.ReadFile(s.activePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	name := strings.TrimSpace(string(b))
	if name == "" {
		return nil, nil
	}
	f, err := s.Load(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if f.Closed() {
		return nil, nil
	}
	return f, nil
}

// Start creates and activates a new flow. It refuses while another open flow
// is active (see StartReplacingStale).
func (s Store) Start(feature, kind string) (*Flow, error) {
	f, _, err := s.StartReplacingStale(feature, kind)
	return f, err
}

// StartReplacingStale creates and activates a new flow. An active flow with no
// state write for more than StaleAfter is abandoned with StaleReason and
// returned as abandoned; any other active flow makes it refuse.
func (s Store) StartReplacingStale(feature, kind string) (f, abandoned *Flow, err error) {
	active, err := s.Active()
	if err != nil {
		return nil, nil, err
	}
	f, err = New(feature, kind)
	if err != nil {
		return nil, nil, err
	}
	if active != nil {
		if !active.Stale() {
			return nil, nil, fmt.Errorf("flow %s (%s) is already active; finish it with `%s`, resume it in this session with `%s`, or cancel it with `%s`",
				active.Feature, active.Kind, CloseCommand, AdoptCommand, AbandonCommand)
		}
		active.AbandonedReason = StaleReason
		active.ClosedAt = Now().Format(time.RFC3339)
		if err := s.Save(active); err != nil {
			return nil, nil, err
		}
		abandoned = active
	}
	if err := s.ensureGitignore(); err != nil {
		return nil, nil, err
	}
	if err := s.Save(f); err != nil {
		return nil, nil, err
	}
	if err := writeAtomic(s.activePath(), []byte(f.Feature+"\n")); err != nil {
		return nil, nil, err
	}
	return f, abandoned, nil
}

// Claim binds the open flow f to session when f has no owner yet, and saves
// it. It reports whether f changed. A nil or closed flow, or an empty session,
// is left untouched.
func (s Store) Claim(f *Flow, session string) (bool, error) {
	session = strings.TrimSpace(session)
	if f == nil || f.Closed() || f.OwnerSessionID != "" || session == "" {
		return false, nil
	}
	f.OwnerSessionID = session
	f.ClaimedAt = Now().Format(time.RFC3339)
	return true, s.Save(f)
}

// Adopt clears the owner of the active flow, so the next hook event of the
// session that ran it claims the flow (the CLI has no session id).
func (s Store) Adopt() (*Flow, error) {
	f, err := s.Active()
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, ErrNoActiveFlow
	}
	f.OwnerSessionID = ""
	f.ClaimedAt = ""
	return f, s.Save(f)
}

// ensureGitignore writes <root>/.pcs/.gitignore with "*" when it is missing,
// so local flow state and Playwright artifacts never reach version control.
func (s Store) ensureGitignore() error {
	path := filepath.Join(s.dir(), ".gitignore")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeAtomic(path, []byte("*\n"))
}

// Record sets the status of phase on the active flow (see RecordRun).
func (s Store) Record(phase, status, agentID, envelopeStatus, source string) (*Flow, error) {
	return s.RecordRun(phase, Run{AgentID: agentID, Status: status, EnvelopeStatus: envelopeStatus, Source: source})
}

// RecordRun appends run to phase on the active flow and updates the phase
// status monotonically: a done phase never goes back to blocked or
// unverified, while a blocked or unverified phase can still become done.
func (s Store) RecordRun(phase string, run Run) (*Flow, error) {
	f, err := s.Active()
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, ErrNoActiveFlow
	}
	idx := f.PhaseIndex(phase)
	if idx < 0 {
		names := make([]string, 0, len(f.Phases))
		for _, p := range f.Phases {
			names = append(names, p.Name)
		}
		return nil, fmt.Errorf("flow %s (%s) has no phase %q (phases: %s)", f.Feature, f.Kind, phase, strings.Join(names, ", "))
	}
	switch run.Status {
	case StatusDone, StatusBlocked, StatusUnverified:
	default:
		return nil, fmt.Errorf("invalid phase status %q", run.Status)
	}
	run.RecordedAt = Now().Format(time.RFC3339)
	p := &f.Phases[idx]
	p.Runs = append(p.Runs, run)
	if p.Status == StatusDone && run.Status != StatusDone {
		return f, s.Save(f)
	}
	p.Status = run.Status
	p.AgentID = run.AgentID
	p.EnvelopeStatus = run.EnvelopeStatus
	p.Source = run.Source
	p.Note = run.Note
	p.RecordedAt = run.RecordedAt
	return f, s.Save(f)
}

// MinAbandonReason is the minimum length of an --abandon reason, so an
// abandonment always carries an explanation a reviewer can check.
const MinAbandonReason = 15

// Close closes the active flow. An incomplete flow closes only when abandon is
// true with a reason of at least MinAbandonReason characters.
func (s Store) Close(abandon bool, reason string) (*Flow, error) {
	f, err := s.Active()
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, ErrNoActiveFlow
	}
	reason = strings.TrimSpace(reason)
	if abandon {
		if len([]rune(reason)) < MinAbandonReason {
			return nil, fmt.Errorf("--abandon requires a --reason of at least %d characters stating the user cancellation", MinAbandonReason)
		}
		f.AbandonedReason = reason
	} else if pending := f.Pending(); len(pending) > 0 {
		return nil, fmt.Errorf("flow %s (%s) is incomplete: pending %s; next: %s; if the user explicitly cancelled, run %s",
			f.Feature, f.Kind, strings.Join(pending, ", "), f.Next().Instruction, AbandonCommand)
	}
	f.ClosedAt = Now().Format(time.RFC3339)
	if err := s.Save(f); err != nil {
		return nil, err
	}
	if err := os.Remove(s.activePath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return f, nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// ResolveRoot returns the project root for cwd: the nearest ancestor holding a
// .pcs directory, else the nearest ancestor holding .git, else cwd itself.
func ResolveRoot(cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		cwd, _ = os.Getwd()
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		abs = filepath.Clean(cwd)
	}
	if dir, ok := findUp(abs, ".pcs", true); ok {
		return dir
	}
	if dir, ok := findUp(abs, ".git", false); ok {
		return dir
	}
	return abs
}

func findUp(start, name string, wantDir bool) (string, bool) {
	dir := start
	for {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && (!wantDir || info.IsDir()) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
