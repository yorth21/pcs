package flow

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSegment(t *testing.T) {
	f, err := New("checkout", KindT2)
	if err != nil {
		t.Fatal(err)
	}
	if got := Segment(f, ""); got != "pcs:checkout t2 ▸ explore 1/5" {
		t.Fatalf("fresh segment = %q", got)
	}
	for i := 0; i < 3; i++ {
		f.Phases[i].Status = StatusDone
	}
	if got := Segment(f, ""); got != "pcs:checkout t2 ▸ apply 4/5" {
		t.Fatalf("segment = %q", got)
	}
	f.Phases[3].Status = StatusDone
	f.Phases[4].Status = StatusStale
	if got := Segment(f, ""); got != "pcs:checkout t2 ▸ verify 5/5 stale" {
		t.Fatalf("stale segment = %q", got)
	}
	f.Phases[4].Status = StatusDone
	if got := Segment(f, ""); got != "pcs:checkout t2 ▸ close" {
		t.Fatalf("complete segment = %q", got)
	}
	f.AwaitingUser = &Await{Question: "q"}
	if got := Segment(f, ""); got != "pcs:checkout t2 ▸ awaiting user" {
		t.Fatalf("awaiting segment = %q", got)
	}
	f.OwnerSessionID = "A"
	if got := Segment(f, "B"); got != "pcs:checkout t2 ▸ other session" {
		t.Fatalf("foreign segment = %q", got)
	}
	if got := Segment(nil, ""); got != "" {
		t.Fatalf("nil segment = %q", got)
	}
}

func statusLineStdin(t *testing.T, dir string) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(map[string]any{"session_id": "s1", "cwd": "/elsewhere", "workspace": map[string]any{"current_dir": dir}})
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b)
}

func TestRunStatusLine(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if code := RunStatusLine(nil, statusLineStdin(t, root), &out, ""); code != 0 || out.Len() != 0 {
		t.Fatalf("no flow: code %d output %q", code, out.String())
	}
	if _, err := (Store{Root: root}).Start("demo", KindAudit); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	RunStatusLine(nil, statusLineStdin(t, sub), &out, "")
	if got := strings.TrimSpace(out.String()); got != "pcs:demo audit ▸ explore 1/2" {
		t.Fatalf("stdin workspace segment = %q", got)
	}
	out.Reset()
	RunStatusLine([]string{"--cwd", root}, strings.NewReader("not json"), &out, "")
	if got := strings.TrimSpace(out.String()); got != "pcs:demo audit ▸ explore 1/2" {
		t.Fatalf("--cwd segment = %q", got)
	}
	// Corrupt state never fails the status line.
	if err := os.WriteFile(filepath.Join(root, ".pcs", "flows", "demo.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := RunStatusLine([]string{"--cwd", root}, nil, &out, ""); code != 0 || out.Len() != 0 {
		t.Fatalf("corrupt state: code %d output %q", code, out.String())
	}
}

func TestRunStatusLineWrap(t *testing.T) {
	root := t.TempDir()
	if _, err := (Store{Root: root}).Start("demo", KindT2); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(t.TempDir(), ".pcs-state.json")
	writeState := func(cmd string) {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"scope": "profile", StatusLineBaseField: cmd})
		if err := os.WriteFile(state, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(args ...string) string {
		t.Helper()
		var out bytes.Buffer
		if code := RunStatusLineWrap(args, statusLineStdin(t, root), &out, ""); code != 0 {
			t.Fatalf("exit code %d", code)
		}
		return strings.TrimRight(out.String(), "\r\n")
	}
	seg := "pcs:demo t2 ▸ explore 1/5"

	writeState("echo base")
	if got := wrap("--state", state); got != "base"+StatusLineSeparator+seg {
		t.Fatalf("wrapped = %q", got)
	}
	// The base command receives the same stdin.
	writeState("cat")
	if got := wrap("--state", state); !strings.Contains(got, `"current_dir"`) || !strings.HasSuffix(got, StatusLineSeparator+seg) {
		t.Fatalf("stdin passthrough = %q", got)
	}
	// A failing base still prints the segment.
	writeState("exit 3")
	if got := wrap("--state", state); got != seg {
		t.Fatalf("failing base = %q", got)
	}
	// Without a base command, only the segment.
	if got := wrap("--state", filepath.Join(t.TempDir(), "missing.json")); got != seg {
		t.Fatalf("no base = %q", got)
	}
	// Without an active flow, only the base output.
	if _, err := (Store{Root: root}).Close(true, "test cleanup of the flow"); err != nil {
		t.Fatal(err)
	}
	if got := wrap("--base", "echo base"); got != "base" {
		t.Fatalf("no flow = %q", got)
	}
}
