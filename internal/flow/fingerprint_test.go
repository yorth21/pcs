package flow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newGitRepo creates a temporary git repository with one committed source
// file, or skips the test when git is not on PATH.
func newGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=pcs", "-c", "user.email=pcs@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	writeFile(t, filepath.Join(root, "src", "app.tsx"), "export const a = 1\n")
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFingerprint(t *testing.T) {
	root := newGitRepo(t)
	base := Fingerprint(root)
	if !strings.HasPrefix(base, "sha256:") {
		t.Fatalf("Fingerprint = %q", base)
	}
	if again := Fingerprint(root); again != base {
		t.Fatalf("unstable fingerprint %q != %q", again, base)
	}
	// Flow state and PCD artifacts never change it.
	writeFile(t, filepath.Join(root, ".pcs", "flows", "x.json"), "{}")
	writeFile(t, filepath.Join(root, "pcs", "specs", "x.md"), "# report\n")
	if got := Fingerprint(root); got != base {
		t.Fatalf("excluded paths changed the fingerprint")
	}
	writeFile(t, filepath.Join(root, "src", "app.tsx"), "export const a = 2\n")
	edited := Fingerprint(root)
	if edited == base {
		t.Fatal("tracked edit did not change the fingerprint")
	}
	writeFile(t, filepath.Join(root, "src", "new.tsx"), "x\n")
	added := Fingerprint(root)
	if added == edited {
		t.Fatal("untracked file did not change the fingerprint")
	}
	writeFile(t, filepath.Join(root, "src", "new.tsx"), "y\n")
	if Fingerprint(root) == added {
		t.Fatal("untracked content edit did not change the fingerprint")
	}
	if got := Fingerprint(t.TempDir()); got != FingerprintUnavailable {
		t.Fatalf("non-repository fingerprint = %q", got)
	}
}

// completeBuildFlow records every phase of a t2 flow as done, with verify
// bound to the current working tree.
func completeBuildFlow(t *testing.T, s Store) {
	t.Helper()
	if _, err := s.Start("demo", KindT2); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"explore", "strategy", "compose", "apply", "verify"} {
		run := Run{Status: StatusDone, EnvelopeStatus: "success", Source: SourceHook}
		if p == PhaseVerify {
			run.Fingerprint = Fingerprint(s.Root)
		}
		if _, err := s.RecordRun(p, run); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCloseRefusesSourceChangedAfterVerify(t *testing.T) {
	root := newGitRepo(t)
	s := Store{Root: root}
	completeBuildFlow(t, s)
	writeFile(t, filepath.Join(root, "src", "app.tsx"), "export const a = 3\n")

	_, err := s.Close(false, "")
	if err == nil || !strings.Contains(err.Error(), StaleVerifyMessage) {
		t.Fatalf("Close error = %v", err)
	}
	f, _ := s.Active()
	if f == nil || f.Phases[f.PhaseIndex(PhaseVerify)].Status != StatusStale {
		t.Fatalf("verify not stale: %+v", f)
	}
	next := f.Next()
	if next.Kind != TransitionDelegate || next.Agent != "pcs-verifier" || !strings.Contains(next.Instruction, StaleVerifyMessage) {
		t.Fatalf("next = %+v", next)
	}

	// A fresh verify of the new tree closes the flow.
	if _, err := s.RecordRun(PhaseVerify, Run{Status: StatusDone, EnvelopeStatus: "success", Fingerprint: Fingerprint(root)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Close(false, ""); err != nil {
		t.Fatalf("Close after re-verify: %v", err)
	}
}

func TestCloseSkipsBindingWithoutFingerprint(t *testing.T) {
	root := newGitRepo(t)
	s := Store{Root: root}
	if _, err := s.Start("demo", KindT2); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"explore", "strategy", "compose", "apply", "verify"} {
		fp := ""
		if p == PhaseVerify {
			fp = FingerprintUnavailable
		}
		if _, err := s.RecordRun(p, Run{Status: StatusDone, Fingerprint: fp}); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(root, "src", "app.tsx"), "changed\n")
	if _, err := s.Close(false, ""); err != nil {
		t.Fatalf("Close with an unavailable fingerprint: %v", err)
	}
}
