package installer

import (
	"runtime/debug"
	"testing"
)

func TestResolveBuildInfo(t *testing.T) {
	vcs := func(mainVersion string, settings ...debug.BuildSetting) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Path: "github.com/yorth21/pcs", Version: mainVersion}, Settings: settings}, true
		}
	}
	revision := debug.BuildSetting{Key: "vcs.revision", Value: "0123456789abcdef"}
	vcsTime := debug.BuildSetting{Key: "vcs.time", Value: "2026-10-01T12:00:00Z"}
	noInfo := func() (*debug.BuildInfo, bool) { return nil, false }

	tests := []struct {
		name                              string
		version, commit, date             string
		read                              func() (*debug.BuildInfo, bool)
		wantVersion, wantCommit, wantDate string
	}{
		{
			name:    "ldflags win over build info",
			version: "1.2.3", commit: "abc1234", date: "2026-10-26T10:00:00Z",
			read:        vcs("v9.9.9", revision, vcsTime),
			wantVersion: "1.2.3", wantCommit: "abc1234", wantDate: "2026-10-26T10:00:00Z",
		},
		{
			name:        "go install @version uses the module version and vcs settings",
			read:        vcs("v1.4.0", revision, vcsTime),
			wantVersion: "1.4.0", wantCommit: "0123456789abcdef", wantDate: "2026-10-01T12:00:00Z",
		},
		{
			name:        "module proxy build without vcs settings",
			read:        vcs("v1.4.0"),
			wantVersion: "1.4.0", wantCommit: "unknown", wantDate: "unknown",
		},
		{
			name:        "local devel build falls back to dev",
			read:        vcs("(devel)", revision),
			wantVersion: "dev", wantCommit: "0123456789abcdef", wantDate: "unknown",
		},
		{
			name:        "no build info at all",
			read:        noInfo,
			wantVersion: "dev", wantCommit: "unknown", wantDate: "unknown",
		},
		{
			name:        "a v-prefixed ldflags version is normalized",
			version:     "v2.0.0",
			read:        noInfo,
			wantVersion: "2.0.0", wantCommit: "unknown", wantDate: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveBuildInfo(tt.version, tt.commit, tt.date, tt.read)
			want := BuildInfo{Version: tt.wantVersion, Commit: tt.wantCommit, Date: tt.wantDate}
			if got != want {
				t.Errorf("resolveBuildInfo() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestBuildInfoString(t *testing.T) {
	got := BuildInfo{Version: "1.2.3", Commit: "abc1234", Date: "2026-10-26T10:00:00Z"}.String()
	if want := "pcs version 1.2.3 (commit abc1234, built 2026-10-26T10:00:00Z)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
