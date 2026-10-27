package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/yorth21/pcs/internal/assets"
	"github.com/yorth21/pcs/internal/marker"
	"github.com/yorth21/pcs/internal/mcp"
	"github.com/yorth21/pcs/internal/registry"
)

const (
	ScopeProfile   = "profile"
	ScopeGlobal    = "global"
	ScopeWorkspace = "workspace"

	DefaultAdapter = "react-shadcn"
	DefaultPersona = "gentleman"
	Version        = "0.1.0"

	StateFileName          = ".pcs-state.json"
	AgentOwnershipFileName = ".pcs-native-agent-ownership.json"
)

// ErrNotInstalled reports that the resolved target has no .pcs-state.json, so
// commands that only operate on existing installations (sync) must refuse.
var ErrNotInstalled = errors.New("no pcs installation found")

// ErrNoMCPRecord reports that uninstall --remove-mcp cannot prove which MCP
// servers, permissions, and plugins pcs added because no state record exists.
var ErrNoMCPRecord = errors.New("no record of pcs-added MCP servers, permissions, or plugins")

// Installation identifies an existing pcs install discovered via its state file.
type Installation struct {
	Scope     string `json:"scope"`
	TargetDir string `json:"target_dir"`
}

// BoolPtr returns a pointer to b for configuring optional boolean flags.
func BoolPtr(b bool) *bool {
	return &b
}

// Options configures an Install, Sync, Uninstall, or Status operation.
type Options struct {
	Scope           string
	TargetDir       string
	ProfileDir      string
	HomeDir         string
	ClaudeConfigDir string
	Adapter         string
	Persona         string
	MCPGuidance     *bool
	WriteMCPConfig  bool
	WriteHook       bool
	WithMCP         bool
	DryRun          bool
	RemoveMCP       bool
	Force           bool
	LauncherDir     string
	LauncherArgs    string
	NoStatusLine    bool
	WithPlugins     bool
	FlagOverrides   map[string]bool
}

// State persists the installed pcs configuration in <target-dir>/.pcs-state.json.
type State struct {
	Version        string   `json:"version"`
	InstalledAt    string   `json:"installed_at"`
	Scope          string   `json:"scope"`
	TargetDir      string   `json:"target_dir"`
	Adapter        string   `json:"adapter"`
	Persona        string   `json:"persona"`
	MCPGuidance    bool     `json:"mcp_guidance"`
	WriteMCPConfig bool     `json:"write_mcp_config"`
	WriteHook      bool     `json:"write_hook"`
	WithMCP        bool     `json:"with_mcp"`
	EnabledMCPs    []string `json:"enabled_mcps"`
	// AddedMCPServers, AddedPermissions, and AddedPlugins record exactly what pcs
	// added. They are never omitted: an empty list ([]) is a record that pcs
	// added nothing, while a missing or null field (nil) means no record exists.
	AddedMCPServers     []string `json:"added_mcp_servers"`
	AddedPermissions    []string `json:"added_permissions"`
	AddedPlugins        []string `json:"added_plugins"`
	AddedHook           bool     `json:"added_hook,omitempty"`
	CreatedClaudeMD     bool     `json:"created_claude_md,omitempty"`
	CreatedClaudeJSON   bool     `json:"created_claude_json,omitempty"`
	CreatedSettingsJSON bool     `json:"created_settings_json,omitempty"`
	LauncherDir         string   `json:"launcher_dir,omitempty"`
	LauncherArgs        string   `json:"launcher_args,omitempty"`
	// NoStatusLine and WithPlugins persist the opt-out/opt-in install flags so a
	// later sync honors them; a legacy state without them keeps the defaults.
	NoStatusLine    bool `json:"no_statusline,omitempty"`
	WithPlugins     bool `json:"with_plugins,omitempty"`
	AddedStatusLine bool `json:"added_status_line,omitempty"`
	// StatusLineSHA256 is the digest of the statusLine value pcs copied, so
	// uninstall removes it only while it is unchanged.
	StatusLineSHA256 string   `json:"status_line_sha256,omitempty"`
	SharedMCPFile    string   `json:"shared_mcp_file,omitempty"`
	ManagedFiles     []string `json:"managed_files"`
}

// hasMCPDeltaRecord reports whether the state records exactly which MCP servers,
// permissions, and plugins pcs added (possibly none).
func (s *State) hasMCPDeltaRecord() bool {
	return s != nil && s.AddedMCPServers != nil && s.AddedPermissions != nil && s.AddedPlugins != nil
}

// AgentOwnership tracks which native subagent files in agents/ are owned by pcs.
type AgentOwnership struct {
	Manager string   `json:"manager"`
	Agents  []string `json:"agents"`
}

// Action describes a single planned or executed filesystem change.
type Action struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
	Detail    string `json:"detail,omitempty"`
}

