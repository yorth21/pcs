package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// Options configures an Install, Sync, Uninstall, or Status operation.
type Options struct {
	Scope      string
	TargetDir  string
	HomeDir    string
	Adapter    string
	Persona    string
	WithMCP    bool
	DryRun     bool
	RemoveMCP  bool
}

// State persists the installed pcs configuration in <target-dir>/.pcs-state.json.
type State struct {
	Version      string   `json:"version"`
	InstalledAt  string   `json:"installed_at"`
	Scope        string   `json:"scope"`
	TargetDir    string   `json:"target_dir"`
	Adapter      string   `json:"adapter"`
	Persona      string   `json:"persona"`
	WithMCP      bool     `json:"with_mcp"`
	EnabledMCPs  []string `json:"enabled_mcps"`
	ManagedFiles []string `json:"managed_files"`
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
	Operation   string   `json:"operation"`
	Scope       string   `json:"scope"`
	TargetDir   string   `json:"target_dir"`
	Adapter     string   `json:"adapter"`
	Persona     string   `json:"persona"`
	WithMCP     bool     `json:"with_mcp"`
	DryRun      bool     `json:"dry_run"`
	LauncherCmd string   `json:"launcher_cmd,omitempty"`
	Actions     []Action `json:"actions"`
}

// StatusReport summarizes the health and installed state of a target directory.
type StatusReport struct {
	Installed        bool            `json:"installed"`
	Scope            string          `json:"scope"`
	TargetDir        string          `json:"target_dir"`
	ClaudeMDPath     string          `json:"claude_md_path"`
	ClaudeJSONPath   string          `json:"claude_json_path"`
	SettingsJSONPath string          `json:"settings_json_path"`
	State            *State          `json:"state,omitempty"`
	Markers          map[string]bool `json:"markers"`
	Agents           map[string]bool `json:"agents"`
	Skills           map[string]bool `json:"skills"`
	MCPServers       map[string]bool `json:"mcp_servers"`
}

type resolvedLayout struct {
	scope            string
	targetDir        string
	claudeMDPath     string
	claudeJSONPath   string
	settingsJSONPath string
	agentsDir        string
	skillsDir        string
	launcherPath     string
	workspaceRoot    string
}

