// Package hook implements the Claude Code hook commands (`pcs hook <event>`)
// that enforce the deterministic PCD flow. Every hook fails open: malformed
// input or an I/O error never blocks the session.
package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/yorth21/pcs/internal/flow"
)

// Events lists the supported hook events, as passed to `pcs hook <event>`.
var Events = []string{"pre-tool-use", "subagent-stop", "stop", "user-prompt-submit"}

// Input is the Claude Code hook payload read from stdin.
type Input struct {
	SessionID           string         `json:"session_id"`
	TranscriptPath      string         `json:"transcript_path"`
	Cwd                 string         `json:"cwd"`
	HookEventName       string         `json:"hook_event_name"`
	AgentID             string         `json:"agent_id"`
	AgentType           string         `json:"agent_type"`
	ToolName            string         `json:"tool_name"`
	ToolInput           map[string]any `json:"tool_input"`
	StopHookActive      bool           `json:"stop_hook_active"`
	AgentTranscriptPath string         `json:"agent_transcript_path"`
	Prompt              string         `json:"prompt"`
	// LastAssistantMessage is the final text of the subagent (SubagentStop).
	LastAssistantMessage string `json:"last_assistant_message"`
}

// Run executes the hook for event. It always returns 0: denials and blocks are
// expressed as JSON on stdout, and internal errors fail open.
func Run(event string, stdin io.Reader, stdout io.Writer) int {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		logError("", event, fmt.Errorf("read stdin: %w", err))
		return 0
	}
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		logError("", event, fmt.Errorf("parse stdin: %w", err))
		return 0
	}
	if strings.TrimSpace(in.Cwd) == "" {
		in.Cwd, _ = os.Getwd()
	}
	if event == "user-prompt-submit" || event == "subagent-stop" {
		ensureCwdGitignore(in.Cwd)
	}
	root := flow.ResolveRoot(in.Cwd)
	store := flow.Store{Root: root}

	var out any
	switch event {
	case "pre-tool-use":
		out, err = preToolUse(store, in)
	case "subagent-stop":
		err = subagentStop(store, in)
	case "stop":
		out, err = stop(store, in)
	case "user-prompt-submit":
		out, err = userPromptSubmit(store, in)
	default:
		err = fmt.Errorf("unknown hook event %q", event)
	}
	if err != nil {
		logError(root, event, err)
		return 0
	}
	if out != nil {
		_ = json.NewEncoder(stdout).Encode(out)
	}
	return 0
}

// ensureCwdGitignore writes <cwd>/.pcs/.gitignore with "*" when <cwd>/.pcs is
// an existing directory without one. Tools such as the Playwright MCP create
// .pcs/ under the session cwd, which may not be the pcs root. It is best
// effort: it never creates .pcs itself and ignores every error.
func ensureCwdGitignore(cwd string) {
	dir := filepath.Join(cwd, ".pcs")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return
	}
	path := filepath.Join(dir, ".gitignore")
	if _, err := os.Lstat(path); err == nil || !os.IsNotExist(err) {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString("*\n")
	_ = f.Close()
}

// logError appends to <root>/.pcs/hook-errors.log only when <root>/.pcs
// already exists, so a failing hook never creates state in unrelated folders.
func logError(root, event string, err error) {
	if root == "" {
		cwd, _ := os.Getwd()
		root = flow.ResolveRoot(cwd)
	}
	dir := filepath.Join(root, ".pcs")
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		return
	}
	f, openErr := os.OpenFile(filepath.Join(dir, "hook-errors.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if openErr != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s: %v\n", time.Now().UTC().Format(time.RFC3339), event, err)
}

type preToolUseDecision struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

type hookSpecific[T any] struct {
	HookSpecificOutput T `json:"hookSpecificOutput"`
}

func deny(reason string) any {
	return hookSpecific[preToolUseDecision]{preToolUseDecision{
		HookEventName:            "PreToolUse",
		PermissionDecision:       "deny",
		PermissionDecisionReason: reason,
	}}
}

func noFlowReason(action string) string {
	return fmt.Sprintf("PCD: no active flow, so %s is not allowed from the main thread. Run `%s` first (audit = read-only review of existing UI; never inline for audits), then follow `pcs flow next`.", action, flow.StartCommand)
}

func delegateReason(f *flow.Flow, action string) string {
	return fmt.Sprintf("PCD flow %s (%s) is active: %s from the main thread is not allowed; delegate instead. Next: %s", f.Feature, f.Kind, action, f.Next().Instruction)
}

// pcsStateReason is the denial reason for edits under .pcs/.
const pcsStateReason = "PCD: .pcs/ holds CLI-owned flow state; change it only through `pcs flow` commands."

const inlineExhausted = "inline budget exhausted; escalate with " + flow.AbandonCommand + " and pcs flow start --kind t2|audit"

