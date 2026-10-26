package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const (
	ServerEngram    = "engram"
	ServerContext7  = "context7"
	ServerCodeGraph = "codegraph"
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

// ResolveEngramBinary returns the preferred engram executable path.
func ResolveEngramBinary() string {
	if _, err := os.Stat("/opt/homebrew/bin/engram"); err == nil {
		return "/opt/homebrew/bin/engram"
	}
	if p, err := LookPathFunc("engram"); err == nil && p != "" {
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

// MergeClaudeJSON reads the existing JSON file at path (or initializes an empty
// object if absent), preserves all existing user keys and custom MCP servers,
// sets/updates the requested MCP server entries (defaulting to engram, context7,
// and codegraph when components is empty), creates a .bak backup when the file
// already exists, and writes formatted JSON atomically.
func MergeClaudeJSON(path string, components []string) error {
	if len(components) == 0 {
		components = DefaultComponents
	}

	root, existed, rawExisting, err := readJSONObject(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	mcpServers := ensureMap(root, "mcpServers")
	for _, comp := range components {
		def, defErr := ServerDefinition(comp)
		if defErr != nil {
			return defErr
		}
		mcpServers[comp] = def
	}

	if existed {
		if err := writeBackup(path+".bak", rawExisting); err != nil {
			return err
		}
	}
	return writeJSONAtomic(path, root)
}

// MergeSettingsJSON merges MCP tool permissions, enabledPlugins, and the
// UserPromptSubmit skill-registry refresh hook into settings.json while
// preserving all existing user configuration and creating a .bak backup.
func MergeSettingsJSON(path string, components []string, targetDir string) error {
	if len(components) == 0 {
		components = DefaultComponents
	}

	root, existed, rawExisting, err := readJSONObject(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	// 1. Merge permissions.allow without duplicates.
	permissions := ensureMap(root, "permissions")
	allowList := ensureStringSlice(permissions, "allow")
	for _, perm := range permissionsForComponents(components) {
		if !slices.Contains(allowList, perm) {
			allowList = append(allowList, perm)
		}
	}
	permissions["allow"] = allowList

	// 2. Configure enabledPlugins (engram@engram: true when engram is enabled).
	if slices.Contains(components, ServerEngram) {
		enabledPlugins := ensureMap(root, "enabledPlugins")
		enabledPlugins["engram@engram"] = true
	}

	// 3. Ensure UserPromptSubmit hook for pcs skill-registry refresh.
	hooks := ensureMap(root, "hooks")
	ensureSkillRegistryHook(hooks, targetDir)

	if existed {
		if err := writeBackup(path+".bak", rawExisting); err != nil {
			return err
		}
	}
	return writeJSONAtomic(path, root)
}

// UnmergeMCP removes pcs-managed MCP servers and hooks when requested.
// When onlyPCSOwned is true, shared infrastructure servers (engram, context7,
// codegraph) are preserved while pcs skill-registry hooks are removed. When
// onlyPCSOwned is false, the 3 MCP servers and pcs permissions are also removed.
func UnmergeMCP(claudeJSONPath, settingsJSONPath string, onlyPCSOwned bool) error {
	if !onlyPCSOwned && strings.TrimSpace(claudeJSONPath) != "" {
		root, existed, rawExisting, err := readJSONObject(claudeJSONPath)
		if err != nil {
			return err
		}
		if existed {
			if mcpServers, ok := root["mcpServers"].(map[string]any); ok {
				for _, comp := range DefaultComponents {
					delete(mcpServers, comp)
				}
			}
			if err := writeBackup(claudeJSONPath+".bak", rawExisting); err != nil {
				return err
			}
			if err := writeJSONAtomic(claudeJSONPath, root); err != nil {
				return err
			}
		}
	}

	if strings.TrimSpace(settingsJSONPath) != "" {
		root, existed, rawExisting, err := readJSONObject(settingsJSONPath)
		if err != nil {
			return err
		}
		if existed {
			removePCSHooks(root)
			if !onlyPCSOwned {
				if perms, ok := root["permissions"].(map[string]any); ok {
					existing := ensureStringSlice(perms, "allow")
					filtered := make([]string, 0, len(existing))
					for _, item := range existing {
						if !slices.Contains(DefaultPermissionAllows, item) {
							filtered = append(filtered, item)
						}
					}
					perms["allow"] = filtered
				}
			}
			if err := writeBackup(settingsJSONPath+".bak", rawExisting); err != nil {
				return err
			}
			if err := writeJSONAtomic(settingsJSONPath, root); err != nil {
				return err
			}
		}
	}

	return nil
}

// InspectConfiguredServers returns which of the 3 foundational MCP servers are
// present in the given .claude.json file.
func InspectConfiguredServers(claudeJSONPath string) (map[string]bool, error) {
	status := map[string]bool{
		ServerEngram:    false,
		ServerContext7:  false,
		ServerCodeGraph: false,
	}
	root, existed, _, err := readJSONObject(claudeJSONPath)
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

func ensureSkillRegistryHook(hooks map[string]any, targetDir string) {
	rawList, _ := hooks["UserPromptSubmit"].([]any)
	for _, item := range rawList {
		if strings.Contains(fmt.Sprint(item), "pcs skill-registry refresh") {
			// Preserve existing pcs skill-registry hook without duplicating.
			return
		}
	}

	cmd := "pcs skill-registry refresh"
	if strings.TrimSpace(targetDir) != "" {
		skillsPath := filepath.Join(targetDir, "skills")
		cmd = fmt.Sprintf("pcs skill-registry refresh --skills-dir %q", skillsPath)
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
}

func removePCSHooks(root map[string]any) {
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		return
	}
	rawList, ok := hooks["UserPromptSubmit"].([]any)
	if !ok {
		return
	}
	filtered := make([]any, 0, len(rawList))
	for _, item := range rawList {
		text := fmt.Sprint(item)
		if strings.Contains(text, "pcs skill-registry refresh") {
			continue
		}
		filtered = append(filtered, item)
	}
	hooks["UserPromptSubmit"] = filtered
}

func readJSONObject(path string) (map[string]any, bool, []byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), false, nil, nil
		}
		return nil, false, nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return make(map[string]any), true, b, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, true, b, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	if obj == nil {
		obj = make(map[string]any)
	}
	return obj, true, b, nil
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

func writeBackup(backupPath string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	if err := os.WriteFile(backupPath, content, 0o644); err != nil {
		return fmt.Errorf("write backup %s: %w", backupPath, err)
	}
	return nil
}

func writeJSONAtomic(path string, data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", path, err)
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON for %s: %w", path, err)
	}
	encoded = append(encoded, '\n')

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, encoded, 0o644); err != nil {
		return fmt.Errorf("write temp JSON %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp JSON %s -> %s: %w", tmpPath, path, err)
	}
	return nil
}
