package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yorth21/pcs/internal/flow"
)

// newGitProject creates a temporary git repository with one committed source
// file, or skips the test when git is not on PATH.
func newGitProject(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	src := filepath.Join(root, "src", "app.tsx")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("export const a = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "-m", "init"}} {
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=pcs", "-c", "user.email=pcs@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

func TestVerifyBindingThroughHooks(t *testing.T) {
	root := newGitProject(t)
	startFlow(t, root, flow.KindT2)
	agents := []string{"pcs-explorer", "pcs-strategist", "pcs-architect", "pcs-builder", "pcs-verifier"}
	for i, agent := range agents {
		runHook(t, "subagent-stop", subagentStopPayload(root, agent, "a"+string(rune('1'+i)), "**Status**: success", ""))
	}
	f, err := (flow.Store{Root: root}).Active()
	if err != nil || f == nil {
		t.Fatalf("active flow: %v", err)
	}
	verify := f.Phases[f.PhaseIndex(flow.PhaseVerify)]
	if verify.Status != flow.StatusDone || !strings.HasPrefix(verify.Fingerprint, "sha256:") {
		t.Fatalf("verify phase = %+v", verify)
	}
	if apply := f.Phases[f.PhaseIndex("apply")]; apply.Fingerprint != "" {
		t.Fatalf("apply phase fingerprinted: %+v", apply)
	}

	// Unchanged tree: stop asks to close.
	if out := runHook(t, "stop", map[string]any{"cwd": root}); !strings.Contains(out, "is complete") {
		t.Fatalf("stop on unchanged tree = %s", out)
	}

	// Source edited after verify: stop treats verify as pending again.
	if err := os.WriteFile(filepath.Join(root, "src", "app.tsx"), []byte("export const a = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := runHook(t, "stop", map[string]any{"cwd": root})
	if !strings.Contains(out, `"block"`) || !strings.Contains(out, "pending verify") || !strings.Contains(out, "pcs-verifier") {
		t.Fatalf("stop after edit = %s", out)
	}
	f, _ = (flow.Store{Root: root}).Active()
	if got := f.Phases[f.PhaseIndex(flow.PhaseVerify)].Status; got != flow.StatusStale {
		t.Fatalf("verify status = %s, want stale", got)
	}

	// A new verifier run rebinds verify to the edited tree.
	runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-verifier", "a9", "**Status**: success", ""))
	if _, err := (flow.Store{Root: root}).Close(false, ""); err != nil {
		t.Fatalf("close after re-verify: %v", err)
	}
}

func TestAuditPhaseFingerprinted(t *testing.T) {
	root := newGitProject(t)
	startFlow(t, root, flow.KindAudit)
	runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-explorer", "a1", "**Status**: success", ""))
	runHook(t, "subagent-stop", subagentStopPayload(root, "pcs-auditor", "a2", "**Status**: partial", ""))
	f, _ := (flow.Store{Root: root}).Active()
	if fp := f.Phases[f.PhaseIndex("audit")].Fingerprint; !strings.HasPrefix(fp, "sha256:") {
		t.Fatalf("audit fingerprint = %q", fp)
	}
	if fp := f.Phases[f.PhaseIndex("explore")].Fingerprint; fp != "" {
		t.Fatalf("explore fingerprint = %q", fp)
	}
}
