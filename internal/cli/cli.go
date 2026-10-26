package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/yorth21/pcs/internal/doctor"
	"github.com/yorth21/pcs/internal/installer"
	"github.com/yorth21/pcs/internal/registry"
)

// Env provides injectable I/O and environment boundaries for CLI execution.
type Env struct {
	Stdout          io.Writer
	Stderr          io.Writer
	HomeDir         string
	WorkDir         string
	ClaudeConfigDir string
}

// Run executes the pcs CLI command router and returns an exit code (0 on success).
func Run(args []string, env Env) int {
	if env.Stdout == nil {
		env.Stdout = os.Stdout
	}
	if env.Stderr == nil {
		env.Stderr = os.Stderr
	}
	if len(args) == 0 {
		printUsage(env.Stdout)
		return 0
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "help", "-h", "--help":
		printUsage(env.Stdout)
		return 0
	case "version", "-v", "--version":
		return runVersion(rest, env)
	case "install":
		return runInstall(rest, env)
	case "sync":
		return runSync(rest, env)
	case "uninstall":
		return runUninstall(rest, env)
	case "status":
		return runStatus(rest, env)
	case "doctor":
		return runDoctor(rest, env)
	case "skill-registry":
		return runSkillRegistry(rest, env)
	default:
		fmt.Fprintf(env.Stderr, "pcs: unknown command %q\n\n", sub)
		printUsage(env.Stderr)
		return 1
	}
}

func runVersion(args []string, env Env) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	asJSON := fs.Bool("json", false, "Output version information as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *asJSON {
		return writeJSONOutput(env.Stdout, map[string]string{
			"name":    "pcs",
			"version": installer.Version,
		})
	}
	fmt.Fprintf(env.Stdout, "pcs version %s\n", installer.Version)
	return 0
}

func defaultScopeFromEnv() string {
	if s := strings.TrimSpace(os.Getenv("PCS_INSTALL_SCOPE")); s != "" {
		return s
	}
	return installer.ScopeGlobal
}

func runInstall(args []string, env Env) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	scope := fs.String("scope", defaultScopeFromEnv(), "Installation scope: global, workspace, or profile (env: PCS_INSTALL_SCOPE)")
	profileAlias := fs.Bool("profile", false, "Shorthand for --scope profile")
	targetDir := fs.String("target-dir", "", "Custom target directory")
	adapter := fs.String("adapter", installer.DefaultAdapter, "Frontend stack adapter (e.g. react-shadcn)")
	persona := fs.String("persona", installer.DefaultPersona, "Coordinator persona variant: gentleman or neutral")
	withMCP := fs.Bool("with-mcp", true, "Configure engram, context7, and codegraph MCP servers and permissions")
	dryRun := fs.Bool("dry-run", false, "Preview planned actions without writing files")
	asJSON := fs.Bool("json", false, "Output installation report as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	chosenScope := *scope
	if *profileAlias {
		chosenScope = installer.ScopeProfile
	}

	rep, err := installer.Install(installer.Options{
		Scope:           chosenScope,
		TargetDir:       *targetDir,
		HomeDir:         env.HomeDir,
		ClaudeConfigDir: env.ClaudeConfigDir,
		Adapter:         *adapter,
		Persona:         *persona,
		WithMCP:         *withMCP,
		DryRun:          *dryRun,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs install failed: %v\n", err)
		return 1
	}
	if *asJSON {
		return writeJSONOutput(env.Stdout, rep)
	}
	printReport(env.Stdout, rep)
	return 0
}

func runSync(args []string, env Env) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	scope := fs.String("scope", defaultScopeFromEnv(), "Target scope: global, workspace, or profile (env: PCS_INSTALL_SCOPE)")
	targetDir := fs.String("target-dir", "", "Custom target directory")
	dryRun := fs.Bool("dry-run", false, "Preview planned sync actions without writing files")
	asJSON := fs.Bool("json", false, "Output sync report as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	rep, err := installer.Sync(installer.Options{
		Scope:           *scope,
		TargetDir:       *targetDir,
		HomeDir:         env.HomeDir,
		ClaudeConfigDir: env.ClaudeConfigDir,
		DryRun:          *dryRun,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs sync failed: %v\n", err)
		return 1
	}
	if *asJSON {
		return writeJSONOutput(env.Stdout, rep)
	}
	printReport(env.Stdout, rep)
	return 0
}

