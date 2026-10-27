package flow

import (
	"bytes"
	"strings"
	"testing"
)

func TestAwaitCLI(t *testing.T) {
	tests := []struct {
		name      string
		owner     string
		args      []string
		wantCode  int
		wantOut   string
		wantErr   string
		wantAwait bool
		wantOwner string
	}{
		{name: "sets the flag for the owning session", owner: "A", args: []string{"--question", "Table or cards?", "--session", "A"},
			wantOut: "end the turn", wantAwait: true, wantOwner: "A"},
		{name: "claims an unowned flow", args: []string{"--question", "Table or cards?", "--session", "A"},
			wantOut: "Question: Table or cards?", wantAwait: true, wantOwner: "A"},
		{name: "requires a question", owner: "A", args: []string{"--question", "   ", "--session", "A"},
			wantCode: 1, wantErr: "--question is required", wantOwner: "A"},
		{name: "refuses a foreign flow", owner: "A", args: []string{"--question", "Table or cards?", "--session", "B"},
			wantCode: 1, wantErr: "from another session", wantOwner: "A"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Store{Root: t.TempDir()}
			f, err := s.Start("demo", KindAudit)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Claim(f, tc.owner); err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			code := RunCLI(append([]string{"await", "--cwd", s.Root}, tc.args...), &out, &errOut, s.Root)
			if code != tc.wantCode {
				t.Fatalf("code = %d, want %d (stderr %q)", code, tc.wantCode, errOut.String())
			}
			if !strings.Contains(out.String(), tc.wantOut) || !strings.Contains(errOut.String(), tc.wantErr) {
				t.Fatalf("stdout %q stderr %q", out.String(), errOut.String())
			}
			got, _ := s.Active()
			if (got.AwaitingUser != nil) != tc.wantAwait || got.OwnerSessionID != tc.wantOwner {
				t.Fatalf("flow = %+v awaiting %+v", got, got.AwaitingUser)
			}
			if tc.wantAwait && (got.AwaitingUser.Question != "Table or cards?" || got.AwaitingUser.AskedAt == "") {
				t.Fatalf("awaiting = %+v", got.AwaitingUser)
			}
		})
	}
}

func TestAwaitWithoutFlow(t *testing.T) {
	var out, errOut bytes.Buffer
	root := t.TempDir()
	if code := RunCLI([]string{"await", "--question", "q", "--session", "A"}, &out, &errOut, root); code != 1 || !strings.Contains(errOut.String(), "no active PCD flow") {
		t.Fatalf("code %d stderr %q", code, errOut.String())
	}
}

func TestClearAwait(t *testing.T) {
	s := Store{Root: t.TempDir()}
	if _, err := s.Start("demo", KindT2); err != nil {
		t.Fatal(err)
	}
	f, err := s.Await("A", "Which density?")
	if err != nil {
		t.Fatal(err)
	}
	if cleared, err := s.ClearAwait(f); !cleared || err != nil {
		t.Fatalf("clear = %v, %v", cleared, err)
	}
	got, _ := s.Active()
	if got.AwaitingUser != nil {
		t.Fatalf("awaiting not cleared: %+v", got.AwaitingUser)
	}
	if cleared, err := s.ClearAwait(got); cleared || err != nil {
		t.Fatalf("second clear = %v, %v", cleared, err)
	}
}
