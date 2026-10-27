package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
)

const (
	ServerEngram    = "engram"
	ServerContext7  = "context7"
	ServerCodeGraph = "codegraph"

	PrePCSBackupSuffix = ".pre-pcs.bak"
)

// DefaultComponents lists the three foundational MCP servers wired by pcs.
var DefaultComponents = []string{
	ServerEngram,
	ServerContext7,
	ServerCodeGraph,
}

// LookPathFunc allows tests to stub binary discovery.
var LookPathFunc = exec.LookPath

// DefaultPermissionAllows lists the MCP tool permission patterns added to settings.json.
var DefaultPermissionAllows = []string{
	"mcp__engram__*",
	"mcp__plugin_engram_engram__*",
	"mcp__codegraph__*",
	"mcp__context7__*",
}

// ClaudeMergeResult records the exact delta applied by MergeClaudeJSONWithDelta.
type ClaudeMergeResult struct {
	CreatedFile  bool     `json:"created_file"`
	AddedServers []string `json:"added_servers"`
}

// SettingsMergeResult records the exact delta applied by MergeSettingsJSONWithDelta.
type SettingsMergeResult struct {
	CreatedFile bool `json:"created_file"`
	// CreatedKeys lists the containers this merge created in an existing file
	// ("permissions", "permissions.allow", "enabledPlugins"), so an uninstall
	// can prune them once empty.
	CreatedKeys      []string `json:"created_keys,omitempty"`
	AddedPermissions []string `json:"added_permissions"`
	AddedPlugins     []string `json:"added_plugins"`
	AddedHook        bool     `json:"added_hook"`
}

// UnmergeDelta specifies which keys were added by pcs so Uninstall removes only pcs-added state.
type UnmergeDelta struct {
	CreatedClaudeJSON   bool
	CreatedSettingsJSON bool
	// CreatedSettingsKeys lists the settings.json containers pcs created (see
	// SettingsMergeResult.CreatedKeys); each is pruned when it ends up empty.
	CreatedSettingsKeys []string
	AddedMCPServers     []string
	AddedPermissions    []string
	AddedPlugins        []string
	AddedHook           bool
	// FlowHookBinary is the recorded pcs binary of the flow hooks, so hooks
	// running a renamed binary are still removed.
	FlowHookBinary string
	// ForceRemoveAllMCP additionally removes the default pcs servers, permissions,
	// and engram@engram plugin even when they were not recorded as pcs-added. It
	// must only be set on explicit user request (uninstall --remove-mcp --force).
	ForceRemoveAllMCP bool
}

// EngramFallbackPaths lists absolute engram locations checked when the PATH
// lookup fails. Tests replace it to make engram resolution deterministic.
var EngramFallbackPaths = []string{"/opt/homebrew/bin/engram"}