// sessionFlow loads the active flow, claims it for session when it has no
// owner yet, and splits it into the flow owned by (or open to) this session
// and a flow owned by another session. At most one of them is non-nil.
func sessionFlow(store flow.Store, session string) (own, foreign *flow.Flow, err error) {
	f, err := store.Active()
	if err != nil || f == nil {
		return nil, nil, err
	}
	if _, err := store.Claim(f, session); err != nil {
		return nil, nil, err
	}
	if f.ForeignTo(session) {
		return nil, f, nil
	}
	return f, nil, nil
}

// gate holds the flow state one PreToolUse decision is made against. A flow
// owned by another session gates exactly like no flow, with its own reason.
type gate struct {
	store   flow.Store
	f       *flow.Flow
	foreign *flow.Flow
}

func (g gate) noFlowReason(action string) string {
	if g.foreign != nil {
		return foreignReason(g.foreign, action)
	}
	return noFlowReason(action)
}

func foreignReason(f *flow.Flow, action string) string {
	return fmt.Sprintf("PCD: this session has no active flow, so %s is not allowed from the main thread. %s", action, f.ForeignNotice())
}

func preToolUse(store flow.Store, in Input) (any, error) {
	own, foreign, err := sessionFlow(store, in.SessionID)
	if err != nil {
		return nil, err
	}
	// Phase completion is recorded only by the SubagentStop hook: no Claude
	// thread, main or subagent, may self-report it or rewrite flow state.
	switch in.ToolName {
	case "Bash":
		if cmd, _ := in.ToolInput["command"].(string); FlowRecordCommand(cmd) {
			return deny(ManualRecordReason), nil
		}
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		if path := editPath(in); path != "" && within(store.Root, path) && hasDirPrefix(relSlash(store.Root, path), ".pcs") {
			return deny(pcsStateReason), nil
		}
	}
	if in.AgentID != "" {
		// Subagents do the delegated work; they are otherwise never gated.
		return nil, nil
	}
	g := gate{store: store, f: own, foreign: foreign}
	switch in.ToolName {
	case "Agent", "Task":
		return g.agent(in)
	case "Read", "Grep", "Glob":
		path := readPath(in)
		if exemptRead(store.Root, path) {
			return nil, nil
		}
		return g.read(in.ToolName + " on " + relDisplay(store.Root, path))
	case "Bash":
		cmd, _ := in.ToolInput["command"].(string)
		if BashAllowed(cmd) {
			return nil, nil
		}
		return g.read("Bash command `" + truncate(cmd, 80) + "`")
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		return g.edit(in)
	}
	return nil, nil
}

// AllowedGenericAgents lists the non-pcs subagent types the main thread may
// still launch, because they never read or edit project source.
var AllowedGenericAgents = []string{"claude-code-guide", "statusline-setup"}

// NonPCSAgentReason is the denial reason for generic subagents on the main thread.
const NonPCSAgentReason = "Use the pcs-* agent returned by pcs flow next"

func (g gate) agent(in Input) (any, error) {
	agent, _ := in.ToolInput["subagent_type"].(string)
	agent = strings.TrimSpace(agent)
	if !strings.HasPrefix(agent, "pcs-") {
		if slices.Contains(AllowedGenericAgents, agent) {
			return nil, nil
		}
		return g.denyGenericAgent(agent), nil
	}
	f := g.f
	if f == nil {
		return deny(g.noFlowReason("delegating to " + agent)), nil
	}
	if len(f.Phases) == 0 {
		return nil, nil
	}
	idx := f.PhaseForAgent(agent)
	if idx < 0 {
		return deny(fmt.Sprintf("PCD flow %s (%s) has no phase for %s. Next: %s", f.Feature, f.Kind, agent, f.Next().Instruction)), nil
	}
	if missing := f.MissingBefore(idx); len(missing) > 0 {
		return deny(fmt.Sprintf("PCD flow %s (%s): %s (phase %s) cannot run before phase %s is done. Next: %s",
			f.Feature, f.Kind, agent, f.Phases[idx].Name, missing[0], f.Next().Instruction)), nil
	}
	return nil, nil
}

// denyGenericAgent blocks generic subagents (Explore, general-purpose, fork,
// or an omitted subagent_type) so the coordinator cannot bypass PCD phases.
func (g gate) denyGenericAgent(agent string) any {
	name := agent
	if name == "" {
		name = "(default)"
	}
	next := flow.NoFlowTransition().Instruction
	switch {
	case g.f != nil:
		next = g.f.Next().Instruction
	case g.foreign != nil:
		next = g.foreign.ForeignNotice()
	}
	return deny(fmt.Sprintf("PCD: subagent %s is not allowed from the main thread. %s. Next: %s", name, NonPCSAgentReason, next))
}

