package flow

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setClock pins Now to t for the duration of the test.
func setClock(t *testing.T, at time.Time) {
	t.Helper()
	prev := Now
	Now = func() time.Time { return at }
	t.Cleanup(func() { Now = prev })
}

func TestClaimAndAdopt(t *testing.T) {
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	setClock(t, base)
	s := Store{Root: t.TempDir()}
	f, err := s.Start("demo", KindAudit)
	if err != nil {
		t.Fatal(err)
	}
	if f.OwnerSessionID != "" || f.UpdatedAt != base.Format(time.RFC3339) {
		t.Fatalf("new flow = %+v", f)
	}
	tests := []struct {
		name      string
		session   string
		wantClaim bool
		wantOwner string
	}{
		{name: "empty session never claims", session: "  ", wantOwner: ""},
		{name: "first session claims", session: "A", wantClaim: true, wantOwner: "A"},
		{name: "second session cannot reclaim", session: "B", wantOwner: "A"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := s.Active()
			claimed, err := s.Claim(f, tc.session)
			if err != nil || claimed != tc.wantClaim {
				t.Fatalf("claim = %v, %v", claimed, err)
			}
			got, _ := s.Active()
			if got.OwnerSessionID != tc.wantOwner {
				t.Fatalf("owner = %q, want %q", got.OwnerSessionID, tc.wantOwner)
			}
		})
	}
	f, _ = s.Active()
	if !f.ForeignTo("B") || f.ForeignTo("A") || f.ForeignTo("") || f.ClaimedAt == "" {
		t.Fatalf("foreign checks on %+v", f)
	}
	if _, err := s.Adopt(); err != nil {
		t.Fatal(err)
	}
	f, _ = s.Active()
	if f.OwnerSessionID != "" || f.ClaimedAt != "" || f.ForeignTo("B") {
		t.Fatalf("after adopt = %+v", f)
	}
	if claimed, err := s.Claim(f, "B"); err != nil || !claimed {
		t.Fatalf("claim after adopt = %v, %v", claimed, err)
	}
	if f, _ = s.Active(); f.OwnerSessionID != "B" {
		t.Fatalf("owner after adopt = %q", f.OwnerSessionID)
	}
	if _, err := (Store{Root: t.TempDir()}).Adopt(); err != ErrNoActiveFlow {
		t.Fatalf("adopt without flow = %v", err)
	}
}

func TestStartStaleFlow(t *testing.T) {
	base := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	tests := []struct {
		name          string
		elapsed       time.Duration
		wantAbandoned bool
	}{
		{name: "recent flow refuses", elapsed: StaleAfter - time.Minute},
		{name: "exactly 12h refuses", elapsed: StaleAfter},
		{name: "idle over 12h auto-abandons", elapsed: StaleAfter + time.Minute, wantAbandoned: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setClock(t, base)
			s := Store{Root: t.TempDir()}
			old, err := s.Start("old", KindT2)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Claim(old, "A"); err != nil {
				t.Fatal(err)
			}
			setClock(t, base.Add(tc.elapsed))
			f, abandoned, err := s.StartReplacingStale("fresh", KindAudit)
			if !tc.wantAbandoned {
				if err == nil || !strings.Contains(err.Error(), AdoptCommand) || abandoned != nil {
					t.Fatalf("start = %v, %v, %v", f, abandoned, err)
				}
				return
			}
			if err != nil || f.Feature != "fresh" || abandoned == nil || abandoned.AbandonedReason != StaleReason {
				t.Fatalf("start = %+v, %+v, %v", f, abandoned, err)
			}
			prev, err := s.Load("old")
			if err != nil || !prev.Closed() || prev.AbandonedReason != StaleReason {
				t.Fatalf("old flow = %+v, %v", prev, err)
			}
			if active, _ := s.Active(); active == nil || active.Feature != "fresh" {
				t.Fatalf("active = %+v", active)
			}
		})
	}
}

func TestStartCLIReportsStaleAbandon(t *testing.T) {
	base := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	setClock(t, base)
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, ".pcs"), 0o755)
	if _, err := (Store{Root: root}).Start("old", KindT2); err != nil {
		t.Fatal(err)
	}
	setClock(t, base.Add(13*time.Hour))
	var out, errOut bytes.Buffer
	if code := RunCLI([]string{"start", "--cwd", root, "--feature", "fresh", "--kind", "audit", "--json"}, &out, &errOut, root); code != 0 {
		t.Fatalf("code %d: %s", code, errOut.String())
	}
	var v StatusView
	if err := json.Unmarshal(out.Bytes(), &v); err != nil || !strings.Contains(v.Message, "auto-abandoned flow old (t2): "+StaleReason) {
		t.Fatalf("view = %+v, %v", v, err)
	}
	out.Reset()
	if code := RunCLI([]string{"adopt", "--cwd", root}, &out, &errOut, root); code != 0 || !strings.Contains(out.String(), "adopted fresh") {
		t.Fatalf("adopt code %d: %q %q", code, out.String(), errOut.String())
	}
}

func TestLegacyStateLoads(t *testing.T) {
	base := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		createdAt string
		now       time.Time
		wantStale bool
	}{
		{name: "recent legacy flow is active and unowned", createdAt: base.Format(time.RFC3339), now: base.Add(time.Hour)},
		{name: "old legacy flow is stale by created_at", createdAt: base.Format(time.RFC3339), now: base.Add(13 * time.Hour), wantStale: true},
		{name: "unparseable timestamps are never stale", createdAt: "", now: base.Add(100 * time.Hour)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setClock(t, tc.now)
			root := t.TempDir()
			legacy := `{"schema":"pcs.flow/v1","feature":"demo","kind":"audit","created_at":"` + tc.createdAt + `",` +
				`"phases":[{"name":"explore","agents":["pcs-explorer"],"status":"pending"}],"inline_reads":0,"inline_files":[]}`
			os.MkdirAll(filepath.Join(root, ".pcs", "flows"), 0o755)
			os.WriteFile(filepath.Join(root, ".pcs", "active"), []byte("demo\n"), 0o644)
			os.WriteFile(filepath.Join(root, ".pcs", "flows", "demo.json"), []byte(legacy), 0o644)
			s := Store{Root: root}
			f, err := s.Active()
			if err != nil || f == nil || f.OwnerSessionID != "" || f.UpdatedAt != "" || f.ForeignTo("A") {
				t.Fatalf("legacy = %+v, %v", f, err)
			}
			if f.Stale() != tc.wantStale {
				t.Fatalf("stale = %v, want %v", f.Stale(), tc.wantStale)
			}
			if claimed, err := s.Claim(f, "A"); err != nil || !claimed {
				t.Fatalf("claim = %v, %v", claimed, err)
			}
			if f, _ = s.Active(); f.OwnerSessionID != "A" || f.UpdatedAt != tc.now.Format(time.RFC3339) {
				t.Fatalf("claimed legacy = %+v", f)
			}
		})
	}
}
