package flow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"
)

// FingerprintUnavailable is the fingerprint recorded when git cannot describe
// the working tree (no git binary, not a repository, or a git error). It never
// marks a phase stale.
const FingerprintUnavailable = "unavailable"

// StaleVerifyMessage explains why `pcs flow close` refused and what to do.
const StaleVerifyMessage = "Source changed after verify; relaunch pcs-verifier"

// fingerprintPhases lists the phases whose done result is bound to the
// working tree it checked.
var fingerprintPhases = []string{PhaseVerify, "audit"}

// FingerprintsPhase reports whether a done result of phase records a
// working-tree fingerprint.
func FingerprintsPhase(phase string) bool { return slices.Contains(fingerprintPhases, phase) }

// gitTimeout bounds every git invocation of Fingerprint.
const gitTimeout = 10 * time.Second

// gitExcludes keeps CLI-owned flow state and PCD artifacts (specs, tasks,
// audits, the verifier's own report) out of the fingerprint.
var gitExcludes = []string{"--", ".", ":(exclude).pcs", ":(exclude)pcs"}

// Fingerprint returns a SHA-256 digest of the working tree under root: the
// `git diff HEAD` content plus every untracked, non-ignored file name and
// content, excluding .pcs/ and pcs/. It is best effort and returns
// FingerprintUnavailable when git is not usable. A commit made after verify
// changes the digest too, so it conservatively requires a new verify.
func Fingerprint(root string) string {
	if _, err := exec.LookPath("git"); err != nil {
		return FingerprintUnavailable
	}
	h := sha256.New()
	diffArgs := []string{"diff", "--no-color", "--no-ext-diff", "--binary", "HEAD"}
	if _, ok := git(root, "rev-parse", "--verify", "--quiet", "HEAD"); !ok {
		// A repository without commits: compare against the empty tree.
		diffArgs = []string{"diff", "--no-color", "--no-ext-diff", "--binary", "4b825dc642cb6eb9a060e54bf8d69288fbee4904"}
	}
	diff, ok := git(root, append(diffArgs, gitExcludes...)...)
	if !ok {
		return FingerprintUnavailable
	}
	h.Write([]byte("diff\x00"))
	h.Write(diff)
	untracked, ok := git(root, append([]string{"ls-files", "-z", "--others", "--exclude-standard"}, gitExcludes...)...)
	if !ok {
		return FingerprintUnavailable
	}
	names := bytes.Split(bytes.TrimRight(untracked, "\x00"), []byte{0})
	slices.SortFunc(names, bytes.Compare)
	for _, name := range names {
		if len(name) == 0 {
			continue
		}
		h.Write([]byte("\x00untracked\x00"))
		h.Write(name)
		h.Write([]byte{0})
		if f, err := os.Open(filepath.Join(root, filepath.FromSlash(string(name)))); err == nil {
			_, _ = io.Copy(h, f)
			f.Close()
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func git(dir string, args ...string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "-c", "core.quotepath=off"}, args...)...)
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	return out, err == nil
}

// VerifyBindingStale recomputes the working-tree fingerprint of a t2/t3 flow
// whose verify phase is done with a recorded fingerprint. When it differs, it
// marks verify stale (so `pcs flow next` delegates the verifier again), saves
// f, and reports true. An unavailable fingerprint, on either side, never
// marks the phase stale.
func (s Store) VerifyBindingStale(f *Flow) (bool, error) {
	if f == nil || (f.Kind != KindT2 && f.Kind != KindT3) {
		return false, nil
	}
	idx := f.PhaseIndex(PhaseVerify)
	if idx < 0 {
		return false, nil
	}
	p := &f.Phases[idx]
	if p.Status != StatusDone || p.Fingerprint == "" || p.Fingerprint == FingerprintUnavailable {
		return false, nil
	}
	current := Fingerprint(s.Root)
	if current == FingerprintUnavailable || current == p.Fingerprint {
		return false, nil
	}
	p.Status = StatusStale
	p.Note = StaleVerifyMessage + " (verified " + p.Fingerprint + ", now " + current + ")."
	return true, s.Save(f)
}
