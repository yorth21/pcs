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
		case StatusUnverified:
			instr = fmt.Sprintf("Phase %s is unverified: subagent %s returned no Return Envelope status. Re-delegate to subagent %s%s for feature %s and require the Return Envelope.",
				p.Name, agent, agent, alt, f.Feature)
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

// Save writes f atomically.
func (s Store) Save(f *Flow) error {
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
// is active.
func (s Store) Start(feature, kind string) (*Flow, error) {
	active, err := s.Active()
	if err != nil {
		return nil, err
	}
	if active != nil {
		return nil, fmt.Errorf("flow %s (%s) is already active; finish it with `%s` or cancel it with `%s`", active.Feature, active.Kind, CloseCommand, AbandonCommand)
	}
	f, err := New(feature, kind)
	if err != nil {
		return nil, err
	}
	if err := s.Save(f); err != nil {
		return nil, err
	}
	if err := writeAtomic(s.activePath(), []byte(f.Feature+"\n")); err != nil {
		return nil, err
	}
	return f, nil
}

// Record sets the status of phase on the active flow.
func (s Store) Record(phase, status, agentID, envelopeStatus, source string) (*Flow, error) {
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
	switch status {
	case StatusDone, StatusBlocked, StatusUnverified:
	default:
		return nil, fmt.Errorf("invalid phase status %q", status)
	}
	p := &f.Phases[idx]
	p.Status = status
	p.AgentID = agentID
	p.EnvelopeStatus = envelopeStatus
	p.Source = source
	p.RecordedAt = Now().Format(time.RFC3339)
	return f, s.Save(f)
}

// Close closes the active flow. An incomplete flow closes only when abandon is
// true with a non-empty reason.
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
		if reason == "" {
			return nil, errors.New("--abandon requires a non-empty --reason")
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