func runUninstall(args []string, env Env) int {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	scope := fs.String("scope", defaultScopeFromEnv(), "Target scope: global, workspace, or profile (env: PCS_INSTALL_SCOPE)")
	targetDir := fs.String("target-dir", "", "Custom target directory")
	removeMCP := fs.Bool("remove-mcp", false, "Also remove engram, context7, and codegraph MCP entries in global/workspace scopes")
	dryRun := fs.Bool("dry-run", false, "Preview planned uninstall actions without removing files")
	asJSON := fs.Bool("json", false, "Output uninstall report as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	rep, err := installer.Uninstall(installer.Options{
		Scope:           *scope,
		TargetDir:       *targetDir,
		HomeDir:         env.HomeDir,
		ClaudeConfigDir: env.ClaudeConfigDir,
		RemoveMCP:       *removeMCP,
		DryRun:          *dryRun,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs uninstall failed: %v\n", err)
		return 1
	}
	if *asJSON {
		return writeJSONOutput(env.Stdout, rep)
	}
	printReport(env.Stdout, rep)
	return 0
}

func runStatus(args []string, env Env) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	scope := fs.String("scope", defaultScopeFromEnv(), "Target scope: global, workspace, or profile (env: PCS_INSTALL_SCOPE)")
	targetDir := fs.String("target-dir", "", "Custom target directory")
	asJSON := fs.Bool("json", false, "Output status report as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	rep, err := installer.Status(installer.Options{
		Scope:           *scope,
		TargetDir:       *targetDir,
		HomeDir:         env.HomeDir,
		ClaudeConfigDir: env.ClaudeConfigDir,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs status failed: %v\n", err)
		return 1
	}
	if *asJSON {
		return writeJSONOutput(env.Stdout, rep)
	}

	fmt.Fprintf(env.Stdout, "Product Composition System (pcs) Status\n")
	fmt.Fprintf(env.Stdout, "  Scope:      %s\n", rep.Scope)
	fmt.Fprintf(env.Stdout, "  Target Dir: %s\n", rep.TargetDir)
	fmt.Fprintf(env.Stdout, "  Installed:  %v\n", rep.Installed)
	if rep.State != nil {
		fmt.Fprintf(env.Stdout, "  Adapter:    %s\n", rep.State.Adapter)
		fmt.Fprintf(env.Stdout, "  Persona:    %s\n", rep.State.Persona)
	}
	printBoolMap(env.Stdout, "Markers", rep.Markers)
	printBoolMap(env.Stdout, "Native Agents", rep.Agents)
	printBoolMap(env.Stdout, "Skills", rep.Skills)
	printBoolMap(env.Stdout, "MCP Servers", rep.MCPServers)
	return 0
}

func runDoctor(args []string, env Env) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	targetDir := fs.String("target-dir", "", "Optional target directory to inspect")
	asJSON := fs.Bool("json", false, "Output diagnostic report as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	rep, err := doctor.Run(doctor.Options{
		TargetDir: *targetDir,
		HomeDir:   env.HomeDir,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs doctor failed: %v\n", err)
		return 1
	}
	if *asJSON {
		code := writeJSONOutput(env.Stdout, rep)
		if !rep.Healthy {
			return 1
		}
		return code
	}

	fmt.Fprintf(env.Stdout, "Product Composition System (pcs) Doctor\n")
	for _, c := range rep.Checks {
		fmt.Fprintf(env.Stdout, "  [%s] (%s) %s: %s\n", strings.ToUpper(c.Status), c.Category, c.Name, c.Detail)
	}
	if !rep.Healthy {
		fmt.Fprintf(env.Stdout, "Verdict: UNHEALTHY\n")
		return 1
	}
	fmt.Fprintf(env.Stdout, "Verdict: HEALTHY\n")
	return 0
}