func (g gate) read(action string) (any, error) {
	f := g.f
	if f == nil {
		return deny(g.noFlowReason(action)), nil
	}
	if f.Kind != flow.KindInline {
		return deny(delegateReason(f, action)), nil
	}
	if f.InlineReads >= flow.InlineReadBudget {
		return deny(fmt.Sprintf("PCD flow %s (inline): %s denied: %s", f.Feature, action, inlineExhausted)), nil
	}
	f.InlineReads++
	return nil, g.store.Save(f)
}

func (g gate) edit(in Input) (any, error) {
	path := editPath(in)
	if path == "" || !within(g.store.Root, path) {
		return nil, nil
	}
	rel := relSlash(g.store.Root, path)
	if hasDirPrefix(rel, "pcs") || hasDirPrefix(rel, ".atl") {
		return nil, nil
	}
	f := g.f
	action := in.ToolName + " on " + rel
	if f == nil {
		return deny(g.noFlowReason(action)), nil
	}
	if f.Kind != flow.KindInline {
		return deny(delegateReason(f, action)), nil
	}
	for _, existing := range f.InlineFiles {
		if samePath(existing, rel) {
			return nil, nil
		}
	}
	if len(f.InlineFiles) >= flow.InlineFileBudget {
		return deny(fmt.Sprintf("PCD flow %s (inline) already edits %s; %s denied: %s", f.Feature, strings.Join(f.InlineFiles, ", "), action, inlineExhausted)), nil
	}
	f.InlineFiles = append(f.InlineFiles, rel)
	return nil, g.store.Save(f)
}

func readPath(in Input) string {
	for _, key := range []string{"file_path", "path"} {
		if p, _ := in.ToolInput[key].(string); strings.TrimSpace(p) != "" {
			return absFrom(in.Cwd, p)
		}
	}
	if in.ToolName == "Glob" {
		if pat, _ := in.ToolInput["pattern"].(string); pat != "" {
			// Use the literal directory prefix of the pattern.
			prefix := pat
			if i := strings.IndexAny(pat, "*?[{"); i >= 0 {
				prefix = pat[:i]
			}
			return absFrom(in.Cwd, prefix)
		}
	}
	return absFrom(in.Cwd, "")
}

func editPath(in Input) string {
	for _, key := range []string{"file_path", "notebook_path"} {
		if p, _ := in.ToolInput[key].(string); strings.TrimSpace(p) != "" {
			return absFrom(in.Cwd, p)
		}
	}
	return ""
}

func absFrom(cwd, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return filepath.Clean(cwd)
	}
	p = filepath.FromSlash(p)
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	return filepath.Clean(p)
}

var docNames = map[string]bool{"claude.md": true, "agents.md": true, "readme.md": true}

func exemptRead(root, path string) bool {
	if !within(root, path) {
		return true
	}
	rel := relSlash(root, path)
	if hasDirPrefix(rel, "pcs") || hasDirPrefix(rel, ".pcs") || hasDirPrefix(rel, ".atl") {
		return true
	}
	return docNames[strings.ToLower(filepath.Base(path))]
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel != ".." && !strings.HasPrefix(rel, "../")
}

func relSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func relDisplay(root, path string) string {
	if within(root, path) {
		return relSlash(root, path)
	}
	return filepath.ToSlash(path)
}

func hasDirPrefix(rel, dir string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		rel, dir = strings.ToLower(rel), strings.ToLower(dir)
	}
	return rel == dir || strings.HasPrefix(rel, dir+"/")
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func subagentStop(store flow.Store, in Input) error {
	f, _, err := sessionFlow(store, in.SessionID)
	if err != nil || f == nil || !strings.HasPrefix(in.AgentType, "pcs-") {
		// A flow owned by another session never receives this session's runs.
		return err
	}
	idx := f.PhaseForAgent(in.AgentType)
	if idx < 0 {
		return nil
	}
	envelope := ParseEnvelopeStatus(in.LastAssistantMessage)
	if envelope == "" {
		envelope = EnvelopeStatus(in.AgentTranscriptPath)
	}
	phase := f.Phases[idx].Name
	status, note := flow.StatusFromEnvelope(phase, envelope)
	_, err = store.RecordRun(phase, flow.Run{
		AgentID: in.AgentID, AgentType: in.AgentType, Status: status,
		EnvelopeStatus: envelope, Source: flow.SourceHook, Note: note,
	})
	return err
}

// envelopeStatusPattern matches a Return Envelope status line anywhere in the
// text: an optional blockquote or list bullet, optional markdown emphasis
// around the key and the value, then one of the known statuses, e.g.
// "**Status**: success", "- **Status:** partial", "Status: blocked", or
// "**Status**: **success**".
var envelopeStatusPattern = regexp.MustCompile("(?im)^[ \t>]*(?:[-*+][ \t]+)?[*_`]{0,3}status[*_`]{0,3}[ \t]*:[ \t]*[*_`]{0,3}[ \t]*(success|partial|blocked|done)(?:[^a-z0-9]|$)")