// LookupEngramBinary returns the engram executable path, checking PATH via
// LookPathFunc first and then EngramFallbackPaths, and whether one was found.
func LookupEngramBinary() (string, bool) {
	if p, err := LookPathFunc("engram"); err == nil && p != "" {
		return p, true
	}
	for _, p := range EngramFallbackPaths {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// ResolveEngramBinary returns the preferred engram executable path (see
// LookupEngramBinary), or the bare command name when none is found.
func ResolveEngramBinary() string {
	if p, ok := LookupEngramBinary(); ok {
		return p
	}
	return "engram"
}

// ServerDefinition returns the canonical MCP server configuration map for a component.
func ServerDefinition(component string) (map[string]any, error) {
	switch strings.ToLower(strings.TrimSpace(component)) {
	case ServerEngram:
		return map[string]any{
			"command": ResolveEngramBinary(),
			"args":    []any{"mcp", "--tools=agent"},
			"type":    "stdio",
			"env":     map[string]any{},
		}, nil
	case ServerContext7:
		return map[string]any{
			"command": "npx",
			"args": []any{
				"-y",
				"--package=@upstash/context7-mcp@2.2.5",
				"--",
				"context7-mcp",
			},
		}, nil
	case ServerCodeGraph:
		return map[string]any{
			"command": "codegraph",
			"args":    []any{"serve", "--mcp"},
			"type":    "stdio",
		}, nil
	default:
		return nil, fmt.Errorf("unknown MCP component %q", component)
	}
}

// MergeClaudeJSON merges missing MCP server entries into path.
func MergeClaudeJSON(path string, components []string) error {
	_, err := MergeClaudeJSONWithDelta(path, components, true)
	return err
}

// MergeClaudeJSONWithDelta reads the existing JSON file at path (or initializes
// an empty object if absent), preserves all existing user keys and pre-existing
// MCP servers (never overwriting an existing server entry such as engram),
// adds only missing MCP server entries, creates a one-time .pre-pcs.bak backup
// when the file already existed and backup is true (false when pcs created the
// file, whose content is then pcs's own), and writes formatted JSON atomically
// while preserving file permissions (defaulting to 0600 for .claude.json).
func MergeClaudeJSONWithDelta(path string, components []string, backup bool) (*ClaudeMergeResult, error) {
	if len(components) == 0 {
		components = DefaultComponents
	}

	root, existed, rawExisting, perm, err := readJSONObjectWithPerm(path, 0o600)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	res := &ClaudeMergeResult{
		CreatedFile: !existed,
	}

	mcpServers := ensureMap(root, "mcpServers")
	for _, comp := range components {
		if _, alreadyPresent := mcpServers[comp]; alreadyPresent {
			continue
		}
		def, defErr := ServerDefinition(comp)
		if defErr != nil {
			return nil, defErr
		}
		mcpServers[comp] = def
		res.AddedServers = append(res.AddedServers, comp)
	}

	if existed && backup {
		if err := WritePrePCSBackupOnce(path, rawExisting, perm); err != nil {
			return nil, err
		}
	}
	if err := writeJSONAtomicWithPerm(path, root, perm); err != nil {
		return nil, err
	}
	return res, nil
}

// MergeSettingsJSON merges MCP tool permissions, enabledPlugins, and the
// UserPromptSubmit skill-registry refresh hook into settings.json.
func MergeSettingsJSON(path string, components []string, targetDir string) error {
	_, err := MergeSettingsJSONWithDelta(path, components, targetDir, true, true, true)
	return err
}

// MergeSettingsJSONWithDelta merges requested MCP permissions/plugins (when
// writeMCPConfig is true) and/or the UserPromptSubmit skill-registry hook (when
// writeHook is true) into settings.json, recording only newly added items and
// the containers it created. An existing file gets a one-time .pre-pcs.bak
// backup only when backup is true (false when pcs created the file).
func MergeSettingsJSONWithDelta(path string, components []string, targetDir string, writeMCPConfig, writeHook, backup bool) (*SettingsMergeResult, error) {
	if len(components) == 0 {
		components = DefaultComponents
	}

	root, existed, rawExisting, perm, err := readJSONObjectWithPerm(path, 0o644)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	res := &SettingsMergeResult{
		CreatedFile: !existed,
	}

	if writeMCPConfig {
		// 1. Merge permissions.allow without duplicates.
		if _, ok := root["permissions"].(map[string]any); !ok {
			res.CreatedKeys = append(res.CreatedKeys, "permissions")
		}
		permissions := ensureMap(root, "permissions")
		if _, ok := permissions["allow"]; !ok {
			res.CreatedKeys = append(res.CreatedKeys, "permissions.allow")
		}
		allowList := ensureStringSlice(permissions, "allow")
		for _, p := range permissionsForComponents(components) {
			if !slices.Contains(allowList, p) {
				allowList = append(allowList, p)
				res.AddedPermissions = append(res.AddedPermissions, p)
			}
		}
		permissions["allow"] = allowList

		// 2. Configure enabledPlugins (engram@engram: true when engram is enabled and not already set).
		if slices.Contains(components, ServerEngram) {
			if _, ok := root["enabledPlugins"].(map[string]any); !ok {
				res.CreatedKeys = append(res.CreatedKeys, "enabledPlugins")
			}
			enabledPlugins := ensureMap(root, "enabledPlugins")
			if _, exists := enabledPlugins["engram@engram"]; !exists {
				enabledPlugins["engram@engram"] = true
				res.AddedPlugins = append(res.AddedPlugins, "engram@engram")
			}
		}
	}

	if writeHook {
		// 3. Ensure UserPromptSubmit hook for pcs skill-registry refresh.
		hooks := ensureMap(root, "hooks")
		res.AddedHook = ensureSkillRegistryHook(hooks, targetDir)
	}

	if existed && backup {
		if err := WritePrePCSBackupOnce(path, rawExisting, perm); err != nil {
			return nil, err
		}
	}
	if err := writeJSONAtomicWithPerm(path, root, perm); err != nil {
		return nil, err
	}
	return res, nil
}

// UnmergeMCP removes pcs-managed MCP servers and hooks when requested.
func UnmergeMCP(claudeJSONPath, settingsJSONPath string, onlyPCSOwned bool) error {
	delta := UnmergeDelta{
		AddedHook:         true,
		ForceRemoveAllMCP: !onlyPCSOwned,
	}
	if !onlyPCSOwned {
		delta.AddedMCPServers = slices.Clone(DefaultComponents)
		delta.AddedPermissions = slices.Clone(DefaultPermissionAllows)
		delta.AddedPlugins = []string{"engram@engram"}
	}
	return UnmergeWithDelta(claudeJSONPath, settingsJSONPath, delta)
}

// UnmergeWithDelta removes only the MCP servers, permissions, plugins, and hooks
// recorded as added by pcs; an empty record removes nothing. When
// ForceRemoveAllMCP is true, the default pcs servers, permissions, and plugin
// are removed as well. Files are rewritten only when something changed.
func UnmergeWithDelta(claudeJSONPath, settingsJSONPath string, delta UnmergeDelta) error {
	serversToRemove := slices.Clone(delta.AddedMCPServers)
	permsToRemove := slices.Clone(delta.AddedPermissions)
	pluginsToRemove := slices.Clone(delta.AddedPlugins)
	if delta.ForceRemoveAllMCP {
		serversToRemove = appendMissing(serversToRemove, DefaultComponents...)
		permsToRemove = appendMissing(permsToRemove, DefaultPermissionAllows...)
		pluginsToRemove = appendMissing(pluginsToRemove, "engram@engram")
	}

	if strings.TrimSpace(claudeJSONPath) != "" && (len(serversToRemove) > 0 || delta.CreatedClaudeJSON) {
		root, existed, _, perm, err := readJSONObjectWithPerm(claudeJSONPath, 0o600)
		if err != nil {
			return err
		}
		if existed {
			changed := false
			if mcpServers, ok := root["mcpServers"].(map[string]any); ok {
				for _, comp := range serversToRemove {
					if _, present := mcpServers[comp]; present {
						delete(mcpServers, comp)
						changed = true
					}
				}
				if len(mcpServers) == 0 && delta.CreatedClaudeJSON {
					delete(root, "mcpServers")
					changed = true
				}
			}
			if delta.CreatedClaudeJSON && len(root) == 0 {
				// Any .pre-pcs.bak is kept: uninstall never deletes backups.
				_ = os.Remove(claudeJSONPath)
			} else if changed {
				if err := writeJSONAtomicWithPerm(claudeJSONPath, root, perm); err != nil {
					return err
				}
			}
		}
	}

	if strings.TrimSpace(settingsJSONPath) != "" {
		root, existed, _, perm, err := readJSONObjectWithPerm(settingsJSONPath, 0o644)
		if err != nil {
			return err
		}
		if existed {
			changed := removePCSHooks(root, []string{delta.FlowHookBinary})
			if len(permsToRemove) > 0 {
				if perms, ok := root["permissions"].(map[string]any); ok {
					existing := ensureStringSlice(perms, "allow")
					filtered := make([]string, 0, len(existing))
					for _, item := range existing {
						if !slices.Contains(permsToRemove, item) {
							filtered = append(filtered, item)
						} else {
							changed = true
						}
					}
					perms["allow"] = filtered
					if len(filtered) == 0 && delta.createdSettingsKey("permissions.allow") {
						delete(perms, "allow")
						changed = true
					}
					if len(perms) == 0 && delta.createdSettingsKey("permissions") {
						delete(root, "permissions")
						changed = true
					}
				}
			}
			if len(pluginsToRemove) > 0 {
				if plugins, ok := root["enabledPlugins"].(map[string]any); ok {
					for _, plug := range pluginsToRemove {
						if _, exists := plugins[plug]; exists {
							delete(plugins, plug)
							changed = true
						}
					}
					if len(plugins) == 0 && delta.createdSettingsKey("enabledPlugins") {
						delete(root, "enabledPlugins")
						changed = true
					}
				}
			}
			if delta.CreatedSettingsJSON && len(root) == 0 {
				// Any .pre-pcs.bak is kept: uninstall never deletes backups.
				_ = os.Remove(settingsJSONPath)
			} else if changed {
				if err := writeJSONAtomicWithPerm(settingsJSONPath, root, perm); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// createdSettingsKey reports whether pcs created the settings.json container
// key: either the whole file or that container was created by pcs.
func (d UnmergeDelta) createdSettingsKey(key string) bool {
	return d.CreatedSettingsJSON || slices.Contains(d.CreatedSettingsKeys, key)
}

func appendMissing(list []string, items ...string) []string {
	for _, item := range items {
		if !slices.Contains(list, item) {
			list = append(list, item)
		}
	}
	return list
}

// InspectConfiguredServers returns which of the 3 foundational MCP servers are
// present in the given .claude.json file.
func InspectConfiguredServers(claudeJSONPath string) (map[string]bool, error) {
	status := map[string]bool{
		ServerEngram:    false,
		ServerContext7:  false,
		ServerCodeGraph: false,
	}
	root, existed, _, _, err := readJSONObjectWithPerm(claudeJSONPath, 0o600)
	if err != nil {
		return status, err
	}
	if !existed {
		return status, nil
	}
	mcpServers, ok := root["mcpServers"].(map[string]any)
	if !ok {
		return status, nil
	}
	for k := range status {
		_, present := mcpServers[k]
		status[k] = present
	}
	return status, nil
}

// InspectSettingsDefaults reports whether settingsJSONPath holds any of the
// default pcs permissions in permissions.allow and whether enabledPlugins has
// an engram@engram entry (with any value). A missing file holds neither.
func InspectSettingsDefaults(settingsJSONPath string) (hasPermissions, hasPlugin bool, err error) {
	root, existed, _, _, err := readJSONObjectWithPerm(settingsJSONPath, 0o644)
	if err != nil || !existed {
		return false, false, err
	}
	if perms, ok := root["permissions"].(map[string]any); ok {
		for _, p := range ensureStringSlice(perms, "allow") {
			if slices.Contains(DefaultPermissionAllows, p) {
				hasPermissions = true
			}
		}
	}
	if plugins, ok := root["enabledPlugins"].(map[string]any); ok {
		_, hasPlugin = plugins["engram@engram"]
	}
	return hasPermissions, hasPlugin, nil
}

func permissionsForComponents(components []string) []string {
	var out []string
	for _, comp := range components {
		switch comp {
		case ServerEngram:
			out = append(out, "mcp__engram__*", "mcp__plugin_engram_engram__*")
		case ServerCodeGraph:
			out = append(out, "mcp__codegraph__*")
		case ServerContext7:
			out = append(out, "mcp__context7__*")
		}
	}
	return out
}

func ensureSkillRegistryHook(hooks map[string]any, targetDir string) bool {
	rawList, _ := hooks["UserPromptSubmit"].([]any)
	for _, item := range rawList {
		if strings.Contains(fmt.Sprint(item), "pcs skill-registry refresh") {
			// Preserve existing pcs skill-registry hook without duplicating.
			return false
		}
	}

	cmd := "pcs skill-registry refresh"
	if strings.TrimSpace(targetDir) != "" {
		skillsPath := strings.ReplaceAll(filepath.ToSlash(filepath.Join(targetDir, "skills")), `\`, "/")
		cmd = fmt.Sprintf("pcs skill-registry refresh --skills-dir \"%s\"", skillsPath)
	}

	entry := map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": cmd,
			},
		},
	}
	hooks["UserPromptSubmit"] = append(rawList, entry)
	return true
}

func removePCSHooks(root map[string]any, flowHookBinaries []string) bool {
	flowChanged := removeFlowHooks(root, flowHookBinaries)
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		return flowChanged
	}
	rawList, ok := hooks["UserPromptSubmit"].([]any)
	if !ok {
		return flowChanged
	}
	changed := flowChanged
	filtered := make([]any, 0, len(rawList))
	for _, item := range rawList {
		text := fmt.Sprint(item)
		if strings.Contains(text, "pcs skill-registry refresh") {
			changed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		delete(hooks, "UserPromptSubmit")
	} else {
		hooks["UserPromptSubmit"] = filtered
	}
	if len(hooks) == 0 {
		delete(root, "hooks")
	}
	return changed
}

func readJSONObjectWithPerm(path string, defaultPerm os.FileMode) (map[string]any, bool, []byte, os.FileMode, error) {
	perm := defaultPerm
	if info, statErr := os.Stat(path); statErr == nil {
		perm = info.Mode().Perm()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), false, nil, perm, nil
		}
		return nil, false, nil, perm, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return make(map[string]any), true, b, perm, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, true, b, perm, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	if obj == nil {
		obj = make(map[string]any)
	}
	return obj, true, b, perm, nil
}

func ensureMap(parent map[string]any, key string) map[string]any {
	if existing, ok := parent[key].(map[string]any); ok && existing != nil {
		return existing
	}
	m := make(map[string]any)
	parent[key] = m
	return m
}

func ensureStringSlice(parent map[string]any, key string) []string {
	raw, ok := parent[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return slices.Clone(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, elem := range v {
			if s, ok := elem.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// WritePrePCSBackupOnce writes `<path>.pre-pcs.bak` only if it does not already exist,
// preserving the original pre-install file content across subsequent sync runs.
func WritePrePCSBackupOnce(path string, content []byte, perm os.FileMode) error {
	backupPath := path + PrePCSBackupSuffix
	if _, err := os.Stat(backupPath); err == nil {
		// Pre-PCS backup already exists; never overwrite it on subsequent syncs.
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	if err := os.WriteFile(backupPath, content, perm); err != nil {
		return fmt.Errorf("write backup %s: %w", backupPath, err)
	}
	return nil
}

func writeJSONAtomicWithPerm(path string, data map[string]any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", path, err)
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON for %s: %w", path, err)
	}
	encoded = append(encoded, '\n')

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, encoded, perm); err != nil {
		return fmt.Errorf("write temp JSON %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp JSON %s -> %s: %w", tmpPath, path, err)
	}
	return nil
}

// ReadMainProfileMCPServers reads the mcpServers map from mainClaudeJSONPath read-only.
// It returns nil, false, nil if the file does not exist or has no non-empty mcpServers map.
func ReadMainProfileMCPServers(mainClaudeJSONPath string) (map[string]any, bool, error) {
	if _, err := os.Stat(mainClaudeJSONPath); os.IsNotExist(err) {
		return nil, false, nil
	}
	root, _, _, _, err := readJSONObjectWithPerm(mainClaudeJSONPath, 0o600)
	if err != nil {
		return nil, false, err
	}
	if serversRaw, ok := root["mcpServers"]; ok {
		if serversMap, ok := serversRaw.(map[string]any); ok && len(serversMap) > 0 {
			return serversMap, true, nil
		}
	}
	return nil, false, nil
}

// GenerateSharedMCPFile writes the shared MCP configuration to outPath with 0o600 permissions.
// If mainClaudeJSONPath contains configured mcpServers, they are reused directly; otherwise,
// default pcs components (engram, context7, codegraph) are generated. The main
// file is only read. Every pcs agent requires Engram, so when the main profile
// defines no engram server (for example because it uses the engram@engram
// plugin), an engram server pointing at the resolved engram binary is added.
// When no engram binary resolves, no engram entry is written and engramMissing
// is true.
func GenerateSharedMCPFile(outPath, mainClaudeJSONPath string) (engramMissing bool, err error) {
	servers, found, err := ReadMainProfileMCPServers(mainClaudeJSONPath)
	if err != nil {
		return false, err
	}
	if !found || len(servers) == 0 {
		servers = make(map[string]any)
		for _, comp := range DefaultComponents {
			if comp == ServerEngram {
				continue
			}
			def, defErr := ServerDefinition(comp)
			if defErr != nil {
				return false, defErr
			}
			servers[comp] = def
		}
	}
	if _, hasEngram := servers[ServerEngram]; !hasEngram {
		if _, ok := LookupEngramBinary(); ok {
			def, defErr := ServerDefinition(ServerEngram)
			if defErr != nil {
				return false, defErr
			}
			servers[ServerEngram] = def
		} else {
			engramMissing = true
		}
	}
	payload := map[string]any{
		"mcpServers": servers,
	}
	return engramMissing, writeJSONAtomicWithPerm(outPath, payload, 0o600)
}

// CopyStatusLineIfMissing copies statusLine from mainSettingsJSONPath into
// targetSettingsJSONPath if targetSettingsJSONPath does not already define one.
// It reports whether a statusLine was copied and whether that created the
// target file. An existing target gets a one-time .pre-pcs.bak backup only when
// backup is true (false when pcs created the file).
func CopyStatusLineIfMissing(targetSettingsJSONPath, mainSettingsJSONPath string, backup bool) (copied, created bool, err error) {
	if _, err := os.Stat(mainSettingsJSONPath); os.IsNotExist(err) {
		return false, false, nil
	}
	mainRoot, _, _, _, err := readJSONObjectWithPerm(mainSettingsJSONPath, 0o644)
	if err != nil {
		return false, false, err
	}
	statusLineVal, ok := mainRoot["statusLine"]
	if !ok || statusLineVal == nil {
		return false, false, nil
	}

	targetRoot, existed, rawExisting, perm, err := readJSONObjectWithPerm(targetSettingsJSONPath, 0o644)
	if err != nil {
		return false, false, err
	}
	if _, alreadyHas := targetRoot["statusLine"]; alreadyHas {
		return false, false, nil
	}
	targetRoot["statusLine"] = statusLineVal
	if existed && backup {
		if err := WritePrePCSBackupOnce(targetSettingsJSONPath, rawExisting, perm); err != nil {
			return false, false, err
		}
	}
	if err := writeJSONAtomicWithPerm(targetSettingsJSONPath, targetRoot, perm); err != nil {
		return false, false, err
	}
	return true, !existed, nil
}

// RemoveStatusLine removes the statusLine key from targetSettingsJSONPath if present.
func RemoveStatusLine(targetSettingsJSONPath string) error {
	if _, err := os.Stat(targetSettingsJSONPath); os.IsNotExist(err) {
		return nil
	}
	targetRoot, _, _, perm, err := readJSONObjectWithPerm(targetSettingsJSONPath, 0o644)
	if err != nil {
		return err
	}
	if _, has := targetRoot["statusLine"]; !has {
		return nil
	}
	delete(targetRoot, "statusLine")
	return writeJSONAtomicWithPerm(targetSettingsJSONPath, targetRoot, perm)
}

// StatusLineSHA256 returns the SHA-256 hex digest of the JSON encoding of the
// statusLine value in settingsJSONPath and whether a statusLine is present.
// encoding/json sorts map keys, so equal values always yield equal digests.
func StatusLineSHA256(settingsJSONPath string) (string, bool, error) {
	if _, err := os.Stat(settingsJSONPath); os.IsNotExist(err) {
		return "", false, nil
	}
	root, _, _, _, err := readJSONObjectWithPerm(settingsJSONPath, 0o644)
	if err != nil {
		return "", false, err
	}
	value, ok := root["statusLine"]
	if !ok {
		return "", false, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", true, fmt.Errorf("encode statusLine in %s: %w", settingsJSONPath, err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), true, nil
}

// PluginEnabled reports whether enabledPlugins[plugin] is true in settingsJSONPath.
// A missing file or key reports false.
func PluginEnabled(settingsJSONPath, plugin string) (bool, error) {
	if _, err := os.Stat(settingsJSONPath); os.IsNotExist(err) {
		return false, nil
	}
	root, _, _, _, err := readJSONObjectWithPerm(settingsJSONPath, 0o644)
	if err != nil {
		return false, err
	}
	plugins, _ := root["enabledPlugins"].(map[string]any)
	enabled, _ := plugins[plugin].(bool)
	return enabled, nil
}

// SharedMCPDrift compares the mcpServers in sharedMCPPath with the main
// profile's mcpServers and returns the sorted names of servers that are
// missing, extra, or different. It returns nil when they match, or when the main
// profile defines no servers (shared-mcp.json then holds the pcs defaults). An
// engram server is not drift when the main profile defines none, because
// GenerateSharedMCPFile adds it.
func SharedMCPDrift(sharedMCPPath, mainClaudeJSONPath string) ([]string, error) {
	mainServers, found, err := ReadMainProfileMCPServers(mainClaudeJSONPath)
	if err != nil || !found {
		return nil, err
	}
	root, _, _, _, err := readJSONObjectWithPerm(sharedMCPPath, 0o600)
	if err != nil {
		return nil, err
	}
	shared, _ := root["mcpServers"].(map[string]any)
	var drift []string
	for name, def := range mainServers {
		if got, ok := shared[name]; !ok || !reflect.DeepEqual(got, def) {
			drift = append(drift, name)
		}
	}
	for name := range shared {
		if _, ok := mainServers[name]; !ok && name != ServerEngram {
			drift = append(drift, name)
		}
	}
	slices.Sort(drift)
	return drift, nil
}

// FlowHookSpec describes one pcs flow hook entry in settings.json.
type FlowHookSpec struct {
	Event   string // Claude Code hook event, e.g. "PreToolUse"
	Matcher string // tool matcher; empty for events without tools
	Arg     string // pcs hook argument, e.g. "pre-tool-use"
}

// FlowHookTimeoutSeconds bounds every pcs flow hook invocation.
const FlowHookTimeoutSeconds = 15

// FlowHookSpecs lists the pcs flow hooks in install order.
var FlowHookSpecs = []FlowHookSpec{
	{Event: "PreToolUse", Matcher: "Read|Grep|Glob|Bash|Edit|Write|MultiEdit|NotebookEdit|Agent|Task", Arg: "pre-tool-use"},
	{Event: "SubagentStop", Arg: "subagent-stop"},
	{Event: "Stop", Arg: "stop"},
	{Event: "UserPromptSubmit", Arg: "user-prompt-submit"},
}

// flowHookCommandPattern identifies a pcs flow hook command: a pcs binary
// (optionally quoted, any path) followed by `hook <event>`.
var flowHookCommandPattern = regexp.MustCompile(`(?i)(^|[\/"\s])pcs(\.exe)?"?\s+hook\s+(pre-tool-use|subagent-stop|stop|user-prompt-submit)\s*$`)

// IsFlowHookCommand reports whether command is a pcs flow hook command: a pcs
// binary followed by `hook <event>`, or the exact command pcs renders for one
// of the recorded binaries (which may be named differently).
func IsFlowHookCommand(command string, binaries ...string) bool {
	command = strings.TrimSpace(command)
	if flowHookCommandPattern.MatchString(command) {
		return true
	}
	for _, bin := range binaries {
		if strings.TrimSpace(bin) == "" {
			continue
		}
		for _, spec := range FlowHookSpecs {
			if command == FlowHookCommand(bin, spec.Arg) {
				return true
			}
		}
	}
	return false
}

// FlowHookCommand renders the hook command for binary and arg in Git Bash
// shell form: a double-quoted forward-slash path followed by `hook <arg>`.
func FlowHookCommand(binary, arg string) string {
	return fmt.Sprintf("\"%s\" hook %s", filepath.ToSlash(binary), arg)
}

// MergeFlowHooks replaces every pcs flow hook in settings.json with fresh
// entries for binary, preserving all other hooks. It writes the file only when
// the result differs, taking a one-time .pre-pcs.bak backup of an existing file
// when backup is true. It reports whether the file changed and whether it was
// created.
//
// previous lists binaries recorded by an earlier install whose hooks are
// replaced as well.
func MergeFlowHooks(path, binary string, backup bool, previous ...string) (changed, created bool, err error) {
	root, existed, rawExisting, perm, err := readJSONObjectWithPerm(path, 0o644)
	if err != nil {
		return false, false, fmt.Errorf("read %s: %w", path, err)
	}
	before, _ := json.Marshal(root)
	removeFlowHooks(root, append([]string{binary}, previous...))
	hooks := ensureMap(root, "hooks")
	for _, spec := range FlowHookSpecs {
		list, _ := hooks[spec.Event].([]any)
		entry := map[string]any{
			"hooks": []any{map[string]any{
				"type":    "command",
				"command": FlowHookCommand(binary, spec.Arg),
				"timeout": FlowHookTimeoutSeconds,
			}},
		}
		if spec.Matcher != "" {
			entry["matcher"] = spec.Matcher
		}
		hooks[spec.Event] = append(list, entry)
	}
	after, _ := json.Marshal(root)
	if existed && string(before) == string(after) {
		return false, false, nil
	}
	if existed && backup {
		if err := WritePrePCSBackupOnce(path, rawExisting, perm); err != nil {
			return false, false, err
		}
	}
	if err := writeJSONAtomicWithPerm(path, root, perm); err != nil {
		return false, false, err
	}
	return true, !existed, nil
}

// RemoveFlowHooks removes every pcs flow hook (see IsFlowHookCommand) from
// settings.json, preserving all other hooks. A missing file is not an error.
func RemoveFlowHooks(path string, binaries ...string) (bool, error) {
	root, existed, _, perm, err := readJSONObjectWithPerm(path, 0o644)
	if err != nil || !existed {
		return false, err
	}
	if !removeFlowHooks(root, binaries) {
		return false, nil
	}
	return true, writeJSONAtomicWithPerm(path, root, perm)
}

// InspectFlowHooks reports, per hook event, whether settings.json holds a pcs
// flow hook, plus the command of each one found.
func InspectFlowHooks(path string, binaries ...string) (map[string]bool, []string, error) {
	present := make(map[string]bool, len(FlowHookSpecs))
	for _, spec := range FlowHookSpecs {
		present[spec.Event] = false
	}
	root, existed, _, _, err := readJSONObjectWithPerm(path, 0o644)
	if err != nil || !existed {
		return present, nil, err
	}
	hooks, _ := root["hooks"].(map[string]any)
	var commands []string
	for _, spec := range FlowHookSpecs {
		list, _ := hooks[spec.Event].([]any)
		for _, group := range list {
			for _, cmd := range groupCommands(group) {
				if IsFlowHookCommand(cmd, binaries...) && strings.HasSuffix(strings.TrimSpace(cmd), " "+spec.Arg) {
					present[spec.Event] = true
					commands = append(commands, cmd)
				}
			}
		}
	}
	return present, commands, nil
}

func groupCommands(group any) []string {
	g, _ := group.(map[string]any)
	inner, _ := g["hooks"].([]any)
	var out []string
	for _, h := range inner {
		if m, ok := h.(map[string]any); ok {
			if c, ok := m["command"].(string); ok {
				out = append(out, c)
			}
		}
	}
	return out
}

// removeFlowHooks strips pcs flow hook commands from every event in root,
// dropping matcher groups and events left empty and an empty hooks map.
func removeFlowHooks(root map[string]any, binaries []string) bool {
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		return false
	}
	changed := false
	for event, rawList := range hooks {
		list, ok := rawList.([]any)
		if !ok {
			continue
		}
		var keptGroups []any
		for _, group := range list {
			g, ok := group.(map[string]any)
			inner, innerOK := g["hooks"].([]any)
			if !ok || !innerOK {
				keptGroups = append(keptGroups, group)
				continue
			}
			var keptInner []any
			for _, h := range inner {
				if m, ok := h.(map[string]any); ok {
					if c, ok := m["command"].(string); ok && IsFlowHookCommand(c, binaries...) {
						changed = true
						continue
					}
				}
				keptInner = append(keptInner, h)
			}
			if len(keptInner) == 0 {
				continue
			}
			g["hooks"] = keptInner
			keptGroups = append(keptGroups, g)
		}
		if len(keptGroups) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = keptGroups
		}
	}
	if len(hooks) == 0 {
		delete(root, "hooks")
	}
	return changed
}
