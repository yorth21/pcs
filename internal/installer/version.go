package installer

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// Build metadata injected at link time by release builds (GoReleaser and
// `make build`):
//
//	-ldflags "-X github.com/yorth21/pcs/internal/installer.version=1.2.3
//	          -X github.com/yorth21/pcs/internal/installer.commit=<sha>
//	          -X github.com/yorth21/pcs/internal/installer.date=<RFC 3339>"
//
// When they are empty, ResolveBuildInfo falls back to debug.ReadBuildInfo.
var (
	version string
	commit  string
	date    string
)

// unknownBuildValue marks build metadata that neither ldflags nor the Go build
// info provide.
const unknownBuildValue = "unknown"

// BuildInfo describes the running pcs binary.
type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// String renders the one-line `pcs version` output.
func (b BuildInfo) String() string {
	return fmt.Sprintf("pcs version %s (commit %s, built %s)", b.Version, b.Commit, b.Date)
}

// Version is the pcs version recorded in .pcs-state.json and printed by
// `pcs version`.
var Version = ResolveBuildInfo().Version

// ResolveBuildInfo returns the build metadata of the running binary.
func ResolveBuildInfo() BuildInfo {
	return resolveBuildInfo(version, commit, date, debug.ReadBuildInfo)
}

// resolveBuildInfo prefers the ldflags values and fills the missing ones from
// read: the main module version (set by `go install ...@vX.Y.Z`; "(devel)" for
// local builds) and the vcs.revision and vcs.time settings stamped by `go
// build` in a VCS checkout. A leading "v" is dropped so every source prints the
// same form; anything still missing is "dev" (version) or "unknown".
func resolveBuildInfo(ldVersion, ldCommit, ldDate string, read func() (*debug.BuildInfo, bool)) BuildInfo {
	info := BuildInfo{Version: ldVersion, Commit: ldCommit, Date: ldDate}
	if bi, ok := read(); ok && bi != nil {
		if info.Version == "" && bi.Main.Version != "(devel)" {
			info.Version = bi.Main.Version
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = s.Value
				}
			case "vcs.time":
				if info.Date == "" {
					info.Date = s.Value
				}
			}
		}
	}
	info.Version = strings.TrimPrefix(info.Version, "v")
	if info.Version == "" {
		info.Version = "dev"
	}
	if info.Commit == "" {
		info.Commit = unknownBuildValue
	}
	if info.Date == "" {
		info.Date = unknownBuildValue
	}
	return info
}
