package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yorth21/pcs/internal/assets"
	"github.com/yorth21/pcs/internal/installer"
	"github.com/yorth21/pcs/internal/mcp"
)

// CheckResult describes a single diagnostic check.
type CheckResult struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Status   string `json:"status"` // "ok", "warn", "error", "unknown"
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
	// GOOS selects platform-specific checks; empty means runtime.GOOS.
	GOOS string
}

// Run executes all pcs ecosystem health checks.
func Run(opts Options) (*DiagnosticReport, error) {
	lookPath := opts.LookPathFunc
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	goos := opts.GOOS
	if goos == "" {
		goos = runtime.GOOS
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
		// A profile session reads MCP servers from shared-mcp.json (passed by the
		// launcher) as well as from the profile .claude.json.
		var sharedServers map[string]bool
		if scope == installer.ScopeProfile {
			sharedServers, _ = mcp.InspectConfiguredServers(filepath.Join(targetDir, "shared-mcp.json"))
		}
		for _, srv := range mcp.DefaultComponents {
			st, msg := "ok", "configured in MCP JSON"
			switch {
			case sharedServers != nil && sharedServers[srv]:
				msg = "configured in shared-mcp.json"
			case status.MCPServers[srv] && sharedServers != nil:
				msg = "configured in the profile .claude.json"
			case status.MCPServers[srv]:
			case sharedServers != nil:
				st, msg = "warn", "not configured in shared-mcp.json or the profile .claude.json. Run 'pcs sync --scope profile' to refresh shared-mcp.json from the main profile"
			default:
				st, msg = "warn", "not configured in MCP JSON"
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
				// Profile check 2b: shared-mcp.json mirrors the main profile's mcpServers.
				drift, err := mcp.SharedMCPDrift(sharedMCP, status.MainClaudeJSONPath)
				switch {
				case err != nil:
					addCheck("profile", "shared-mcp-sync", "warn", fmt.Sprintf("cannot compare with the main profile: %v", err))
				case len(drift) > 0:
					addCheck("profile", "shared-mcp-sync", "warn", fmt.Sprintf("shared-mcp.json differs from the main profile for: %s. Run 'pcs sync --scope profile' to refresh it", strings.Join(drift, ", ")))
				default:
					addCheck("profile", "shared-mcp-sync", "ok", "shared-mcp.json matches the main profile mcpServers")
				}
			} else {
				addCheck("profile", "shared-mcp.json", "warn", "shared-mcp.json missing. Run 'pcs sync --scope profile' to generate it")
			}

			// Profile check 2c: every pcs agent requires Engram, provided either by an
			// engram server in shared-mcp.json or by the engram@engram plugin.
			addEngramSourceCheck(addCheck, sharedMCP, status.SettingsJSONPath, targetDir)

			// Profile check 3: plugins requested with --with-plugins are enabled in the profile.
			if status.State != nil && status.State.WithPlugins {
				enabled, err := mcp.PluginEnabled(status.SettingsJSONPath, installer.EngramPlugin)
				name := "plugin:" + installer.EngramPlugin
				switch {
				case err != nil:
					addCheck("profile", name, "warn", fmt.Sprintf("cannot read %s: %v", status.SettingsJSONPath, err))
				case enabled:
					addCheck("profile", name, "ok", "enabled in profile settings.json")
				default:
					addCheck("profile", name, "warn", fmt.Sprintf("not enabled in %s. Run 'pcs sync --scope profile --with-plugins', or run: %s",
						status.SettingsJSONPath, installer.PluginInstallCommand(goos, targetDir)))
				}
			}

			// Profile check 3b: pcs flow hooks that enforce the PCD flow.
			addFlowHooksCheck(addCheck, status.SettingsJSONPath, status.State)

			// Profile check 4: authentication check (file existence only, never read credentials)
			credsPath := filepath.Join(targetDir, ".credentials.json")
			switch _, err := os.Stat(credsPath); {
			case err == nil:
				addCheck("profile", "authentication", "ok", "profile authenticated (.credentials.json present)")
			case goos == "darwin":
				addCheck("profile", "authentication", "unknown", "cannot verify: on macOS Claude Code stores credentials in the Keychain, not .credentials.json. If 'claude-pcs' asks you to log in, execute '/login' once")
			default:
				addCheck("profile", "authentication", "warn", "profile not authenticated yet. Run 'claude-pcs' and execute '/login' once")
			}
		}
	}

	return report, nil
}

// addEngramSourceCheck reports an error when the profile has no Engram source:
// neither an engram server in shared-mcp.json nor enabledPlugins["engram@engram"]
// in the profile settings.json.
func addEngramSourceCheck(addCheck func(cat, name, status, detail string), sharedMCP, settingsPath, targetDir string) {
	servers, err := mcp.InspectConfiguredServers(sharedMCP)
	if err == nil && servers[mcp.ServerEngram] {
		addCheck("profile", "engram", "ok", fmt.Sprintf("Engram provided by the engram MCP server in %s", sharedMCP))
		return
	}
	if enabled, pluginErr := mcp.PluginEnabled(settingsPath, installer.EngramPlugin); pluginErr == nil && enabled {
		addCheck("profile", "engram", "ok", fmt.Sprintf("Engram provided by the %s plugin enabled in %s", installer.EngramPlugin, settingsPath))
		return
	}
	addCheck("profile", "engram", "error", fmt.Sprintf("no Engram source for the profile %s: %s has no engram MCP server and %s is not enabled in %s. Install engram (%s) and run 'pcs sync --scope profile', or run 'pcs sync --scope profile --with-plugins'",
		targetDir, sharedMCP, installer.EngramPlugin, settingsPath, installer.EngramProjectURL))
}

// addFlowHooksCheck reports whether the pcs flow hooks (pcs hook <event>) are
// present in the profile settings.json and run an existing pcs binary. Hooks
// disabled with --no-hooks report ok.
func addFlowHooksCheck(addCheck func(cat, name, status, detail string), settingsPath string, state *installer.State) {
	if state != nil && state.FlowHooks != nil && !*state.FlowHooks {
		addCheck("profile", "flow-hooks", "ok", "disabled (installed with --no-hooks); the PCD flow is not enforced")
		return
	}
	binary := ""
	if state != nil {
		binary = state.FlowHookBinary
	}
	present, _, err := mcp.InspectFlowHooks(settingsPath, binary)
	if err != nil {
		addCheck("profile", "flow-hooks", "warn", fmt.Sprintf("cannot read %s: %v", settingsPath, err))
		return
	}
	var missing []string
	for _, spec := range mcp.FlowHookSpecs {
		if !present[spec.Event] {
			missing = append(missing, spec.Event)
		}
	}
	if len(missing) > 0 {
		addCheck("profile", "flow-hooks", "warn", fmt.Sprintf("missing %s in %s. Run 'pcs sync --scope profile --with-hooks'", strings.Join(missing, ", "), settingsPath))
		return
	}
	if binary != "" {
		if _, err := os.Stat(binary); err != nil {
			addCheck("profile", "flow-hooks", "warn", fmt.Sprintf("hooks run %s, which does not exist; run 'pcs sync --scope profile' from the installed pcs binary", filepath.ToSlash(binary)))
			return
		}
	}
	addCheck("profile", "flow-hooks", "ok", fmt.Sprintf("PreToolUse, SubagentStop, Stop, UserPromptSubmit present in %s", settingsPath))
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