func resolveLayout(opts Options) (*resolvedLayout, error) {
	scope := strings.ToLower(strings.TrimSpace(opts.Scope))
	if scope == "" {
		scope = ScopeProfile
	}
	if scope != ScopeProfile && scope != ScopeGlobal && scope != ScopeWorkspace {
		return nil, fmt.Errorf("invalid scope %q (supported: profile, global, workspace)", opts.Scope)
	}

	homeDir := strings.TrimSpace(opts.HomeDir)
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
	}

	targetDir := strings.TrimSpace(opts.TargetDir)
	if targetDir == "" {
		switch scope {
		case ScopeProfile:
			targetDir = filepath.Join(homeDir, ".claude-pcs")
		case ScopeGlobal:
			targetDir = filepath.Join(homeDir, ".claude")
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
		layout.claudeMDPath = filepath.Join(absTarget, "CLAUDE.md")
		layout.claudeJSONPath = filepath.Join(absTarget, ".claude.json")
		layout.launcherPath = filepath.Join(absTarget, "bin", "claude-pcs")
	case ScopeGlobal:
		layout.claudeMDPath = filepath.Join(absTarget, "CLAUDE.md")
		// In global mode, .claude.json lives in the parent of .claude (e.g. ~/.claude.json).
		layout.claudeJSONPath = filepath.Join(filepath.Dir(absTarget), ".claude.json")
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

// Install deploys the pcs orchestrator blocks, native subagents, core & adapter
// skills, and optional MCP configuration into the target scope.
func Install(opts Options) (*Report, error) {
	return applyInstall(opts, "install")
}

// Sync re-reads .pcs-state.json (when present) and refreshes all embedded
// brain files, CLAUDE.md markers, MCP wiring, and skill registries.
func Sync(opts Options) (*Report, error) {
	layout, err := resolveLayout(opts)
	if err != nil {
		return nil, err
	}
	statePath := filepath.Join(layout.targetDir, StateFileName)
	if saved, err := readState(statePath); err == nil && saved != nil {
		if opts.Scope == "" {
			opts.Scope = saved.Scope
		}
		if opts.Adapter == "" {
			opts.Adapter = saved.Adapter
		}
		if opts.Persona == "" {
			opts.Persona = saved.Persona
		}
		opts.WithMCP = saved.WithMCP
	}
	return applyInstall(opts, "sync")
}

func applyInstall(opts Options, opName string) (*Report, error) {
	if strings.TrimSpace(opts.Adapter) == "" {
		opts.Adapter = DefaultAdapter
	}
	if strings.TrimSpace(opts.Persona) == "" {
		opts.Persona = DefaultPersona
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

	layout, err := resolveLayout(opts)
	if err != nil {
		return nil, err
	}

	report := &Report{
		Operation: opName,
		Scope:     layout.scope,
		TargetDir: layout.targetDir,
		Adapter:   opts.Adapter,
		Persona:   opts.Persona,
		WithMCP:   opts.WithMCP,
		DryRun:    opts.DryRun,
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

	// 1. Compose CLAUDE.md with base infrastructure markers (when absent) and pcs markers.
	existingClaudeMD := ""
	if b, readErr := os.ReadFile(layout.claudeMDPath); readErr == nil {
		existingClaudeMD = string(b)
	}
	updatedClaudeMD := existingClaudeMD
	if opts.WithMCP && !marker.HasBlock(updatedClaudeMD, assets.MarkerCodeGraphGuidance) {
		updatedClaudeMD = marker.UpsertBlock(updatedClaudeMD, assets.MarkerCodeGraphGuidance, assets.CodeGraphGuidanceBlock())
	}
	if !marker.HasBlock(updatedClaudeMD, assets.MarkerPersona) {
		updatedClaudeMD = marker.UpsertBlock(updatedClaudeMD, assets.MarkerPersona, personaBlock)
	}
	if opts.WithMCP && !marker.HasBlock(updatedClaudeMD, assets.MarkerEngramProtocol) {
		updatedClaudeMD = marker.UpsertBlock(updatedClaudeMD, assets.MarkerEngramProtocol, assets.EngramProtocolBlock())
	}
	updatedClaudeMD = marker.UpsertBlock(updatedClaudeMD, assets.MarkerOrchestrator, orchBlock)
	updatedClaudeMD = marker.UpsertBlock(updatedClaudeMD, assets.MarkerUXRouting, routingBlock)

	if !opts.DryRun {
		if err := writeFileCreatingDirs(layout.claudeMDPath, []byte(updatedClaudeMD), 0o644); err != nil {
			return nil, err
		}
	}
	recordFile("upsert-markers", layout.claudeMDPath, "pcs:orchestrator, pcs:ux-routing")

	// 2. Write 5 native subagents and ownership manifest.
	var agentNames []string
	for _, agent := range agentsList {
		dst := filepath.Join(layout.agentsDir, agent.RelPath)
		agentNames = append(agentNames, agent.RelPath)
		if !opts.DryRun {
			if err := writeFileCreatingDirs(dst, agent.Content, 0o644); err != nil {
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

	// 4. Configure MCP servers (engram, context7, codegraph) and settings.json when WithMCP is true.
	var enabledMCPs []string
	if opts.WithMCP {
		enabledMCPs = slices.Clone(mcp.DefaultComponents)
		if !opts.DryRun {
			if err := mcp.MergeClaudeJSON(layout.claudeJSONPath, enabledMCPs); err != nil {
				return nil, err
			}
			if err := mcp.MergeSettingsJSON(layout.settingsJSONPath, enabledMCPs, layout.targetDir); err != nil {
				return nil, err
			}
		}
		recordFile("merge-mcp", layout.claudeJSONPath, strings.Join(enabledMCPs, ", "))
		recordFile("merge-settings", layout.settingsJSONPath, "permissions.allow, enabledPlugins, UserPromptSubmit hook")
	}

	// 5. In profile scope, create the executable bin/claude-pcs launcher helper.
	if layout.scope == ScopeProfile && layout.launcherPath != "" {
		launcherScript := fmt.Sprintf("#!/usr/bin/env sh\nexport CLAUDE_CONFIG_DIR=%q\nexec claude \"$@\"\n", layout.targetDir)
		if !opts.DryRun {
			if err := writeFileCreatingDirs(layout.launcherPath, []byte(launcherScript), 0o755); err != nil {
				return nil, err
			}
		}
		report.LauncherCmd = fmt.Sprintf("CLAUDE_CONFIG_DIR=%q claude", layout.targetDir)
		recordFile("write-launcher", layout.launcherPath, "executable profile wrapper")
	}

	// 6. In workspace scope, refresh <workspace>/.atl/skill-registry.md immediately.
	if layout.scope == ScopeWorkspace && layout.workspaceRoot != "" {
		regPath := filepath.Join(layout.workspaceRoot, ".atl", "skill-registry.md")
		if !opts.DryRun {
			if _, err := registry.Refresh(layout.workspaceRoot, []string{layout.skillsDir}); err != nil {
				return nil, err
			}
		}
		recordFile("refresh-skill-registry", regPath, ".atl/skill-registry.md")
	}

	// 7. Persist .pcs-state.json.
	statePath := filepath.Join(layout.targetDir, StateFileName)
	state := State{
		Version:      Version,
		InstalledAt:  time.Now().UTC().Format(time.RFC3339),
		Scope:        layout.scope,
		TargetDir:    layout.targetDir,
		Adapter:      opts.Adapter,
		Persona:      opts.Persona,
		WithMCP:      opts.WithMCP,
		EnabledMCPs:  enabledMCPs,
		ManagedFiles: managedFiles,
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
// skills while preserving all user files, custom agents, and gentle-ai files.
func Uninstall(opts Options) (*Report, error) {
	layout, err := resolveLayout(opts)
	if err != nil {
		return nil, err
	}

	report := &Report{
		Operation: "uninstall",
		Scope:     layout.scope,
		TargetDir: layout.targetDir,
		DryRun:    opts.DryRun,
	}

	// 1. Remove pcs:orchestrator and pcs:ux-routing markers from CLAUDE.md (never gentle-ai markers!).
	if b, err := os.ReadFile(layout.claudeMDPath); err == nil {
		content := string(b)
		updated, r1 := marker.RemoveBlock(content, assets.MarkerOrchestrator)
		updated, r2 := marker.RemoveBlock(updated, assets.MarkerUXRouting)
		if r1 || r2 {
			if !opts.DryRun {
				if err := os.WriteFile(layout.claudeMDPath, []byte(updated), 0o644); err != nil {
					return nil, fmt.Errorf("update %s: %w", layout.claudeMDPath, err)
				}
			}
			report.Actions = append(report.Actions, Action{
				Operation: "remove-markers",
				Path:      layout.claudeMDPath,
				Detail:    "pcs:orchestrator, pcs:ux-routing",
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

	// 4. Unmerge pcs hooks (and MCP servers if RemoveMCP is true or in isolated profile mode).
	onlyPCSOwned := !opts.RemoveMCP && layout.scope != ScopeProfile
	if !opts.DryRun {
		_ = mcp.UnmergeMCP(layout.claudeJSONPath, layout.settingsJSONPath, onlyPCSOwned)
	}
	report.Actions = append(report.Actions, Action{
		Operation: "unmerge-mcp-hooks",
		Path:      layout.settingsJSONPath,
	})

	// 5. Remove profile launcher if present.
	if layout.launcherPath != "" {
		if _, err := os.Stat(layout.launcherPath); err == nil {
			if !opts.DryRun {
				_ = os.Remove(layout.launcherPath)
			}
			report.Actions = append(report.Actions, Action{
				Operation: "remove-launcher",
				Path:      layout.launcherPath,
			})
		}
	}

	// 6. Remove .pcs-state.json.
	statePath := filepath.Join(layout.targetDir, StateFileName)
	if _, err := os.Stat(statePath); err == nil {
		if !opts.DryRun {
			_ = os.Remove(statePath)
		}
		report.Actions = append(report.Actions, Action{
			Operation: "remove-state",
			Path:      statePath,
		})
	}

	return report, nil
}

// Status inspects the target directory and reports installed markers, agents,
// skills, and MCP server wiring.
func Status(opts Options) (*StatusReport, error) {
	layout, err := resolveLayout(opts)
	if err != nil {
		return nil, err
	}

	rep := &StatusReport{
		Scope:            layout.scope,
		TargetDir:        layout.targetDir,
		ClaudeMDPath:     layout.claudeMDPath,
		ClaudeJSONPath:   layout.claudeJSONPath,
		SettingsJSONPath: layout.settingsJSONPath,
		Markers: map[string]bool{
			assets.MarkerOrchestrator: false,
			assets.MarkerUXRouting:    false,
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
		rep.Markers[assets.MarkerOrchestrator] = marker.HasBlock(content, assets.MarkerOrchestrator)
		rep.Markers[assets.MarkerUXRouting] = marker.HasBlock(content, assets.MarkerUXRouting)
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