func runSkillRegistry(args []string, env Env) int {
	if len(args) == 0 || args[0] != "refresh" {
		fmt.Fprintf(env.Stderr, "usage: pcs skill-registry refresh [--cwd <dir>] [--skills-dir <dir>] [--json]\n")
		return 2
	}

	fs := flag.NewFlagSet("skill-registry refresh", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	cwd := fs.String("cwd", env.WorkDir, "Workspace root directory")
	skillsDir := fs.String("skills-dir", "", "Additional skills directory to include")
	asJSON := fs.Bool("json", false, "Output refresh result as JSON")

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	root := strings.TrimSpace(*cwd)
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(env.Stderr, "pcs skill-registry refresh: %v\n", err)
			return 1
		}
	}

	var extras []string
	if strings.TrimSpace(*skillsDir) != "" {
		extras = append(extras, *skillsDir)
	}

	res, err := registry.Refresh(root, extras)
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs skill-registry refresh failed: %v\n", err)
		return 1
	}
	if *asJSON {
		return writeJSONOutput(env.Stdout, res)
	}
	fmt.Fprintf(env.Stdout, "Refreshed %s (%d skills, cache_hit=%v)\n",
		res.OutputPath, len(res.Skills), res.CacheHit)
	return 0
}

func writeJSONOutput(w io.Writer, v any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return 1
	}
	return 0
}

func printReport(w io.Writer, rep *installer.Report) {
	mode := ""
	if rep.DryRun {
		mode = " (dry-run)"
	}
	fmt.Fprintf(w, "pcs %s%s completed for scope=%s target=%s\n",
		rep.Operation, mode, rep.Scope, rep.TargetDir)
	for _, a := range rep.Actions {
		if a.Detail != "" {
			fmt.Fprintf(w, "  - [%s] %s (%s)\n", a.Operation, a.Path, a.Detail)
		} else {
			fmt.Fprintf(w, "  - [%s] %s\n", a.Operation, a.Path)
		}
	}
	if rep.LauncherCmd != "" {
		fmt.Fprintf(w, "\nLaunch isolated Product Composition session with:\n  %s\n", rep.LauncherCmd)
	}
}

func printBoolMap(w io.Writer, title string, m map[string]bool) {
	fmt.Fprintf(w, "  %s:\n", title)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "    - %s: %v\n", k, m[k])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `Product Composition System (pcs) — LLM-first Product & UX/UI Brain CLI

Usage:
  pcs <command> [flags]

Commands:
  install                  Install pcs orchestrator, 5 native subagents, skills, and MCP config
  sync                     Re-sync embedded brain files and markers into an existing installation
  uninstall                Cleanly remove pcs-owned markers, native subagents, and skills
  status                   Inspect installed markers, native subagents, skills, and MCP servers
  doctor                   Run diagnostic checks on MCP binaries, embedded brain, and target dir
  skill-registry refresh   Scan SKILL.md files and generate .atl/skill-registry.md
  version                  Print pcs CLI version
  help                     Show this help message

Key Flags (install / sync / uninstall / status):
  --scope global|workspace|profile   Target deployment mode (default: global -> ~/.claude or $CLAUDE_CONFIG_DIR; env: PCS_INSTALL_SCOPE)
  --target-dir <dir>                 Override target configuration directory
  --adapter <name>                   Frontend stack adapter (default: react-shadcn)
  --persona gentleman|neutral        Coordinator persona variant (default: gentleman)
  --with-mcp=true|false              Wire engram, context7, and codegraph MCP servers (default: true)
  --dry-run                          Preview planned actions without modifying disk
  --json                             Emit structured JSON output
`)
}
