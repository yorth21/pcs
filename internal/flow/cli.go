package flow

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// StatusView is the JSON shape printed by every pcs flow command.
type StatusView struct {
	Root           string     `json:"root"`
	Active         bool       `json:"active"`
	Flow           *Flow      `json:"flow,omitempty"`
	Pending        []string   `json:"pending"`
	NextTransition Transition `json:"next_transition"`
	Message        string     `json:"message,omitempty"`
}

// RunCLI executes `pcs flow <subcommand>` and returns the exit code.
func RunCLI(args []string, stdout, stderr io.Writer, workDir string) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	sub, rest := args[0], args[1:]
	fs := flag.NewFlagSet("flow "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	cwd := fs.String("cwd", workDir, "Project directory (the root is resolved from it)")
	asJSON := fs.Bool("json", false, "Output JSON")
	var feature, kind, phase, status, agentID, reason, question, session *string
	var abandon *bool
	switch sub {
	case "start":
		feature = fs.String("feature", "", "Kebab-case feature name")
		kind = fs.String("kind", "", "Flow kind: audit, inline, t2, or t3")
	case "status", "next", "adopt":
	case "await":
		question = fs.String("question", "", "The one blocking question you will present to the user")
		session = fs.String("session", os.Getenv(SessionEnv), "Claude Code session id (default: $"+SessionEnv+")")
	case "record":
		phase = fs.String("phase", "", "Phase name")
		status = fs.String("status", "", "Phase status: done or blocked")
		agentID = fs.String("agent-id", "", "Subagent identifier")
	case "close":
		abandon = fs.Bool("abandon", false, "Close an incomplete flow after an explicit user cancellation")
		reason = fs.String("reason", "", "Reason for --abandon")
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "pcs flow: unknown subcommand %q\n%s", sub, usage)
		return 2
	}
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	store := Store{Root: ResolveRoot(*cwd)}
	fail := func(err error) int {
		if *asJSON {
			_ = writeJSON(stdout, map[string]any{"error": err.Error(), "root": store.Root})
		}
		fmt.Fprintf(stderr, "pcs flow %s: %v\n", sub, err)
		return 1
	}

	var f *Flow
	var err error
	msg := ""
	switch sub {
	case "start":
		var abandoned *Flow
		f, abandoned, err = store.StartReplacingStale(strings.TrimSpace(*feature), strings.TrimSpace(*kind))
		msg = "started"
		if err == nil && abandoned != nil {
			msg = fmt.Sprintf("started; auto-abandoned flow %s (%s): %s", abandoned.Feature, abandoned.Kind, abandoned.AbandonedReason)
		}
	case "status", "next":
		f, err = store.Active()
	case "adopt":
		f, err = store.Adopt()
		if err == nil {
			msg = "adopted " + f.Feature + ": the next hook event of this session claims it"
		}
	case "await":
		f, err = store.Await(*session, *question)
		if err == nil {
			msg = "awaiting the user on " + f.Feature + ": present the question to the user and end the turn; the Stop hook does not block until the user answers"
		}
	case "record":
		st := strings.TrimSpace(*status)
		if st != StatusDone && st != StatusBlocked {
			return fail(fmt.Errorf("--status must be done or blocked, got %q", st))
		}
		f, err = store.Record(strings.TrimSpace(*phase), st, strings.TrimSpace(*agentID), "", SourceManual)
		msg = "recorded (source: manual)"
	case "close":
		var closed *Flow
		closed, err = store.Close(*abandon, *reason)
		if err == nil {
			msg = "closed " + closed.Feature
			if closed.AbandonedReason != "" {
				msg = "abandoned " + closed.Feature + ": " + closed.AbandonedReason
			}
		}
	}
	if err != nil {
		if errors.Is(err, ErrNoActiveFlow) {
			err = fmt.Errorf("%w; start one with `%s`", err, StartCommand)
		}
		return fail(err)
	}

	view := StatusView{Root: store.Root, Active: f != nil, Flow: f, Pending: []string{}, Message: msg}
	if f != nil {
		view.Pending = append(view.Pending, f.Pending()...)
	}
	view.NextTransition = f.Next()

	if *asJSON {
		if sub == "next" {
			return writeJSON(stdout, view.NextTransition)
		}
		return writeJSON(stdout, view)
	}
	if sub == "await" {
		fmt.Fprintf(stdout, "pcs flow: %s\nQuestion: %s\n", msg, f.AwaitingUser.Question)
		return 0
	}
	if sub == "next" {
		fmt.Fprintln(stdout, view.NextTransition.Instruction)
		if view.NextTransition.Note != "" {
			fmt.Fprintln(stdout, view.NextTransition.Note)
		}
		return 0
	}
	printView(stdout, view)
	return 0
}

func printView(w io.Writer, v StatusView) {
	if v.Message != "" {
		fmt.Fprintf(w, "pcs flow: %s\n", v.Message)
	}
	fmt.Fprintf(w, "Root: %s\n", v.Root)
	if v.Flow == nil {
		fmt.Fprintln(w, "Active flow: none")
	} else {
		fmt.Fprintf(w, "Active flow: %s (%s)\n", v.Flow.Feature, v.Flow.Kind)
		owner := v.Flow.OwnerSessionID
		if owner == "" {
			owner = "unclaimed (the next hook event claims it)"
		}
		fmt.Fprintf(w, "  owner session: %s\n", owner)
		if a := v.Flow.AwaitingUser; a != nil {
			fmt.Fprintf(w, "  awaiting user since %s: %s\n", a.AskedAt, a.Question)
		}
		for _, p := range v.Flow.Phases {
			fmt.Fprintf(w, "  - %s [%s] %s\n", p.Name, p.Status, strings.Join(p.Agents, " | "))
		}
		if v.Flow.Kind == KindInline {
			fmt.Fprintf(w, "  inline reads: %d/%d, inline files: %v\n", v.Flow.InlineReads, InlineReadBudget, v.Flow.InlineFiles)
		}
	}
	fmt.Fprintf(w, "Next: %s\n", v.NextTransition.Instruction)
	if v.NextTransition.Note != "" {
		fmt.Fprintf(w, "Note: %s\n", v.NextTransition.Note)
	}
}

func writeJSON(w io.Writer, v any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return 1
	}
	return 0
}

const usage = `usage: pcs flow <subcommand> [--cwd <dir>] [--json]

Subcommands:
  start --feature <kebab> --kind audit|inline|t2|t3   Start and activate a PCD flow (auto-abandons a flow idle for >12h)
  status                                              Show the active flow
  next                                                Print the typed next transition
  adopt                                               Hand the active flow to the session that runs it
  await --question "..." [--session id]               Wait for the user's answer to one blocking question
                                                      (the Stop hook does not block until the user replies)
  record --phase <p> --status done|blocked [--agent-id x]
                                                      Manually record a phase (source: manual)
  close [--abandon --reason "..."]                    Close the active flow
  statusline [--cwd <dir>]                            Print the one-line active flow segment (never fails)
`