// ParseEnvelopeStatus returns the lowercase status of the last Return
// Envelope status line in text, or "" when there is none.
func ParseEnvelopeStatus(text string) string {
	matches := envelopeStatusPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.ToLower(matches[len(matches)-1][1])
}

// EnvelopeStatus returns the Return Envelope status of the last assistant
// text message in the JSONL transcript at path, or "" when the transcript or
// the status line is missing.
func EnvelopeStatus(path string) string {
	return ParseEnvelopeStatus(lastAssistantText(path))
}

// ManualRecordReason is the denial reason for main-thread `pcs flow record`.
const ManualRecordReason = "Phase completion is recorded by the SubagentStop hook from the subagent Return Envelope; no main thread or subagent may run pcs flow record. If it was not recorded, relaunch the agent so it ends with `**Status**: success|partial|blocked`. A human can run pcs flow record in their own terminal outside Claude."

// flowRecordPattern finds a pcs binary (any path, optionally quoted, with or
// without .exe) invoked with `flow record`, also inside `sh -c "..."` strings.
var flowRecordPattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_.-])pcs(?:\.exe)?['"]?\s+flow\s+record(?:\s|$|['";|&])`)

// FlowRecordCommand reports whether command runs `pcs flow record`.
func FlowRecordCommand(command string) bool {
	return flowRecordPattern.MatchString(command)
}

func lastAssistantText(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	last := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	for sc.Scan() {
		var entry struct {
			Type    string `json:"type"`
			Message struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &entry) != nil {
			continue
		}
		if entry.Type != "assistant" && entry.Message.Role != "assistant" {
			continue
		}
		if t := contentText(entry.Message.Content); strings.TrimSpace(t) != "" {
			last = t
		}
	}
	return last
}

func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

type stopDecision struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// AwaitEscape tells a blocked coordinator how to end the turn to wait for
// the user's decision.
const AwaitEscape = "If you need the user's decision, run `" + flow.AwaitCommand + "`, present the question, and end the turn."

func stop(store flow.Store, in Input) (any, error) {
	f, _, err := sessionFlow(store, in.SessionID)
	if in.StopHookActive || err != nil || f == nil || f.AwaitingUser != nil {
		// A flow owned by another session never blocks this session, and a
		// flow awaiting the user's answer lets the turn end.
		return nil, err
	}
	if pending := f.Pending(); len(pending) > 0 {
		return stopDecision{Decision: "block", Reason: fmt.Sprintf(
			"PCD flow %s (%s) is incomplete: pending %s. Next: %s. %s If the user explicitly cancelled, run: %s",
			f.Feature, f.Kind, strings.Join(pending, ", "), strings.TrimSuffix(f.Next().Instruction, "."), AwaitEscape, flow.AbandonCommand)}, nil
	}
	return stopDecision{Decision: "block", Reason: fmt.Sprintf(
		"PCD flow %s (%s) is complete: run `%s` before ending the turn. %s", f.Feature, f.Kind, flow.CloseCommand, AwaitEscape)}, nil
}

type promptContext struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// NoFlowRule is the UserPromptSubmit context injected when no flow is active.
const NoFlowRule = "PCD: before touching project files run `pcs flow start --feature <kebab> --kind audit|inline|t2|t3` (audit = read-only review of existing UI; never inline for audits), then follow `pcs flow next`."

// AnsweredContext prefixes the UserPromptSubmit context when the prompt
// answers the blocking question recorded by `pcs flow await`.
const AnsweredContext = "The user answered your blocking question: continue the flow with `pcs flow next`."

func userPromptSubmit(store flow.Store, in Input) (any, error) {
	f, foreign, err := sessionFlow(store, in.SessionID)
	if err != nil {
		return nil, err
	}
	ctx := NoFlowRule
	if foreign != nil {
		ctx = NoFlowRule + " " + foreign.ForeignNotice()
	}
	answered := ""
	if f != nil {
		cleared, err := store.ClearAwait(f)
		if err != nil {
			return nil, err
		}
		if cleared {
			answered = AnsweredContext + " "
		}
	}
	if f != nil {
		pending := f.Pending()
		state := "all mandatory phases done"
		if len(pending) > 0 {
			state = "pending " + strings.Join(pending, ", ")
		}
		ctx = fmt.Sprintf("%sPCD flow %s (%s) is active: %s. Next: %s", answered, f.Feature, f.Kind, state, f.Next().Instruction)
	}
	return hookSpecific[promptContext]{promptContext{HookEventName: "UserPromptSubmit", AdditionalContext: ctx}}, nil
}
