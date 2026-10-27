package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/yorth21/pcs/internal/doctor"
	"github.com/yorth21/pcs/internal/flow"
	"github.com/yorth21/pcs/internal/hook"
	"github.com/yorth21/pcs/internal/installer"
	"github.com/yorth21/pcs/internal/registry"
)

// Env provides injectable I/O and environment boundaries for CLI execution.
type Env struct {
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
	HomeDir         string
	WorkDir         string
	ClaudeConfigDir string
	ExecCommand     func(name string, args ...string) ([]byte, error)
}

// Run executes the pcs CLI command router and returns an exit code (0 on success).
func Run(args []string, env Env) int {
	if env.Stdout == nil {
		env.Stdout = os.Stdout
	}
	if env.Stderr == nil {
		env.Stderr = os.Stderr
	}
	if env.Stdin == nil {
		env.Stdin = os.Stdin
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
	case "codegraph":
		return runCodeGraph(rest, env)
	case "flow":
		if len(rest) > 0 && rest[0] == "statusline" {
			return flow.RunStatusLine(rest[1:], env.Stdin, env.Stdout, env.WorkDir)
		}
		return flow.RunCLI(rest, env.Stdout, env.Stderr, env.WorkDir)
	case "statusline":
		return flow.RunStatusLine(rest, env.Stdin, env.Stdout, env.WorkDir)
	case "statusline-wrap":
		return flow.RunStatusLineWrap(rest, env.Stdin, env.Stdout, env.WorkDir)
	case "hook":
		return runHook(rest, env)
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
	info := installer.ResolveBuildInfo()
	if *asJSON {
		return writeJSONOutput(env.Stdout, map[string]string{
			"name":    "pcs",
			"version": info.Version,
			"commit":  info.Commit,
			"date":    info.Date,
		})
	}
	fmt.Fprintln(env.Stdout, info.String())
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
	profileDir := fs.String("profile-dir", "", "Custom profile directory when using --scope profile")
	targetDir := fs.String("target-dir", "", "Custom target directory")
	adapter := fs.String("adapter", installer.DefaultAdapter, "Frontend stack adapter (e.g. react-shadcn)")
	persona := fs.String("persona", installer.DefaultPersona, "Coordinator persona variant: gentleman or neutral")
	mcpGuidance := fs.Bool("mcp-guidance", true, "Inject pcs:codegraph-guidance and pcs:engram-protocol prompt blocks into CLAUDE.md")
	writeMCPConfig := fs.Bool("write-mcp-config", false, "Write missing engram, context7, and codegraph MCP entries into .claude.json and settings.json")
	writeHook := fs.Bool("write-hook", false, "Write the UserPromptSubmit skill-registry refresh hook into settings.json")
	withMCP := fs.Bool("with-mcp", false, "Convenience alias for --mcp-guidance=true --write-mcp-config=true --write-hook=true")
	launcherDir := fs.String("launcher-dir", "", "Custom launcher installation directory (default: ~/.local/bin or %LOCALAPPDATA%\\pcs\\bin)")
	launcherArgs := fs.String("launcher-args", "", "Default arguments baked into the claude-pcs launchers: split on whitespace, single or double quotes group words, no escape character; every value is passed literally")
	withPlugins := fs.Bool("with-plugins", false, "Add the engram marketplace and install the engram@engram plugin into the profile via the claude CLI (remembered by sync)")
	noStatusLine := fs.Bool("no-statusline", false, "Do not copy statusLine from main profile into profile settings.json (remembered by sync)")
	withHooks := fs.Bool("with-hooks", false, "Write the pcs flow hooks (PreToolUse, SubagentStop, Stop, UserPromptSubmit) into settings.json; default on for --scope profile only (remembered by sync)")
	withFlowStatusLine := fs.Bool("with-flow-statusline", true, "Profile scope: wrap the profile statusLine with pcs statusline-wrap to append the active flow segment; default on unless --no-statusline (remembered by sync)")
	noHooks := fs.Bool("no-hooks", false, "Do not write the pcs flow hooks into settings.json (remembered by sync)")
	dryRun := fs.Bool("dry-run", false, "Preview planned actions without writing files")
	asJSON := fs.Bool("json", false, "Output installation report as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	overrides := visitedFlags(fs)
	flowHooks, ok := flowHooksChoice(overrides, *withHooks, *noHooks, "install", env)
	if !ok {
		return 2
	}
	chosenScope := profileDirImpliesProfile(*scope, overrides)
	if *profileAlias {
		chosenScope = installer.ScopeProfile
		overrides["scope"] = true
	}

	// The global default must never silently add a second install next to an
	// existing profile-only install: require an explicit --scope instead.
	if !explicitTargetFlag(overrides) && strings.TrimSpace(os.Getenv("PCS_INSTALL_SCOPE")) == "" {
		found, err := installer.DiscoverInstalls(installer.Options{HomeDir: env.HomeDir, ClaudeConfigDir: env.ClaudeConfigDir})
		if err != nil {
			fmt.Fprintf(env.Stderr, "pcs install failed: %v\n", err)
			return 1
		}
		hasGlobal := false
		var profiles []string
		for _, inst := range found {
			switch inst.Scope {
			case installer.ScopeGlobal:
				hasGlobal = true
			case installer.ScopeProfile:
				profiles = append(profiles, inst.TargetDir)
			}
		}
		if !hasGlobal && len(profiles) > 0 {
			fmt.Fprintf(env.Stderr, "pcs install failed: a pcs profile install exists at %s and there is no global install; pass --scope profile to re-install the profile, or --scope global to install into the main Claude profile\n", strings.Join(profiles, ", "))
			return 1
		}
	}

	rep, err := installer.Install(installer.Options{
		Scope:           chosenScope,
		TargetDir:       *targetDir,
		ProfileDir:      *profileDir,
		HomeDir:         env.HomeDir,
		ClaudeConfigDir: env.ClaudeConfigDir,
		Adapter:         *adapter,
		Persona:         *persona,
		MCPGuidance:     installer.BoolPtr(*mcpGuidance),
		WriteMCPConfig:  *writeMCPConfig,
		WriteHook:       *writeHook,
		WithMCP:         *withMCP,
		LauncherDir:     *launcherDir,
		LauncherArgs:    *launcherArgs,
		WithPlugins:     *withPlugins,
		NoStatusLine:    *noStatusLine,
		FlowHooks:       flowHooks,
		FlowStatusLine:  flagBoolPtr(overrides, "with-flow-statusline", *withFlowStatusLine),
		DryRun:          *dryRun,
		FlagOverrides:   overrides,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs install failed: %v\n", err)
		return 1
	}
	printWarnings(env.Stderr, rep)
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
	profileAlias := fs.Bool("profile", false, "Shorthand for --scope profile")
	profileDir := fs.String("profile-dir", "", "Custom profile directory when using --scope profile")
	targetDir := fs.String("target-dir", "", "Custom target directory")
	adapter := fs.String("adapter", "", "Override frontend stack adapter (e.g. react-shadcn)")
	persona := fs.String("persona", "", "Override coordinator persona variant: gentleman or neutral")
	mcpGuidance := fs.Bool("mcp-guidance", true, "Override whether pcs:codegraph-guidance and pcs:engram-protocol prompt blocks are injected")
	writeMCPConfig := fs.Bool("write-mcp-config", false, "Override whether missing MCP servers and permissions are written to .claude.json and settings.json")
	writeHook := fs.Bool("write-hook", false, "Override whether the UserPromptSubmit skill-registry hook is written to settings.json")
	withMCP := fs.Bool("with-mcp", false, "Convenience alias for --mcp-guidance=true --write-mcp-config=true --write-hook=true")
	launcherDir := fs.String("launcher-dir", "", "Custom launcher installation directory")
	launcherArgs := fs.String("launcher-args", "", "Default arguments baked into the claude-pcs launchers: split on whitespace, single or double quotes group words, no escape character; every value is passed literally")
	withPlugins := fs.Bool("with-plugins", false, "Add the engram marketplace and install the engram@engram plugin into the profile via the claude CLI (remembered by sync)")
	noStatusLine := fs.Bool("no-statusline", false, "Do not copy statusLine from main profile into profile settings.json (remembered by sync)")
	withHooks := fs.Bool("with-hooks", false, "Write the pcs flow hooks into settings.json (remembered by sync)")
	withFlowStatusLine := fs.Bool("with-flow-statusline", true, "Profile scope: wrap the profile statusLine with pcs statusline-wrap (remembered by sync)")
	noHooks := fs.Bool("no-hooks", false, "Remove or do not write the pcs flow hooks in settings.json (remembered by sync)")
	dryRun := fs.Bool("dry-run", false, "Preview planned sync actions without writing files")
	asJSON := fs.Bool("json", false, "Output sync report as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	overrides := visitedFlags(fs)
	flowHooks, ok := flowHooksChoice(overrides, *withHooks, *noHooks, "sync", env)
	if !ok {
		return 2
	}
	chosenScope := ""
	if overrides["scope"] {
		chosenScope = *scope
	}
	if *profileAlias {
		chosenScope = installer.ScopeProfile
		overrides["scope"] = true
	}
	if chosenScope == "" && overrides["profile-dir"] {
		chosenScope = installer.ScopeProfile
		overrides["scope"] = true
	}
	chosenProfileDir := *profileDir
	chosenTargetDir := *targetDir

	// Without any explicit target selector, sync only existing installs found via
	// their .pcs-state.json; it never falls back to creating a global install.
	if chosenScope == "" && !overrides["target-dir"] && strings.TrimSpace(os.Getenv("PCS_INSTALL_SCOPE")) == "" {
		inst, ok := discoverSingleInstall("sync", "run 'pcs install' first, or pass --scope, --profile-dir, or --target-dir", env)
		if !ok {
			return 1
		}
		chosenScope = inst.Scope
		overrides["scope"] = true
		chosenProfileDir, chosenTargetDir = installDirs(inst, chosenProfileDir, chosenTargetDir)
	}

	var mcpGuidancePtr *bool
	if overrides["mcp-guidance"] {
		mcpGuidancePtr = installer.BoolPtr(*mcpGuidance)
	}

	rep, err := installer.Sync(installer.Options{
		Scope:           chosenScope,
		TargetDir:       chosenTargetDir,
		ProfileDir:      chosenProfileDir,
		HomeDir:         env.HomeDir,
		ClaudeConfigDir: env.ClaudeConfigDir,
		Adapter:         *adapter,
		Persona:         *persona,
		MCPGuidance:     mcpGuidancePtr,
		WriteMCPConfig:  *writeMCPConfig,
		WriteHook:       *writeHook,
		WithMCP:         *withMCP,
		LauncherDir:     *launcherDir,
		LauncherArgs:    *launcherArgs,
		WithPlugins:     *withPlugins,
		NoStatusLine:    *noStatusLine,
		FlowHooks:       flowHooks,
		FlowStatusLine:  flagBoolPtr(overrides, "with-flow-statusline", *withFlowStatusLine),
		DryRun:          *dryRun,
		FlagOverrides:   overrides,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs sync failed: %v\n", err)
		return 1
	}
	printWarnings(env.Stderr, rep)
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
	profileAlias := fs.Bool("profile", false, "Shorthand for --scope profile")
	profileDir := fs.String("profile-dir", "", "Custom profile directory when using --scope profile")
	targetDir := fs.String("target-dir", "", "Custom target directory")
	removeMCP := fs.Bool("remove-mcp", false, "Remove pcs MCP servers, permissions, and plugins recorded in .pcs-state.json (an empty record removes nothing); refuses without a record unless --force")
	force := fs.Bool("force", false, "With --remove-mcp and no install record, remove the default engram, context7, and codegraph servers, their permissions, and the engram@engram plugin (prints a warning)")
	dryRun := fs.Bool("dry-run", false, "Preview planned uninstall actions without removing files")
	asJSON := fs.Bool("json", false, "Output uninstall report as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	visited := visitedFlags(fs)
	if *force && !explicitTargetFlag(visited) {
		fmt.Fprintf(env.Stderr, "pcs uninstall failed: --force requires an explicit target: pass --scope (global, workspace, or profile), --profile-dir, or --target-dir on the command line (PCS_INSTALL_SCOPE alone is not enough)\n")
		return 1
	}
	chosenScope := profileDirImpliesProfile(*scope, visited)
	if *profileAlias {
		chosenScope = installer.ScopeProfile
	}
	chosenProfileDir, chosenTargetDir := *profileDir, *targetDir

	// Without any explicit target selector, uninstall behaves like sync: it only
	// removes the single existing install found via its .pcs-state.json.
	if !explicitTargetFlag(visited) && strings.TrimSpace(os.Getenv("PCS_INSTALL_SCOPE")) == "" {
		inst, ok := discoverSingleInstall("uninstall", "pass --scope, --profile-dir, or --target-dir to target one explicitly", env)
		if !ok {
			return 1
		}
		chosenScope = inst.Scope
		chosenProfileDir, chosenTargetDir = installDirs(inst, chosenProfileDir, chosenTargetDir)
	}

	rep, err := installer.Uninstall(installer.Options{
		Scope:           chosenScope,
		TargetDir:       chosenTargetDir,
		ProfileDir:      chosenProfileDir,
		HomeDir:         env.HomeDir,
		ClaudeConfigDir: env.ClaudeConfigDir,
		RemoveMCP:       *removeMCP,
		Force:           *force,
		DryRun:          *dryRun,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs uninstall failed: %v\n", err)
		return 1
	}
	printWarnings(env.Stderr, rep)
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
	profileAlias := fs.Bool("profile", false, "Shorthand for --scope profile")
	profileDir := fs.String("profile-dir", "", "Custom profile directory when using --scope profile")
	targetDir := fs.String("target-dir", "", "Custom target directory")
	asJSON := fs.Bool("json", false, "Output status report as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	chosenScope := profileDirImpliesProfile(*scope, visitedFlags(fs))
	if *profileAlias {
		chosenScope = installer.ScopeProfile
	}

	rep, err := installer.Status(installer.Options{
		Scope:           chosenScope,
		TargetDir:       *targetDir,
		ProfileDir:      *profileDir,
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

// flowHooksChoice maps --with-hooks/--no-hooks onto installer.Options.FlowHooks:
// nil when neither flag was passed (scope default, or the saved choice on sync).
func flowHooksChoice(visited map[string]bool, withHooks, noHooks bool, cmd string, env Env) (*bool, bool) {
	with := visited["with-hooks"] && withHooks
	without := visited["no-hooks"] && noHooks
	switch {
	case with && without:
		fmt.Fprintf(env.Stderr, "pcs %s: --with-hooks and --no-hooks are mutually exclusive\n", cmd)
		return nil, false
	case with:
		return installer.BoolPtr(true), true
	case without:
		return installer.BoolPtr(false), true
	case visited["with-hooks"]:
		return installer.BoolPtr(false), true
	case visited["no-hooks"]:
		return installer.BoolPtr(true), true
	}
	return nil, true
}

// flagBoolPtr returns a pointer to value when the named flag was passed, and
// nil otherwise (the scope default, or the saved choice on sync).
func flagBoolPtr(visited map[string]bool, name string, value bool) *bool {
	if !visited[name] {
		return nil
	}
	return installer.BoolPtr(value)
}

// profileDirImpliesProfile returns the profile scope when --profile-dir was given
// without an explicit --scope or PCS_INSTALL_SCOPE, so a profile directory is
// never silently ignored in favor of the global target.
func profileDirImpliesProfile(scope string, visited map[string]bool) string {
	if visited["profile-dir"] && !visited["scope"] && strings.TrimSpace(os.Getenv("PCS_INSTALL_SCOPE")) == "" {
		return installer.ScopeProfile
	}
	return scope
}

// explicitTargetFlag reports whether a target selector was passed on the
// command line: --scope, --profile, --profile-dir, or --target-dir.
func explicitTargetFlag(visited map[string]bool) bool {
	return visited["scope"] || visited["profile"] || visited["profile-dir"] || visited["target-dir"]
}

// discoverSingleInstall returns the only existing pcs install (global target or
// default profile). It prints the refusal and returns false when none or
// several are found; hint tells the user how to proceed when none exists.
func discoverSingleInstall(cmd, hint string, env Env) (installer.Installation, bool) {
	found, err := installer.DiscoverInstalls(installer.Options{HomeDir: env.HomeDir, ClaudeConfigDir: env.ClaudeConfigDir})
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs %s failed: %v\n", cmd, err)
		return installer.Installation{}, false
	}
	switch len(found) {
	case 0:
		fmt.Fprintf(env.Stderr, "pcs %s failed: no pcs installation found (checked the global target and the default profile); %s\n", cmd, hint)
	case 1:
		return found[0], true
	default:
		fmt.Fprintf(env.Stderr, "pcs %s failed: multiple pcs installations found; re-run with --scope (and --profile-dir for a custom profile) to choose one:\n", cmd)
		for _, inst := range found {
			fmt.Fprintf(env.Stderr, "  - %s: %s\n", inst.Scope, inst.TargetDir)
		}
	}
	return installer.Installation{}, false
}

// installDirs maps a discovered install onto the --profile-dir or --target-dir
// value that selects it, keeping the given values otherwise.
func installDirs(inst installer.Installation, profileDir, targetDir string) (string, string) {
	switch inst.Scope {
	case installer.ScopeProfile:
		profileDir = inst.TargetDir
	case installer.ScopeWorkspace:
		targetDir = inst.TargetDir
	}
	return profileDir, targetDir
}

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	m := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		m[f.Name] = true
	})
	return m
}

func runDoctor(args []string, env Env) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	scope := fs.String("scope", "", "Target scope: global, workspace, or profile")
	profileAlias := fs.Bool("profile", false, "Shorthand for --scope profile")
	profileDir := fs.String("profile-dir", "", "Custom profile directory when inspecting profile scope")
	targetDir := fs.String("target-dir", "", "Optional target directory to inspect")
	asJSON := fs.Bool("json", false, "Output diagnostic report as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	chosenScope := profileDirImpliesProfile(*scope, visitedFlags(fs))
	if *profileAlias {
		chosenScope = installer.ScopeProfile
	}

	rep, err := doctor.Run(doctor.Options{
		Scope:      chosenScope,
		TargetDir:  *targetDir,
		ProfileDir: *profileDir,
		HomeDir:    env.HomeDir,
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

func runHook(args []string, env Env) int {
	if len(args) != 1 || !slices.Contains(hook.Events, args[0]) {
		fmt.Fprintf(env.Stderr, "usage: pcs hook %s  (reads the Claude Code hook JSON from stdin)\n", strings.Join(hook.Events, "|"))
		return 2
	}
	return hook.Run(args[0], env.Stdin, env.Stdout)
}

func runCodeGraph(args []string, env Env) int {
	if len(args) == 0 || args[0] != "init" {
		fmt.Fprintf(env.Stderr, "usage: pcs codegraph init [--cwd <dir>] [--json]\n")
		return 2
	}

	fs := flag.NewFlagSet("codegraph init", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	cwd := fs.String("cwd", env.WorkDir, "Project root directory to initialize")
	asJSON := fs.Bool("json", false, "Output initialization result as JSON")

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	root := strings.TrimSpace(*cwd)
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(env.Stderr, "pcs codegraph init: %v\n", err)
			return 1
		}
	}

	absRoot, err := validateCodeGraphRoot(root, env.HomeDir)
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs codegraph init refused: %v\n", err)
		return 1
	}

	execFn := env.ExecCommand
	if execFn == nil {
		execFn = func(name string, cmdArgs ...string) ([]byte, error) {
			return exec.Command(name, cmdArgs...).CombinedOutput()
		}
	}

	out, err := execFn("codegraph", "init", absRoot)
	if err != nil {
		fmt.Fprintf(env.Stderr, "pcs codegraph init failed: %v (%s)\n", err, strings.TrimSpace(string(out)))
		return 1
	}

	if *asJSON {
		return writeJSONOutput(env.Stdout, map[string]any{
			"status":       "initialized",
			"project_root": absRoot,
			"output":       strings.TrimSpace(string(out)),
		})
	}
	fmt.Fprintf(env.Stdout, "Initialized CodeGraph index at %s\n", absRoot)
	return 0
}

func validateCodeGraphRoot(root, homeDir string) (string, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "/" || trimmed == `\` {
		return "", fmt.Errorf("cannot initialize CodeGraph at filesystem root %q", trimmed)
	}

	absRoot, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", root, err)
	}
	cleanRoot := filepath.Clean(absRoot)

	if filepath.Dir(cleanRoot) == cleanRoot {
		return "", fmt.Errorf("cannot initialize CodeGraph at filesystem root %q", cleanRoot)
	}

	home := strings.TrimSpace(homeDir)
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home != "" {
		if absHome, err := filepath.Abs(home); err == nil {
			if equalCleanPath(absHome, cleanRoot) {
				return "", fmt.Errorf("cannot initialize CodeGraph in user home directory %q", cleanRoot)
			}
		}
	}

	// Temporary directories are refused before the existence check so the
	// refusal reason is stable on every platform. The Unix roots only apply off
	// Windows, where "/tmp" resolves to an unrelated "<drive>:\tmp".
	var forbiddenTempRoots []string
	if runtime.GOOS == "windows" {
		if systemRoot := strings.TrimSpace(os.Getenv("SystemRoot")); systemRoot != "" {
			forbiddenTempRoots = append(forbiddenTempRoots, filepath.Join(systemRoot, "Temp"))
		}
	} else {
		forbiddenTempRoots = append(forbiddenTempRoots, "/tmp", "/var/tmp", "/private/tmp", "/private/var/tmp")
	}
	if sysTmp := strings.TrimSpace(os.TempDir()); sysTmp != "" {
		if absTmp, err := filepath.Abs(sysTmp); err == nil {
			forbiddenTempRoots = append(forbiddenTempRoots, absTmp)
			if evalTmp, err := filepath.EvalSymlinks(absTmp); err == nil {
				forbiddenTempRoots = append(forbiddenTempRoots, evalTmp)
			}
		}
	}
	evalRoot := cleanRoot
	if resolved, err := filepath.EvalSymlinks(cleanRoot); err == nil {
		evalRoot = resolved
	}
	for _, forbidden := range forbiddenTempRoots {
		if equalCleanPath(cleanRoot, forbidden) || equalCleanPath(evalRoot, forbidden) {
			return "", fmt.Errorf("cannot initialize CodeGraph in temporary directory %q", cleanRoot)
		}
	}

	info, err := os.Stat(cleanRoot)
	if err != nil {
		return "", fmt.Errorf("target directory %q does not exist: %w", cleanRoot, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("target path %q is not a directory", cleanRoot)
	}

	return cleanRoot, nil
}

func equalCleanPath(a, b string) bool {
	na := strings.ToLower(strings.TrimSuffix(filepath.ToSlash(filepath.Clean(a)), "/"))
	nb := strings.ToLower(strings.TrimSuffix(filepath.ToSlash(filepath.Clean(b)), "/"))
	return na != "" && na == nb
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
	if len(rep.NextSteps) > 0 {
		fmt.Fprintf(w, "\nNext steps:\n")
		for _, s := range rep.NextSteps {
			fmt.Fprintf(w, "  • %s\n", s)
		}
	}
}

// printWarnings writes every report warning to w (stderr), so they stay visible
// even when stdout carries --json output.
func printWarnings(w io.Writer, rep *installer.Report) {
	for _, warning := range rep.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warning)
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
  install                  Install pcs orchestrator, 5 native subagents, skills, and optional MCP config
                           (without --scope it refuses when only a profile install exists)
  sync                     Re-sync an existing installation (never creates one; without --scope it
                           discovers existing installs and requires --scope when several exist)
  uninstall                Cleanly remove pcs-owned markers, native subagents, and skills (without --scope
                           it discovers existing installs like sync)
  status                   Inspect installed markers, native subagents, skills, and MCP servers
  doctor                   Run diagnostic checks on MCP binaries, embedded brain, and target dir
  skill-registry refresh   Scan SKILL.md files and generate .atl/pcs-skill-registry.md
  codegraph init           Validate project root and initialize CodeGraph index (.codegraph/)
  flow <subcommand>        Deterministic PCD flow state: start, status, next, record, close
                           (state in <project>/.pcs/flows/<feature>.json)
  hook <event>             Claude Code hook entry point: pre-tool-use, subagent-stop, stop,
                           user-prompt-submit (installed by default in profile scope)
  statusline [--cwd <dir>] Print the active flow segment (e.g. "pcs:x t2 ▸ apply 4/5"); reads the
                           Claude Code statusLine stdin JSON; prints nothing without a flow
  statusline-wrap          Profile statusLine command: runs the base command recorded in
                           --state <.pcs-state.json> (2s timeout) and appends " │ " + the segment
  version                  Print pcs CLI version
  help                     Show this help message

Key Flags (install / sync / uninstall / status):
  --scope global|workspace|profile   Target deployment mode (default: global -> ~/.claude or $CLAUDE_CONFIG_DIR; env: PCS_INSTALL_SCOPE)
  --profile-dir <dir>                Profile directory (default: ~/.claude-pcs; implies --scope profile; ignores $CLAUDE_CONFIG_DIR)
  --target-dir <dir>                 Override target configuration directory
  --adapter <name>                   Frontend stack adapter (default: react-shadcn)
  --persona gentleman|neutral        Coordinator persona variant (default: gentleman)
  --mcp-guidance=true|false          Inject pcs:codegraph-guidance and pcs:engram-protocol into CLAUDE.md (default: true)
  --write-mcp-config                 Write missing engram, context7, and codegraph MCP entries into .claude.json and settings.json (default: false)
  --write-hook                       Write UserPromptSubmit skill-registry refresh hook into settings.json (default: false)
  --with-hooks / --no-hooks          Write or skip the pcs flow hooks (pcs hook <event>) in settings.json
                                     (default: on for --scope profile, off otherwise; remembered by sync)
  --with-flow-statusline=true|false  (profile scope) Wrap the profile statusLine with pcs statusline-wrap
                                     (default: on unless --no-statusline; remembered by sync)
  --remove-mcp                       (uninstall) Remove only the MCP servers, permissions, and plugins pcs recorded as added
  --force                            (uninstall) With --remove-mcp and no install record, remove the default MCP entries anyway
                                     (requires --scope, --profile-dir, or --target-dir on the command line)
  --dry-run                          Preview planned actions without modifying disk
  --json                             Emit structured JSON output
`)
}
