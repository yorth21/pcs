package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RegistryFileName is the PCS-namespaced skill registry file written inside .atl/.
const RegistryFileName = "pcs-skill-registry.md"

// SkillEntry represents a discovered SKILL.md entry.
type SkillEntry struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

// RefreshResult reports the outcome of refreshing .atl/pcs-skill-registry.md.
type RefreshResult struct {
	OutputPath string       `json:"output_path"`
	Hash       string       `json:"hash"`
	CacheHit   bool         `json:"cache_hit"`
	Skills     []SkillEntry `json:"skills"`
}

// Refresh scans rootDir and extraSkillDirs for SKILL.md files, parses their
// YAML frontmatter, and writes a deterministic .atl/pcs-skill-registry.md table
// with a SHA-256 content hash header for cache-hit fast paths.
func Refresh(rootDir string, extraSkillDirs []string) (*RefreshResult, error) {
	if strings.TrimSpace(rootDir) == "" {
		return nil, fmt.Errorf("rootDir is required")
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve rootDir: %w", err)
	}

	candidates := []struct {
		dir   string
		scope string
	}{
		{dir: filepath.Join(absRoot, "brain", "skills"), scope: "workspace-brain"},
		{dir: filepath.Join(absRoot, ".claude", "skills"), scope: "workspace"},
		{dir: filepath.Join(absRoot, "skills"), scope: "workspace"},
	}

	normalizedExtras := make([]string, 0, len(extraSkillDirs))
	for _, extra := range extraSkillDirs {
		if strings.TrimSpace(extra) == "" {
			continue
		}
		absExtra, absErr := filepath.Abs(extra)
		if absErr != nil {
			absExtra = extra
		}
		normalizedExtras = append(normalizedExtras, absExtra)
	}

	for _, absExtra := range normalizedExtras {
		candidates = append(candidates, struct {
			dir   string
			scope string
		}{dir: absExtra, scope: classifyScope(absRoot, absExtra, normalizedExtras)})
	}

	byName := make(map[string]SkillEntry)
	for _, cand := range candidates {
		entries, err := os.ReadDir(cand.dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), "_") || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			skillFile := filepath.Join(cand.dir, entry.Name(), "SKILL.md")
			b, err := os.ReadFile(skillFile)
			if err != nil {
				continue
			}
			name, desc := parseFrontmatter(string(b), entry.Name())
			displayPath := formatDisplayPath(absRoot, skillFile)
			// Prefer earlier candidates (workspace-local overrides external profile/global dirs).
			if _, exists := byName[name]; !exists {
				byName[name] = SkillEntry{
					Name:        name,
					Scope:       cand.scope,
					Description: desc,
					Path:        displayPath,
				}
			}
		}
	}

	skills := make([]SkillEntry, 0, len(byName))
	for _, s := range byName {
		skills = append(skills, s)
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	body := renderTable(skills)
	sum := sha256.Sum256([]byte(body))
	hash := hex.EncodeToString(sum[:])

	outPath := filepath.Join(absRoot, ".atl", RegistryFileName)
	header := fmt.Sprintf("<!-- pcs:skill-registry-hash:%s -->\n", hash)
	fullContent := header + body

	existing, readErr := os.ReadFile(outPath)
	if readErr == nil && strings.HasPrefix(string(existing), header) {
		return &RefreshResult{
			OutputPath: outPath,
			Hash:       hash,
			CacheHit:   true,
			Skills:     skills,
		}, nil
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return nil, fmt.Errorf("create .atl directory: %w", err)
	}
	if err := os.WriteFile(outPath, []byte(fullContent), 0o644); err != nil {
		return nil, fmt.Errorf("write skill registry: %w", err)
	}

	return &RefreshResult{
		OutputPath: outPath,
		Hash:       hash,
		CacheHit:   false,
		Skills:     skills,
	}, nil
}

func classifyScope(absRoot, skillDir string, extraSkillDirs []string) string {
	normRoot := normalizePathForCompare(absRoot)
	normSkill := normalizePathForCompare(skillDir)

	if normRoot != "" && (normSkill == normRoot || strings.HasPrefix(normSkill, normRoot+"/")) {
		return "workspace"
	}
	if strings.Contains(normSkill, ".claude-pcs") {
		return "profile"
	}
	if envProfile := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); envProfile != "" {
		normEnv := normalizePathForCompare(envProfile)
		if normEnv != "" && (normSkill == normEnv || strings.HasPrefix(normSkill, normEnv+"/")) {
			return "profile"
		}
	}
	if strings.HasSuffix(normSkill, "/.claude/skills") {
		return "global"
	}
	for _, extra := range extraSkillDirs {
		normExtra := normalizePathForCompare(extra)
		if normExtra != "" && (normSkill == normExtra || strings.HasPrefix(normSkill, normExtra+"/")) {
			return "profile"
		}
	}
	return "global"
}

func normalizePathForCompare(p string) string {
	cleaned := filepath.ToSlash(filepath.Clean(strings.TrimSpace(p)))
	return strings.ToLower(strings.TrimSuffix(cleaned, "/"))
}

func formatDisplayPath(absRoot, skillFile string) string {
	if rel, err := filepath.Rel(absRoot, skillFile); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(skillFile)
}

func parseFrontmatter(content, fallbackName string) (string, string) {
	name := fallbackName
	desc := ""

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return name, desc
	}

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		if after, ok := strings.CutPrefix(line, "name:"); ok {
			val := strings.TrimSpace(after)
			val = strings.Trim(val, `"'`)
			if val != "" {
				name = val
			}
		} else if after, ok := strings.CutPrefix(line, "description:"); ok {
			val := strings.TrimSpace(after)
			val = strings.Trim(val, `"'`)
			if val != "" {
				desc = val
			}
		}
	}
	return name, desc
}

func renderTable(skills []SkillEntry) string {
	var b strings.Builder
	b.WriteString("# Skill Registry\n\n")
	b.WriteString("| Skill | Scope | Trigger / Description | Path |\n")
	b.WriteString("|-------|-------|-----------------------|------|\n")
	for _, s := range skills {
		desc := strings.ReplaceAll(s.Description, "|", "\\|")
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | `%s` |\n", s.Name, s.Scope, desc, s.Path))
	}
	return b.String()
}