// Report summarizes an Install, Sync, or Uninstall run.
type Report struct {
	Operation      string   `json:"operation"`
	Scope          string   `json:"scope"`
	TargetDir      string   `json:"target_dir"`
	Adapter        string   `json:"adapter"`
	Persona        string   `json:"persona"`
	MCPGuidance    bool     `json:"mcp_guidance"`
	WriteMCPConfig bool     `json:"write_mcp_config"`
	WriteHook      bool     `json:"write_hook"`
	WithMCP        bool     `json:"with_mcp"`
	DryRun         bool     `json:"dry_run"`
	LauncherCmd    string   `json:"launcher_cmd,omitempty"`
	LauncherDir    string   `json:"launcher_dir,omitempty"`
	LauncherOnPath bool     `json:"launcher_on_path"`
	NextSteps      []string `json:"next_steps,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
	Actions        []Action `json:"actions"`
}

// StatusReport summarizes the health and installed state of a target directory.
type StatusReport struct {
	Installed        bool   `json:"installed"`
	Scope            string `json:"scope"`
	TargetDir        string `json:"target_dir"`
	ClaudeMDPath     string `json:"claude_md_path"`
	ClaudeJSONPath   string `json:"claude_json_path"`
	SettingsJSONPath string `json:"settings_json_path"`
	SharedMCPPath    string `json:"shared_mcp_path,omitempty"`
	// MainClaudeJSONPath is the main profile .claude.json that profile scope
	// mirrors into shared-mcp.json.
	MainClaudeJSONPath string          `json:"main_claude_json_path,omitempty"`
	State              *State          `json:"state,omitempty"`
	Markers            map[string]bool `json:"markers"`
	Agents             map[string]bool `json:"agents"`
	Skills             map[string]bool `json:"skills"`
	MCPServers         map[string]bool `json:"mcp_servers"`
}

type resolvedLayout struct {
	scope              string
	targetDir          string
	claudeMDPath       string
	claudeJSONPath     string
	settingsJSONPath   string
	agentsDir          string
	skillsDir          string
	launcherPath       string
	sharedMCPPath      string
	mainClaudeJSONPath string
	mainSettingsPath   string
	userBinDir         string
	userBinLauncher    string
	workspaceRoot      string
}

func normalizeOptions(opts *Options) {
	if opts.FlagOverrides["with-mcp"] {
		if opts.WithMCP {
			if !opts.FlagOverrides["mcp-guidance"] {
				opts.MCPGuidance = BoolPtr(true)
			}
			if !opts.FlagOverrides["write-mcp-config"] {
				opts.WriteMCPConfig = true
			}
			if !opts.FlagOverrides["write-hook"] {
				opts.WriteHook = true
			}
		} else {
			if !opts.FlagOverrides["mcp-guidance"] {
				opts.MCPGuidance = BoolPtr(false)
			}
			if !opts.FlagOverrides["write-mcp-config"] {
				opts.WriteMCPConfig = false
			}
			if !opts.FlagOverrides["write-hook"] {
				opts.WriteHook = false
			}
		}
	} else if opts.WithMCP {
		if opts.MCPGuidance == nil && !opts.FlagOverrides["mcp-guidance"] {
			opts.MCPGuidance = BoolPtr(true)
		}
		if !opts.FlagOverrides["write-mcp-config"] {
			opts.WriteMCPConfig = true
		}
		if !opts.FlagOverrides["write-hook"] {
			opts.WriteHook = true
		}
	}
	if opts.MCPGuidance == nil {
		opts.MCPGuidance = BoolPtr(true)
	}
	opts.WithMCP = opts.WriteMCPConfig && opts.WriteHook && *opts.MCPGuidance
}

func isDirOnPath(dir string) bool {
	if dir == "" {
		return false
	}
	cleanDir := filepath.Clean(dir)
	pathEnv := os.Getenv("PATH")
	for _, p := range filepath.SplitList(pathEnv) {
		if filepath.Clean(p) == cleanDir || (runtime.GOOS == "windows" && strings.EqualFold(filepath.Clean(p), cleanDir)) {
			return true
		}
	}
	return false
}

func resolveLayout(opts Options) (*resolvedLayout, error) {
	scope := strings.ToLower(strings.TrimSpace(opts.Scope))
	if scope == "" {
		scope = strings.ToLower(strings.TrimSpace(os.Getenv("PCS_INSTALL_SCOPE")))
	}
	if scope == "" {
		scope = ScopeGlobal
	}
	if scope != ScopeProfile && scope != ScopeGlobal && scope != ScopeWorkspace {
		return nil, fmt.Errorf("invalid scope %q (supported: global, workspace, profile)", opts.Scope)
	}

	homeDir := strings.TrimSpace(opts.HomeDir)
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
	}
	absHome, err := filepath.Abs(homeDir)
	if err != nil {
		absHome = homeDir
	}

	claudeConfigDir := strings.TrimSpace(opts.ClaudeConfigDir)
	if claudeConfigDir == "" {
		claudeConfigDir = strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	}

	profileDir := strings.TrimSpace(opts.ProfileDir)
	targetDir := strings.TrimSpace(opts.TargetDir)
	usedCustomClaudeConfigDir := false
	if targetDir == "" {
		switch scope {
		case ScopeProfile:
			// Profile scope deliberately ignores CLAUDE_CONFIG_DIR: only
			// --profile-dir (or --target-dir) may select the profile target.
			if profileDir != "" {
				targetDir = profileDir
			} else {
				targetDir = filepath.Join(absHome, ".claude-pcs")
			}
		case ScopeGlobal:
			if claudeConfigDir != "" {
				targetDir = claudeConfigDir
				usedCustomClaudeConfigDir = true
			} else {
				targetDir = filepath.Join(absHome, ".claude")
			}
		case ScopeWorkspace:
			cwd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("resolve working directory: %w", err)
			}
			targetDir = filepath.Join(cwd, ".claude")
		}
	}

	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("resolve target directory: %w", err)
	}

	layout := &resolvedLayout{
		scope:            scope,
		targetDir:        absTarget,
		settingsJSONPath: filepath.Join(absTarget, "settings.json"),
		agentsDir:        filepath.Join(absTarget, "agents"),
		skillsDir:        filepath.Join(absTarget, "skills"),
	}

	switch scope {
	case ScopeProfile:
		if err := guardProfileTarget(absTarget, absHome, claudeConfigDir); err != nil {
			return nil, err
		}
		layout.claudeMDPath = filepath.Join(absTarget, "CLAUDE.md")
		layout.claudeJSONPath = filepath.Join(absTarget, ".claude.json")
		layout.launcherPath = filepath.Join(absTarget, "bin", "claude-pcs")
		layout.sharedMCPPath = filepath.Join(absTarget, "shared-mcp.json")
		layout.mainClaudeJSONPath = filepath.Join(absHome, ".claude.json")
		layout.mainSettingsPath = filepath.Join(absHome, ".claude", "settings.json")
		if opts.LauncherDir != "" {
			absLauncherDir, err := filepath.Abs(opts.LauncherDir)
			if err == nil {
				layout.userBinDir = absLauncherDir
			} else {
				layout.userBinDir = opts.LauncherDir
			}
		} else {
			if runtime.GOOS == "windows" {
				localAppData := os.Getenv("LOCALAPPDATA")
				if localAppData == "" {
					localAppData = filepath.Join(absHome, "AppData", "Local")
				}
				layout.userBinDir = filepath.Join(localAppData, "pcs", "bin")
			} else {
				layout.userBinDir = filepath.Join(absHome, ".local", "bin")
			}
		}
		layout.userBinLauncher = filepath.Join(layout.userBinDir, "claude-pcs")
	case ScopeGlobal:
		layout.claudeMDPath = filepath.Join(absTarget, "CLAUDE.md")
		defaultGlobalDir := filepath.Join(absHome, ".claude")
		if usedCustomClaudeConfigDir && filepath.Clean(absTarget) != filepath.Clean(defaultGlobalDir) {
			layout.claudeJSONPath = filepath.Join(absTarget, ".claude.json")
		} else {
			// Default global layout: ~/.claude with ~/.claude.json in the parent directory.
			layout.claudeJSONPath = filepath.Join(filepath.Dir(absTarget), ".claude.json")
		}
	case ScopeWorkspace:
		// If the user passed a repo root or a .claude directory, normalize so
		// targetDir is <workspace>/.claude and CLAUDE.md is at <workspace>/CLAUDE.md.
		if filepath.Base(absTarget) != ".claude" {
			layout.workspaceRoot = absTarget
			layout.targetDir = filepath.Join(absTarget, ".claude")
			layout.settingsJSONPath = filepath.Join(layout.targetDir, "settings.json")
			layout.agentsDir = filepath.Join(layout.targetDir, "agents")
			layout.skillsDir = filepath.Join(layout.targetDir, "skills")
		} else {
			layout.workspaceRoot = filepath.Dir(absTarget)
		}
		layout.claudeMDPath = filepath.Join(layout.workspaceRoot, "CLAUDE.md")
		layout.claudeJSONPath = filepath.Join(layout.workspaceRoot, ".mcp.json")
	}

	return layout, nil
}

// guardProfileTarget refuses a profile target that overlaps the main Claude
// profile: the home directory, the main configuration directory (~/.claude, or
// $CLAUDE_CONFIG_DIR when set) or any of its ancestors, or a directory whose
// derived .claude.json or settings.json is the main profile's file (for example
// through a symlink). The $CLAUDE_CONFIG_DIR checks are skipped when that
// directory already holds a pcs profile install, which is the case inside a
// claude-pcs launcher session; ~/.claude is never exempt.
func guardProfileTarget(absTarget, absHome, claudeConfigDir string) error {
	if samePath(absTarget, absHome) {
		return fmt.Errorf("refusing profile target %s: it is the home directory, so the profile .claude.json would be the main ~/.claude.json; choose another --profile-dir", absTarget)
	}
	type mainProfile struct {
		dir, label string
		files      []string
	}
	mainDir := filepath.Join(absHome, ".claude")
	mains := []mainProfile{{
		dir:   mainDir,
		label: "the main Claude config directory",
		files: []string{filepath.Join(absHome, ".claude.json"), filepath.Join(mainDir, ".claude.json"), filepath.Join(mainDir, "settings.json")},
	}}
	if claudeConfigDir != "" {
		absConfig, err := filepath.Abs(claudeConfigDir)
		if err != nil {
			absConfig = claudeConfigDir
		}
		if saved, err := readState(filepath.Join(absConfig, StateFileName)); err != nil || saved.Scope != ScopeProfile {
			mains = append(mains, mainProfile{
				dir:   absConfig,
				label: "the main Claude config directory ($CLAUDE_CONFIG_DIR)",
				files: []string{filepath.Join(absConfig, ".claude.json"), filepath.Join(absConfig, "settings.json")},
			})
		}
	}

	derived := []string{filepath.Join(absTarget, ".claude.json"), filepath.Join(absTarget, "settings.json")}
	for _, m := range mains {
		if samePath(absTarget, m.dir) {
			return fmt.Errorf("refusing profile target %s: it is %s; choose another --profile-dir", absTarget, m.label)
		}
		if isAncestorPath(absTarget, m.dir) {
			return fmt.Errorf("refusing profile target %s: it contains %s %s; choose another --profile-dir", absTarget, m.label, m.dir)
		}
		for _, own := range derived {
			for _, mainFile := range m.files {
				if samePath(own, mainFile) {
					return fmt.Errorf("refusing profile target %s: its %s is the main profile file %s; choose another --profile-dir", absTarget, filepath.Base(own), mainFile)
				}
			}
		}
	}
	return nil
}

// isAncestorPath reports whether ancestor strictly contains p after both are
// canonicalized like samePath. Comparison is case-insensitive on Windows.
func isAncestorPath(ancestor, p string) bool {
	a, b := canonicalPath(ancestor), canonicalPath(p)
	if runtime.GOOS == "windows" {
		a, b = strings.ToLower(a), strings.ToLower(b)
	}
	rel, err := filepath.Rel(a, b)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// samePath reports whether a and b name the same location after making them
// absolute, cleaning them, and resolving symlinks of their longest existing
// ancestor. Comparison is case-insensitive on Windows.
func samePath(a, b string) bool {
	ra, rb := canonicalPath(a), canonicalPath(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(ra, rb)
	}
	return ra == rb
}

func canonicalPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	rest := ""
	cur := abs
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// DiscoverInstalls returns the existing pcs installs among the global target
// (~/.claude or $CLAUDE_CONFIG_DIR) and the default profile (~/.claude-pcs),
// identified by their .pcs-state.json. Duplicate paths are reported once, and
// each install carries the scope recorded in its state file.
func DiscoverInstalls(opts Options) ([]Installation, error) {
	var found []Installation
	for _, scope := range []string{ScopeGlobal, ScopeProfile} {
		layout, err := resolveLayout(Options{Scope: scope, HomeDir: opts.HomeDir, ClaudeConfigDir: opts.ClaudeConfigDir})
		if err != nil {
			// A refused candidate (e.g. profile equal to the main dir) is not an install.
			continue
		}
		statePath := filepath.Join(layout.targetDir, StateFileName)
		saved, err := readState(statePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", statePath, err)
		}
		if slices.ContainsFunc(found, func(i Installation) bool { return samePath(i.TargetDir, layout.targetDir) }) {
			continue
		}
		recordedScope := saved.Scope
		if recordedScope == "" {
			recordedScope = scope
		}
		found = append(found, Installation{Scope: recordedScope, TargetDir: layout.targetDir})
	}
	return found, nil
}

// Install deploys the pcs orchestrator blocks, native subagents, core & adapter
// skills, and optional MCP configuration into the target scope.
func Install(opts Options) (*Report, error) {
	normalizeOptions(&opts)
	return applyInstall(opts, "install")
}

// Sync re-reads .pcs-state.json, merges any explicit CLI flag overrides, and
// refreshes all embedded brain files, CLAUDE.md markers, MCP wiring, and skill
// registries. It never creates a new installation: when the resolved target has
// no .pcs-state.json it returns an error wrapping ErrNotInstalled.
func Sync(opts Options) (*Report, error) {
	layout, err := resolveLayout(opts)
	if err != nil {
		return nil, err
	}
	statePath := filepath.Join(layout.targetDir, StateFileName)
	saved, err := readState(statePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w at %s (missing %s); run 'pcs install' first", ErrNotInstalled, layout.targetDir, StateFileName)
		}
		return nil, fmt.Errorf("read %s: %w", statePath, err)
	}
	if !opts.FlagOverrides["scope"] && opts.Scope == "" && saved.Scope != "" {
		opts.Scope = saved.Scope
		if saved.Scope != layout.scope && opts.TargetDir == "" {
			// Keep syncing the directory whose state file was read.
			opts.TargetDir = layout.targetDir
		}
	}
	if !opts.FlagOverrides["adapter"] && opts.Adapter == "" {
		opts.Adapter = saved.Adapter
	}
	if !opts.FlagOverrides["persona"] && opts.Persona == "" {
		opts.Persona = saved.Persona
	}
	if !opts.FlagOverrides["with-mcp"] {
		if !opts.FlagOverrides["mcp-guidance"] && opts.MCPGuidance == nil {
			opts.MCPGuidance = BoolPtr(saved.MCPGuidance)
		}
		if !opts.FlagOverrides["write-mcp-config"] {
			opts.WriteMCPConfig = saved.WriteMCPConfig
		}
		if !opts.FlagOverrides["write-hook"] {
			opts.WriteHook = saved.WriteHook
		}
	}
	if !opts.FlagOverrides["launcher-dir"] && opts.LauncherDir == "" {
		opts.LauncherDir = saved.LauncherDir
	}
	if !opts.FlagOverrides["launcher-args"] && opts.LauncherArgs == "" {
		opts.LauncherArgs = saved.LauncherArgs
	}
	if !opts.FlagOverrides["no-statusline"] && !opts.NoStatusLine {
		opts.NoStatusLine = saved.NoStatusLine
	}
	if !opts.FlagOverrides["with-plugins"] && !opts.WithPlugins {
		opts.WithPlugins = saved.WithPlugins
	}
	normalizeOptions(&opts)
	return applyInstall(opts, "sync")
}

func applyInstall(opts Options, opName string) (*Report, error) {
	if strings.TrimSpace(opts.Adapter) == "" {
		opts.Adapter = DefaultAdapter
	}
	if strings.TrimSpace(opts.Persona) == "" {
		opts.Persona = DefaultPersona
	}
	mcpGuidance := true
	if opts.MCPGuidance != nil {
		mcpGuidance = *opts.MCPGuidance
	}

	adapterDirName, err := assets.AdapterSkillDirName(opts.Adapter)
	if err != nil {
		return nil, err
	}
	personaBlock, err := assets.PersonaBlock(opts.Persona)
	if err != nil {
		return nil, err
	}
	orchBlock, err := assets.OrchestratorBlock()
	if err != nil {
		return nil, err
	}
	routingBlock, err := assets.UXRoutingBlock()
	if err != nil {
		return nil, err
	}
	agentsList, err := assets.NativeAgents()
	if err != nil {
		return nil, err
	}
	sharedContract, err := assets.SharedPhaseContract()
	if err != nil {
		return nil, err
	}
	coreFiles, err := assets.CoreSkillFiles()
	if err != nil {
		return nil, err
	}
	adapterFiles, err := assets.AdapterSkillFiles(opts.Adapter)
	if err != nil {
		return nil, err
	}

	launcherArgList, err := SplitLauncherArgs(opts.LauncherArgs)
	if err != nil {
		return nil, err
	}

	layout, err := resolveLayout(opts)
	if err != nil {
		return nil, err
	}

	// Substitute custom profile/installed skills directory into prompt assets when non-default.
	resolvedSkillsSlash := filepath.ToSlash(layout.skillsDir)
	if layout.scope == ScopeProfile {
		orchBlock = strings.ReplaceAll(orchBlock, "~/.claude-pcs/skills", resolvedSkillsSlash)
		sharedContract = []byte(strings.ReplaceAll(string(sharedContract), "~/.claude-pcs/skills", resolvedSkillsSlash))
	}

	statePath := filepath.Join(layout.targetDir, StateFileName)
	prevState, _ := readState(statePath)

	report := &Report{
		Operation:      opName,
		Scope:          layout.scope,
		TargetDir:      layout.targetDir,
		Adapter:        opts.Adapter,
		Persona:        opts.Persona,
		MCPGuidance:    mcpGuidance,
		WriteMCPConfig: opts.WriteMCPConfig,
		WriteHook:      opts.WriteHook,
		WithMCP:        opts.WithMCP,
		DryRun:         opts.DryRun,
	}

	var managedFiles []string
	recordFile := func(op, p, detail string) {
		report.Actions = append(report.Actions, Action{
			Operation: op,
			Path:      p,
			Detail:    detail,
		})
		if !slices.Contains(managedFiles, p) {
			managedFiles = append(managedFiles, p)
		}
	}

	// 1. Compose CLAUDE.md with pcs-owned infrastructure, persona, orchestrator, and routing markers.
	existingClaudeMD := ""
	claudeMDExisted := false
	claudeMDPerm := os.FileMode(0o644)
	if info, statErr := os.Stat(layout.claudeMDPath); statErr == nil {
		claudeMDPerm = info.Mode().Perm()
	}
	if b, readErr := os.ReadFile(layout.claudeMDPath); readErr == nil {
		existingClaudeMD = string(b)
		claudeMDExisted = true
	}

	updatedClaudeMD := existingClaudeMD
	if mcpGuidance {
		updatedClaudeMD = marker.UpsertBlock(updatedClaudeMD, assets.MarkerCodeGraphGuidance, assets.CodeGraphGuidanceBlock())
	} else {
		updatedClaudeMD, _ = marker.RemoveBlock(updatedClaudeMD, assets.MarkerCodeGraphGuidance)
	}
	updatedClaudeMD = marker.UpsertBlock(updatedClaudeMD, assets.MarkerPersona, personaBlock)
	if mcpGuidance {
		updatedClaudeMD = marker.UpsertBlock(updatedClaudeMD, assets.MarkerEngramProtocol, assets.EngramProtocolBlock())
	} else {
		updatedClaudeMD, _ = marker.RemoveBlock(updatedClaudeMD, assets.MarkerEngramProtocol)
	}
	updatedClaudeMD = marker.UpsertBlock(updatedClaudeMD, assets.MarkerOrchestrator, orchBlock)
	updatedClaudeMD = marker.UpsertBlock(updatedClaudeMD, assets.MarkerUXRouting, routingBlock)

	if !opts.DryRun {
		if claudeMDExisted {
			if err := mcp.WritePrePCSBackupOnce(layout.claudeMDPath, []byte(existingClaudeMD), claudeMDPerm); err != nil {
				return nil, err
			}
		}
		if err := writeFileCreatingDirs(layout.claudeMDPath, []byte(updatedClaudeMD), claudeMDPerm); err != nil {
			return nil, err
		}
	}
	markerSummary := "pcs:persona, pcs:orchestrator, pcs:ux-routing"
	if mcpGuidance {
		markerSummary = "pcs:codegraph-guidance, pcs:persona, pcs:engram-protocol, pcs:orchestrator, pcs:ux-routing"
	}
	recordFile("upsert-markers", layout.claudeMDPath, markerSummary)

	// 2. Write 5 native subagents and ownership manifest.
	var agentNames []string
	for _, agent := range agentsList {
		dst := filepath.Join(layout.agentsDir, agent.RelPath)
		agentNames = append(agentNames, agent.RelPath)
		content := agent.Content
		if layout.scope == ScopeProfile {
			content = []byte(strings.ReplaceAll(string(content), "~/.claude-pcs/skills", resolvedSkillsSlash))
		}
		if !opts.DryRun {
			if err := writeFileCreatingDirs(dst, content, 0o644); err != nil {
				return nil, err
			}
		}
		recordFile("write-agent", dst, agent.RelPath)
	}

	ownershipPath := filepath.Join(layout.agentsDir, AgentOwnershipFileName)
	ownership := AgentOwnership{
		Manager: "pcs",
		Agents:  agentNames,
	}
	if !opts.DryRun {
		if err := writeJSONFile(ownershipPath, ownership); err != nil {
			return nil, err
		}
	}
	recordFile("write-ownership", ownershipPath, fmt.Sprintf("%d native agents", len(agentNames)))

	// 3. Write shared contract, pcs-core, and stack adapter skill files.
	sharedDst := filepath.Join(layout.skillsDir, "_shared", "pcs-phase-common.md")
	if !opts.DryRun {
		if err := writeFileCreatingDirs(sharedDst, sharedContract, 0o644); err != nil {
			return nil, err
		}
	}
	recordFile("write-skill-shared", sharedDst, "_shared/pcs-phase-common.md")

	for _, f := range coreFiles {
		dst := filepath.Join(layout.skillsDir, filepath.FromSlash(f.RelPath))
		if !opts.DryRun {
			if err := writeFileCreatingDirs(dst, f.Content, 0o644); err != nil {
				return nil, err
			}
		}
		recordFile("write-skill-core", dst, f.RelPath)
	}

	for _, f := range adapterFiles {
		dst := filepath.Join(layout.skillsDir, filepath.FromSlash(f.RelPath))
		if !opts.DryRun {
			if err := writeFileCreatingDirs(dst, f.Content, 0o644); err != nil {
				return nil, err
			}
		}
		recordFile("write-skill-adapter", dst, f.RelPath)
	}
	_ = adapterDirName

	// 4. Configure MCP servers (.claude.json) and/or settings.json only when explicitly requested.
	var enabledMCPs []string
	// Start with empty (non-nil) records: a fresh install knows it added nothing yet.
	addedMCPServers := []string{}
	addedPermissions := []string{}
	addedPlugins := []string{}
	addedHook := false
	createdClaudeMD := !claudeMDExisted
	createdClaudeJSON := false
	createdSettingsJSON := false

	if prevState != nil {
		// slices.Clone preserves nil, so a legacy state without a record stays
		// unrecorded unless this run adds entries (a safe, partial record).
		addedMCPServers = slices.Clone(prevState.AddedMCPServers)
		addedPermissions = slices.Clone(prevState.AddedPermissions)
		addedPlugins = slices.Clone(prevState.AddedPlugins)
		addedHook = prevState.AddedHook
		createdClaudeMD = prevState.CreatedClaudeMD
		createdClaudeJSON = prevState.CreatedClaudeJSON
		createdSettingsJSON = prevState.CreatedSettingsJSON
	}

	if opts.WriteMCPConfig {
		enabledMCPs = slices.Clone(mcp.DefaultComponents)
		if !opts.DryRun {
			claudeRes, err := mcp.MergeClaudeJSONWithDelta(layout.claudeJSONPath, enabledMCPs)
			if err != nil {
				return nil, err
			}
			if claudeRes.CreatedFile {
				createdClaudeJSON = true
			}
			for _, s := range claudeRes.AddedServers {
				if !slices.Contains(addedMCPServers, s) {
					addedMCPServers = append(addedMCPServers, s)
				}
			}
		}
		recordFile("merge-mcp", layout.claudeJSONPath, strings.Join(enabledMCPs, ", "))
	}

	if opts.WriteMCPConfig || opts.WriteHook {
		components := slices.Clone(mcp.DefaultComponents)
		if !opts.DryRun {
			settingsRes, err := mcp.MergeSettingsJSONWithDelta(layout.settingsJSONPath, components, layout.targetDir, opts.WriteMCPConfig, opts.WriteHook)
			if err != nil {
				return nil, err
			}
			if settingsRes.CreatedFile {
				createdSettingsJSON = true
			}
			for _, p := range settingsRes.AddedPermissions {
				if !slices.Contains(addedPermissions, p) {
					addedPermissions = append(addedPermissions, p)
				}
			}
			for _, pl := range settingsRes.AddedPlugins {
				if !slices.Contains(addedPlugins, pl) {
					addedPlugins = append(addedPlugins, pl)
				}
			}
			if settingsRes.AddedHook {
				addedHook = true
			}
		}
		recordFile("merge-settings", layout.settingsJSONPath, "settings.json update")
	}

	// 5. In profile scope, create shared-mcp.json, statusLine copy, and cross-platform launchers.
	sharedMCPFile := ""
	addedStatusLine := false
	statusLineSHA256 := ""
	if prevState != nil {
		sharedMCPFile = prevState.SharedMCPFile
		addedStatusLine = prevState.AddedStatusLine
		statusLineSHA256 = prevState.StatusLineSHA256
	}

	if layout.scope == ScopeProfile && layout.sharedMCPPath != "" {
		// engramMissing records that shared-mcp.json got no engram server because
		// no engram binary resolved; a successful --with-plugins provides Engram.
		engramMissing := false
		if !opts.DryRun {
			missing, err := mcp.GenerateSharedMCPFile(layout.sharedMCPPath, layout.mainClaudeJSONPath)
			if err != nil {
				return nil, fmt.Errorf("generate shared-mcp.json: %w", err)
			}
			engramMissing = missing
		}
		sharedMCPFile = layout.sharedMCPPath
		recordFile("write-shared-mcp", layout.sharedMCPPath, "shared MCP configuration (0600)")

		if !opts.NoStatusLine {
			if !opts.DryRun {
				copied, err := mcp.CopyStatusLineIfMissing(layout.settingsJSONPath, layout.mainSettingsPath)
				if err != nil {
					return nil, fmt.Errorf("copy statusLine: %w", err)
				}
				if copied {
					digest, _, err := mcp.StatusLineSHA256(layout.settingsJSONPath)
					if err != nil {
						return nil, fmt.Errorf("record statusLine digest: %w", err)
					}
					addedStatusLine = true
					statusLineSHA256 = digest
					recordFile("copy-statusline", layout.settingsJSONPath, "copied statusLine from main profile")
				}
			} else {
				recordFile("copy-statusline", layout.settingsJSONPath, "copy statusLine from main profile (dry-run)")
			}
		}

		if opts.WithPlugins {
			detail := "claude plugin marketplace add " + EngramMarketplaceSource + "; claude plugin install " + EngramPlugin
			if opts.DryRun {
				detail += " (dry-run)"
			} else if failures := installRequiredPlugins(runtime.GOOS, layout.targetDir); len(failures) > 0 {
				detail += " (failed; see warnings)"
				report.Warnings = append(report.Warnings, failures...)
			} else {
				engramMissing = false
			}
			report.Actions = append(report.Actions, Action{Operation: "install-plugins", Path: layout.targetDir, Detail: detail})
		}
		if engramMissing {
			report.Warnings = append(report.Warnings, fmt.Sprintf(
				"no engram binary found and the main profile defines no engram MCP server, so %s has no engram server and pcs agents cannot use Engram memory; install engram (%s) and run 'pcs sync --scope profile', or run 'pcs sync --scope profile --with-plugins'",
				layout.sharedMCPPath, EngramProjectURL))
		}

		scripts := renderLaunchers(layout.targetDir, layout.sharedMCPPath, launcherArgList)

		launcherDirs := []string{filepath.Dir(layout.launcherPath)}
		if layout.userBinDir != "" && filepath.Clean(layout.userBinDir) != filepath.Clean(filepath.Dir(layout.launcherPath)) {
			launcherDirs = append(launcherDirs, layout.userBinDir)
		}

		for _, lDir := range launcherDirs {
			shFile := filepath.Join(lDir, "claude-pcs")
			cmdFile := filepath.Join(lDir, "claude-pcs.cmd")
			ps1File := filepath.Join(lDir, "claude-pcs.ps1")
			if !opts.DryRun {
				if err := writeFileCreatingDirs(shFile, []byte(scripts.posix), 0o755); err != nil {
					return nil, err
				}
				if err := writeFileCreatingDirs(cmdFile, []byte(scripts.cmd), 0o644); err != nil {
					return nil, err
				}
				if err := writeFileCreatingDirs(ps1File, []byte(scripts.powerShell), 0o644); err != nil {
					return nil, err
				}
			}
			recordFile("write-launcher", shFile, "POSIX sh profile launcher")
			recordFile("write-launcher", cmdFile, "Windows cmd profile launcher")
			recordFile("write-launcher", ps1File, "PowerShell profile launcher")
		}

		report.LauncherDir = layout.userBinDir
		report.LauncherOnPath = isDirOnPath(layout.userBinDir)

		report.LauncherCmd = launcherCommand(runtime.GOOS, layout.targetDir, layout.sharedMCPPath)
		report.NextSteps = append(report.NextSteps,
			profileNextSteps(runtime.GOOS, layout.targetDir, layout.userBinDir, report.LauncherOnPath, opts.WithPlugins)...)
	}

	// 6. In workspace scope, refresh <workspace>/.atl/pcs-skill-registry.md immediately.
	if layout.scope == ScopeWorkspace && layout.workspaceRoot != "" {
		regPath := filepath.Join(layout.workspaceRoot, ".atl", registry.RegistryFileName)
		if !opts.DryRun {
			if _, err := registry.Refresh(layout.workspaceRoot, []string{layout.skillsDir}); err != nil {
				return nil, err
			}
		}
		recordFile("refresh-skill-registry", regPath, ".atl/"+registry.RegistryFileName)
	}

	// 7. Persist .pcs-state.json.
	state := State{
		Version:             Version,
		InstalledAt:         time.Now().UTC().Format(time.RFC3339),
		Scope:               layout.scope,
		TargetDir:           layout.targetDir,
		Adapter:             opts.Adapter,
		Persona:             opts.Persona,
		MCPGuidance:         mcpGuidance,
		WriteMCPConfig:      opts.WriteMCPConfig,
		WriteHook:           opts.WriteHook,
		WithMCP:             opts.WithMCP,
		EnabledMCPs:         enabledMCPs,
		AddedMCPServers:     addedMCPServers,
		AddedPermissions:    addedPermissions,
		AddedPlugins:        addedPlugins,
		AddedHook:           addedHook,
		CreatedClaudeMD:     createdClaudeMD,
		CreatedClaudeJSON:   createdClaudeJSON,
		CreatedSettingsJSON: createdSettingsJSON,
		LauncherDir:         layout.userBinDir,
		LauncherArgs:        opts.LauncherArgs,
		NoStatusLine:        opts.NoStatusLine,
		WithPlugins:         opts.WithPlugins,
		AddedStatusLine:     addedStatusLine,
		StatusLineSHA256:    statusLineSHA256,
		SharedMCPFile:       sharedMCPFile,
		ManagedFiles:        managedFiles,
	}
	if !opts.DryRun {
		if err := writeJSONFile(statePath, state); err != nil {
			return nil, err
		}
	}
	report.Actions = append(report.Actions, Action{
		Operation: "write-state",
		Path:      statePath,
		Detail:    StateFileName,
	})

	return report, nil
}

// Uninstall cleanly removes pcs-owned markers, pcs-* native agents, and pcs-*
// skills while preserving all user files, custom agents, and third-party files.
func Uninstall(opts Options) (*Report, error) {
	layout, err := resolveLayout(opts)
	if err != nil {
		return nil, err
	}

	statePath := filepath.Join(layout.targetDir, StateFileName)
	savedState, _ := readState(statePath)

	// Refuse before touching anything when --remove-mcp cannot be proven safe.
	if opts.RemoveMCP && !savedState.hasMCPDeltaRecord() && !opts.Force {
		return nil, fmt.Errorf("%w in %s: refusing --remove-mcp because pcs cannot prove which entries it added; re-run with --force to remove the default engram, context7, and codegraph servers, their permissions, and the engram@engram plugin even if they predate pcs", ErrNoMCPRecord, statePath)
	}

	report := &Report{
		Operation: "uninstall",
		Scope:     layout.scope,
		TargetDir: layout.targetDir,
		DryRun:    opts.DryRun,
	}

	// 1. Remove all 5 pcs:* markers from CLAUDE.md while preserving any user or third-party content byte-for-byte.
	claudeMDPerm := os.FileMode(0o644)
	if info, statErr := os.Stat(layout.claudeMDPath); statErr == nil {
		claudeMDPerm = info.Mode().Perm()
	}
	if b, err := os.ReadFile(layout.claudeMDPath); err == nil {
		content := string(b)
		updated := content
		anyRemoved := false
		for _, m := range assets.AllPCSMarkers {
			var removed bool
			updated, removed = marker.RemoveBlock(updated, m)
			if removed {
				anyRemoved = true
			}
		}
		if anyRemoved {
			if !opts.DryRun {
				if savedState != nil && savedState.CreatedClaudeMD && strings.TrimSpace(updated) == "" {
					_ = os.Remove(layout.claudeMDPath)
				} else {
					if err := os.WriteFile(layout.claudeMDPath, []byte(updated), claudeMDPerm); err != nil {
						return nil, fmt.Errorf("update %s: %w", layout.claudeMDPath, err)
					}
				}
			}
			report.Actions = append(report.Actions, Action{
				Operation: "remove-markers",
				Path:      layout.claudeMDPath,
				Detail:    strings.Join(assets.AllPCSMarkers, ", "),
			})
		}
	}

	// 2. Remove pcs-owned native agents listed in .pcs-native-agent-ownership.json (or default pcs-*.md).
	ownershipPath := filepath.Join(layout.agentsDir, AgentOwnershipFileName)
	agentsToRemove := slices.Clone(assets.NativeAgentNames)
	if b, err := os.ReadFile(ownershipPath); err == nil {
		var owned AgentOwnership
		if json.Unmarshal(b, &owned) == nil && len(owned.Agents) > 0 {
			agentsToRemove = owned.Agents
		}
	}
	for _, agentName := range agentsToRemove {
		p := filepath.Join(layout.agentsDir, agentName)
		if _, err := os.Stat(p); err == nil {
			if !opts.DryRun {
				_ = os.Remove(p)
			}
			report.Actions = append(report.Actions, Action{
				Operation: "remove-agent",
				Path:      p,
			})
		}
	}
	if _, err := os.Stat(ownershipPath); err == nil {
		if !opts.DryRun {
			_ = os.Remove(ownershipPath)
		}
		report.Actions = append(report.Actions, Action{
			Operation: "remove-ownership",
			Path:      ownershipPath,
		})
	}

	// 3. Remove pcs-owned skill directories (pcs-core, pcs-adapter-*, and _shared/pcs-phase-common.md).
	sharedFile := filepath.Join(layout.skillsDir, "_shared", "pcs-phase-common.md")
	if _, err := os.Stat(sharedFile); err == nil {
		if !opts.DryRun {
			_ = os.Remove(sharedFile)
			// Remove _shared dir only if empty.
			_ = os.Remove(filepath.Dir(sharedFile))
		}
		report.Actions = append(report.Actions, Action{
			Operation: "remove-skill-shared",
			Path:      sharedFile,
		})
	}

	skillSubdirs := []string{"pcs-core"}
	for _, a := range assets.SupportedAdapters {
		skillSubdirs = append(skillSubdirs, "pcs-adapter-"+a)
	}
	for _, dirName := range skillSubdirs {
		dirPath := filepath.Join(layout.skillsDir, dirName)
		if _, err := os.Stat(dirPath); err == nil {
			if !opts.DryRun {
				if err := os.RemoveAll(dirPath); err != nil {
					return nil, fmt.Errorf("remove skill dir %s: %w", dirPath, err)
				}
			}
			report.Actions = append(report.Actions, Action{
				Operation: "remove-skill-dir",
				Path:      dirPath,
			})
		}
	}

	// 4. Unmerge exactly the pcs-added MCP entries recorded in the saved state (an
	// empty record removes nothing). Defaults are removed only with --remove-mcp
	// --force when no record exists.
	delta := mcp.UnmergeDelta{}
	if savedState != nil {
		delta.CreatedClaudeJSON = savedState.CreatedClaudeJSON
		delta.CreatedSettingsJSON = savedState.CreatedSettingsJSON
		delta.AddedMCPServers = slices.Clone(savedState.AddedMCPServers)
		delta.AddedPermissions = slices.Clone(savedState.AddedPermissions)
		delta.AddedPlugins = slices.Clone(savedState.AddedPlugins)
		delta.AddedHook = savedState.AddedHook
	}
	if opts.RemoveMCP && !savedState.hasMCPDeltaRecord() {
		delta.ForceRemoveAllMCP = true
		report.Warnings = append(report.Warnings,
			"--force: no record of pcs-added MCP entries; removing the default engram, context7, and codegraph servers, their permissions, and the engram@engram plugin even if they predate pcs")
	}
	if !opts.DryRun {
		_ = mcp.UnmergeWithDelta(layout.claudeJSONPath, layout.settingsJSONPath, delta)
	}
	report.Actions = append(report.Actions, Action{
		Operation: "unmerge-mcp-hooks",
		Path:      layout.settingsJSONPath,
	})

	// 5. In workspace scope, remove <workspace>/.atl/pcs-skill-registry.md (and .atl/ if empty).
	if layout.scope == ScopeWorkspace && layout.workspaceRoot != "" {
		regPath := filepath.Join(layout.workspaceRoot, ".atl", registry.RegistryFileName)
		if _, err := os.Stat(regPath); err == nil {
			if !opts.DryRun {
				_ = os.Remove(regPath)
				_ = os.Remove(filepath.Dir(regPath))
			}
			report.Actions = append(report.Actions, Action{
				Operation: "remove-skill-registry",
				Path:      regPath,
			})
		}
	}

	// 6. In profile scope, remove shared-mcp.json, statusLine, and launchers.
	if layout.sharedMCPPath != "" {
		if _, err := os.Stat(layout.sharedMCPPath); err == nil {
			if !opts.DryRun {
				_ = os.Remove(layout.sharedMCPPath)
			}
			report.Actions = append(report.Actions, Action{
				Operation: "remove-shared-mcp",
				Path:      layout.sharedMCPPath,
			})
		}
	}
	if savedState != nil && savedState.AddedStatusLine {
		uninstallStatusLine(layout, savedState, opts.DryRun, report)
	}

	// Remove cross-platform profile launchers if present and clean up bin/ if empty.
	if layout.launcherPath != "" {
		targetDirs := []string{filepath.Dir(layout.launcherPath)}
		if savedState != nil && savedState.LauncherDir != "" {
			if !slices.Contains(targetDirs, savedState.LauncherDir) {
				targetDirs = append(targetDirs, savedState.LauncherDir)
			}
		} else if layout.userBinDir != "" && !slices.Contains(targetDirs, layout.userBinDir) {
			targetDirs = append(targetDirs, layout.userBinDir)
		}

		for _, tDir := range targetDirs {
			for _, name := range []string{"claude-pcs", "claude-pcs.cmd", "claude-pcs.ps1"} {
				lp := filepath.Join(tDir, name)
				if _, err := os.Stat(lp); err == nil {
					if savedState == nil && !isPCSLauncher(lp) {
						// Without a state record, only remove files that carry the pcs launcher signature.
						continue
					}
					if !opts.DryRun {
						_ = os.Remove(lp)
					}
					report.Actions = append(report.Actions, Action{
						Operation: "remove-launcher",
						Path:      lp,
					})
				}
			}
			if !opts.DryRun {
				_ = os.Remove(tDir)
			}
		}
	}

	// 7. Remove .pcs-state.json.
	if _, err := os.Stat(statePath); err == nil {
		if !opts.DryRun {
			_ = os.Remove(statePath)
		}
		report.Actions = append(report.Actions, Action{
			Operation: "remove-state",
			Path:      statePath,
		})
	}

	// 8. Keep every .pre-pcs.bak backup and list it so the user can restore or delete it.
	for _, p := range []string{layout.claudeMDPath, layout.claudeJSONPath, layout.settingsJSONPath} {
		if p == "" {
			continue
		}
		bak := p + mcp.PrePCSBackupSuffix
		if _, err := os.Stat(bak); err == nil {
			report.Actions = append(report.Actions, Action{
				Operation: "keep-backup",
				Path:      bak,
				Detail:    "pre-pcs backup kept; restore or delete it manually",
			})
		}
	}

	return report, nil
}

// uninstallStatusLine removes the statusLine pcs copied into the profile only
// while it still matches what pcs wrote: the recorded digest, or for a legacy
// state without one, the main profile's current statusLine. A changed value is
// kept and reported.
func uninstallStatusLine(layout *resolvedLayout, saved *State, dryRun bool, report *Report) {
	current, present, err := mcp.StatusLineSHA256(layout.settingsJSONPath)
	if err != nil || !present {
		return
	}
	want := saved.StatusLineSHA256
	if want == "" && layout.mainSettingsPath != "" {
		want, _, _ = mcp.StatusLineSHA256(layout.mainSettingsPath)
	}
	if want == "" || current != want {
		report.Actions = append(report.Actions, Action{
			Operation: "keep-statusline",
			Path:      layout.settingsJSONPath,
			Detail:    "statusLine changed since pcs copied it",
		})
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("kept statusLine in %s because it changed since pcs copied it", layout.settingsJSONPath))
		return
	}
	if !dryRun {
		_ = mcp.RemoveStatusLine(layout.settingsJSONPath)
	}
	report.Actions = append(report.Actions, Action{
		Operation: "remove-statusline",
		Path:      layout.settingsJSONPath,
	})
}

// Status inspects the target directory and reports installed markers, agents,
// skills, and MCP server wiring.
func Status(opts Options) (*StatusReport, error) {
	layout, err := resolveLayout(opts)
	if err != nil {
		return nil, err
	}

	rep := &StatusReport{
		Scope:              layout.scope,
		TargetDir:          layout.targetDir,
		ClaudeMDPath:       layout.claudeMDPath,
		ClaudeJSONPath:     layout.claudeJSONPath,
		SettingsJSONPath:   layout.settingsJSONPath,
		SharedMCPPath:      layout.sharedMCPPath,
		MainClaudeJSONPath: layout.mainClaudeJSONPath,
		Markers: map[string]bool{
			assets.MarkerCodeGraphGuidance: false,
			assets.MarkerPersona:           false,
			assets.MarkerEngramProtocol:    false,
			assets.MarkerOrchestrator:      false,
			assets.MarkerUXRouting:         false,
		},
		Agents:     make(map[string]bool),
		Skills:     make(map[string]bool),
		MCPServers: make(map[string]bool),
	}

	statePath := filepath.Join(layout.targetDir, StateFileName)
	if s, err := readState(statePath); err == nil && s != nil {
		rep.State = s
	}

	if b, err := os.ReadFile(layout.claudeMDPath); err == nil {
		content := string(b)
		for _, m := range assets.AllPCSMarkers {
			rep.Markers[m] = marker.HasBlock(content, m)
		}
	}

	allAgentsPresent := true
	for _, agentName := range assets.NativeAgentNames {
		_, err := os.Stat(filepath.Join(layout.agentsDir, agentName))
		present := err == nil
		rep.Agents[agentName] = present
		if !present {
			allAgentsPresent = false
		}
	}

	skillChecks := map[string]string{
		"_shared/pcs-phase-common.md": filepath.Join(layout.skillsDir, "_shared", "pcs-phase-common.md"),
		"pcs-core":                    filepath.Join(layout.skillsDir, "pcs-core", "SKILL.md"),
		"pcs-adapter-react-shadcn":    filepath.Join(layout.skillsDir, "pcs-adapter-react-shadcn", "SKILL.md"),
	}
	allSkillsPresent := true
	for k, p := range skillChecks {
		_, err := os.Stat(p)
		present := err == nil
		rep.Skills[k] = present
		if !present {
			allSkillsPresent = false
		}
	}

	if servers, err := mcp.InspectConfiguredServers(layout.claudeJSONPath); err == nil {
		rep.MCPServers = servers
	}

	rep.Installed = rep.Markers[assets.MarkerOrchestrator] &&
		rep.Markers[assets.MarkerUXRouting] &&
		allAgentsPresent &&
		allSkillsPresent

	return rep, nil
}

// isPCSLauncher reports whether the file at path looks like a launcher generated
// by pcs (it scopes CLAUDE_CONFIG_DIR and passes the profile shared-mcp.json).
func isPCSLauncher(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(b)
	return strings.Contains(content, "CLAUDE_CONFIG_DIR") && strings.Contains(content, "shared-mcp.json")
}

func readState(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func writeFileCreatingDirs(path string, content []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, content, perm); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	return nil
}

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return writeFileCreatingDirs(path, b, 0o644)
}
