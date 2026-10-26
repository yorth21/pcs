package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yorth21/pcs/internal/assets"
	"github.com/yorth21/pcs/internal/installer"
)

// CheckResult describes a single diagnostic check.
type CheckResult struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Status   string `json:"status"` // "ok", "warn", "error"
	Detail   string `json:"detail"`
}

// DiagnosticReport summarizes all binary, embedded brain, and target installation checks.
type DiagnosticReport struct {
	Healthy   bool          `json:"healthy"`
	TargetDir string        `json:"target_dir,omitempty"`
	Checks    []CheckResult `json:"checks"`
}

// Options configures a doctor diagnostic run.
type Options struct {
	Scope        string
	TargetDir    string
	ProfileDir   string
	HomeDir      string
	LookPathFunc func(file string) (string, error)
}

// Run executes all pcs ecosystem health checks.
func Run(opts Options) (*DiagnosticReport, error) {
	lookPath := opts.LookPathFunc
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	report := &DiagnosticReport{
		Healthy:   true,
		TargetDir: opts.TargetDir,
	}

	addCheck := func(cat, name, status, detail string) {
		if status == "error" {
			report.Healthy = false
		}
		report.Checks = append(report.Checks, CheckResult{
			Category: cat,
			Name:     name,
			Status:   status,
			Detail:   detail,
		})
	}

	// 1. Binary availability checks.
	binaries := []struct {
		bin  string
		desc string
	}{
		{bin: "engram", desc: "Persistent Memory MCP server"},
		{bin: "codegraph", desc: "Structural Code Intelligence MCP server"},
		{bin: "npx", desc: "Context7 Live Docs MCP runner"},
		{bin: "git", desc: "Version control CLI"},
	}

	for _, b := range binaries {
		path, err := lookPath(b.bin)
		if err != nil && b.bin == "engram" {
			if _, statErr := os.Stat("/opt/homebrew/bin/engram"); statErr == nil {
				path = "/opt/homebrew/bin/engram"
				err = nil
			}
		}
		if err != nil {
			addCheck("binary", b.bin, "warn", fmt.Sprintf("not found on PATH (%s)", b.desc))
		} else {
			addCheck("binary", b.bin, "ok", fmt.Sprintf("%s (%s)", path, b.desc))
		}
	}

	// 2. Embedded BrainFS integrity checks.
	if _, err := assets.OrchestratorBlock(); err != nil {
		addCheck("brain-embed", "pcs-orchestrator.md", "error", err.Error())
	} else {
		addCheck("brain-embed", "pcs-orchestrator.md", "ok", "embedded <!-- pcs:orchestrator --> block verified")
	}

	if _, err := assets.UXRoutingBlock(); err != nil {
		addCheck("brain-embed", "pcs-routing.md", "error", err.Error())
	} else {
		addCheck("brain-embed", "pcs-routing.md", "ok", "embedded <!-- pcs:ux-routing --> block verified")
	}

	if agents, err := assets.NativeAgents(); err != nil || len(agents) != 5 {
		addCheck("brain-embed", "native-agents", "error", fmt.Sprintf("expected 5 native agents, err=%v", err))
	} else {
		addCheck("brain-embed", "native-agents", "ok", "5 native subagents (pcs-explorer..pcs-verifier) verified")
	}

	if _, err := assets.SharedPhaseContract(); err != nil {
		addCheck("brain-embed", "pcs-phase-common.md", "error", err.Error())
	} else {
		addCheck("brain-embed", "pcs-phase-common.md", "ok", "shared phase contract verified")
	}

	if coreFiles, err := assets.CoreSkillFiles(); err != nil || len(coreFiles) == 0 {
		addCheck("brain-embed", "pcs-core", "error", fmt.Sprintf("pcs-core skill missing: %v", err))
	} else {
		addCheck("brain-embed", "pcs-core", "ok", fmt.Sprintf("%d embedded files verified", len(coreFiles)))
	}

	if adapterFiles, err := assets.AdapterSkillFiles("react-shadcn"); err != nil || len(adapterFiles) == 0 {
		addCheck("brain-embed", "pcs-adapter-react-shadcn", "error", fmt.Sprintf("adapter skill missing: %v", err))
	} else {
		addCheck("brain-embed", "pcs-adapter-react-shadcn", "ok", fmt.Sprintf("%d embedded files verified", len(adapterFiles)))
	}

	// 3. Target installation checks if target directory exists or was explicitly requested.
	targetDir := strings.TrimSpace(opts.TargetDir)
	if targetDir == "" && opts.ProfileDir != "" {
		targetDir = opts.ProfileDir
	}
	explicitTarget := targetDir != ""
	if targetDir == "" {
		home := opts.HomeDir
		if home == "" {
			home, _ = os.UserHomeDir()
		}
		if home != "" {
			candidate := filepath.Join(home, ".claude-pcs")
			if _, err := os.Stat(candidate); err == nil {
				targetDir = candidate
			}
		}
	}

	if targetDir != "" {
		report.TargetDir = targetDir
		scope := opts.Scope
		if scope == "" {
			scope = inferScope(targetDir)
		}
		status, err := installer.Status(installer.Options{
			Scope:      scope,
			TargetDir:  targetDir,
			ProfileDir: opts.ProfileDir,
			HomeDir:    opts.HomeDir,
		})
		if err != nil {
			addCheck("target", "status", "error", err.Error())
			return report, nil
		}

		checkTargetBool := func(name string, ok bool, okMsg, missingMsg string) {
			st := "ok"
			msg := okMsg
			if !ok {
				if explicitTarget {
					st = "error"
				} else {
					st = "warn"
				}
				msg = missingMsg
			}
			addCheck("target", name, st, msg)
		}

		checkTargetBool("marker:pcs:orchestrator", status.Markers[assets.MarkerOrchestrator],
			"present in CLAUDE.md", "missing in CLAUDE.md")
		checkTargetBool("marker:pcs:ux-routing", status.Markers[assets.MarkerUXRouting],
			"present in CLAUDE.md", "missing in CLAUDE.md")

		for _, agentName := range assets.NativeAgentNames {
			checkTargetBool("agent:"+agentName, status.Agents[agentName],
				"installed", "missing from agents/")
		}
		for skillName, present := range status.Skills {
			checkTargetBool("skill:"+skillName, present,
				"installed", "missing from skills/")
		}
		for srv, present := range status.MCPServers {
			st := "ok"
			msg := "configured in MCP JSON"
			if !present {
				st = "warn"
				msg = "not configured in MCP JSON"
			}
			addCheck("target-mcp", "mcp:"+srv, st, msg)
		}

		if scope == installer.ScopeProfile {
			// Profile check 1: launcher on PATH
			if lp, err := lookPath("claude-pcs"); err == nil {
				addCheck("profile", "launcher", "ok", fmt.Sprintf("claude-pcs found on PATH (%s)", lp))
			} else {
				addCheck("profile", "launcher", "warn", "claude-pcs not found on PATH. Run 'pcs install --scope profile' or add launcher directory to PATH")
			}

			// Profile check 2: shared-mcp.json presence
			sharedMCP := filepath.Join(targetDir, "shared-mcp.json")
			if _, err := os.Stat(sharedMCP); err == nil {
				addCheck("profile", "shared-mcp.json", "ok", "shared MCP configuration file present")
			} else {
				addCheck("profile", "shared-mcp.json", "warn", "shared-mcp.json missing. Run 'pcs sync --scope profile' to generate it")
			}

			// Profile check 3: authentication check (file existence only, never read credentials)
			credsPath := filepath.Join(targetDir, ".credentials.json")
			if _, err := os.Stat(credsPath); err == nil {
				addCheck("profile", "authentication", "ok", "profile authenticated (.credentials.json present)")
			} else {
				addCheck("profile", "authentication", "warn", "profile not authenticated yet. Run 'claude-pcs' and execute '/login' once")
			}
		}
	}

	return report, nil
}

func inferScope(targetDir string) string {
	clean := filepath.Clean(targetDir)
	base := filepath.Base(clean)
	if strings.Contains(base, ".claude-pcs") {
		return installer.ScopeProfile
	}
	if base == ".claude" {
		return installer.ScopeGlobal
	}
	return installer.ScopeProfile
}
