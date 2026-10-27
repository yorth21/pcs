package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusFromEnvelope(t *testing.T) {
	tests := []struct {
		phase, envelope, want string
		wantNote              bool
	}{
		{"explore", "success", StatusDone, false},
		{"explore", "partial", StatusDone, false},
		{"explore", "done", StatusDone, false},
		{"explore", "blocked", StatusBlocked, false},
		{"explore", "", StatusUnverified, false},
		{"audit", "partial", StatusDone, false},
		{PhaseVerify, "success", StatusDone, false},
		{PhaseVerify, "partial", StatusBlocked, true},
		{PhaseVerify, "blocked", StatusBlocked, false},
		{PhaseVerify, "", StatusUnverified, false},
	}
	for _, tc := range tests {
		got, note := StatusFromEnvelope(tc.phase, tc.envelope)
		if got != tc.want || (note != "") != tc.wantNote {
			t.Errorf("StatusFromEnvelope(%q, %q) = %q, %q", tc.phase, tc.envelope, got, note)
		}
	}
}

func TestRecordRunMonotonic(t *testing.T) {
	tests := []struct {
		name string
		runs []string
		want string
	}{
		{name: "done survives unverified", runs: []string{StatusDone, StatusUnverified}, want: StatusDone},
		{name: "done survives blocked", runs: []string{StatusDone, StatusBlocked}, want: StatusDone},
		{name: "blocked becomes done", runs: []string{StatusBlocked, StatusDone}, want: StatusDone},
		{name: "unverified becomes done", runs: []string{StatusUnverified, StatusDone}, want: StatusDone},
		{name: "unverified becomes blocked", runs: []string{StatusUnverified, StatusBlocked}, want: StatusBlocked},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Store{Root: t.TempDir()}
			if _, err := s.Start("demo", KindAudit); err != nil {
				t.Fatal(err)
			}
			var f *Flow
			var err error
			for i, st := range tc.runs {
				f, err = s.RecordRun("explore", Run{AgentID: fmt.Sprintf("a%d", i), AgentType: "pcs-explorer", Status: st, Source: SourceHook})
				if err != nil {
					t.Fatal(err)
				}
			}
			p := f.Phases[0]
			if p.Status != tc.want || len(p.Runs) != len(tc.runs) {
				t.Fatalf("phase = %+v", p)
			}
			for i, r := range p.Runs {
				if r.AgentID != fmt.Sprintf("a%d", i) || r.RecordedAt == "" {
					t.Fatalf("run %d = %+v", i, r)
				}
			}
		})
	}
}

func TestNextDuplicateRunGuard(t *testing.T) {
	want := "The previous pcs-explorer run ended without a Return Envelope Status line; relaunch it and require the final `**Status**:` line."
	f, _ := New("demo", KindAudit)
	f.Phases[0].Status = StatusUnverified
	f.Phases[0].Runs = []Run{{AgentID: "a1", AgentType: "pcs-explorer", Status: StatusUnverified}}
	if tr := f.Next(); tr.Kind != TransitionDelegate || !strings.Contains(tr.Instruction, want) {
		t.Fatalf("unverified next = %+v", tr)
	}
	f.Phases[0].Status = StatusBlocked
	if tr := f.Next(); !strings.Contains(tr.Instruction, "is blocked") || !strings.Contains(tr.Instruction, want) {
		t.Fatalf("blocked with unverified run = %+v", tr)
	}
	f.Phases[0].Status = StatusDone
	if tr := f.Next(); strings.Contains(tr.Instruction, "previous") {
		t.Fatalf("done phase still warns: %+v", tr)
	}
	b, _ := New("demo", KindT2)
	for i := range 4 {
		b.Phases[i].Status = StatusDone
	}
	b.Phases[4].Status = StatusBlocked
	b.Phases[4].Note = VerifyPartialNote
	if tr := b.Next(); tr.Phase != PhaseApply || tr.Agent != "pcs-builder" || !strings.Contains(tr.Note, "Severity 3-4") {
		t.Fatalf("verify partial next = %+v", tr)
	}
}

func TestStartWritesGitignoreAndAbandonReasonLength(t *testing.T) {
	root := t.TempDir()
	s := Store{Root: root}
	if _, err := s.Start("demo", KindAudit); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".pcs", ".gitignore"))
	if err != nil || string(b) != "*\n" {
		t.Fatalf(".gitignore = %q, %v", b, err)
	}
	if _, err := s.Close(true, "short reason"); err == nil || !strings.Contains(err.Error(), "at least 15") {
		t.Fatalf("short abandon err = %v", err)
	}
	if _, err := s.Close(true, "user cancelled the audit"); err != nil {
		t.Fatal(err)
	}
	// An existing .gitignore is kept.
	if err := os.WriteFile(filepath.Join(root, ".pcs", ".gitignore"), []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start("other", KindT2); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, ".pcs", ".gitignore")); string(b) != "custom\n" {
		t.Fatalf(".gitignore overwritten: %q", b)
	}
}
