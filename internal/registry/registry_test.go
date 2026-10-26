package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefresh(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T, rootDir, extraDir string)
		useExtra       bool
		wantSkillNames []string
		wantContains   []string
	}{
		{
			name: "discovers workspace brain skills and writes deterministic table",
			setup: func(t *testing.T, rootDir, _ string) {
				writeSkill(t, filepath.Join(rootDir, "brain", "skills", "pcs-core", "SKILL.md"),
					"pcs-core", "Trigger: UX design, 5 planes.")
				writeSkill(t, filepath.Join(rootDir, "brain", "skills", "pcs-adapter-react-shadcn", "SKILL.md"),
					"pcs-adapter-react-shadcn", "Trigger: React 19 + shadcn/ui.")
				// _shared should be ignored as a skill entry.
				if err := os.MkdirAll(filepath.Join(rootDir, "brain", "skills", "_shared"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			wantSkillNames: []string{"pcs-adapter-react-shadcn", "pcs-core"},
			wantContains: []string{
				"| Skill | Scope | Trigger / Description | Path |",
				"| `pcs-core` | `workspace-brain` | Trigger: UX design, 5 planes. | `brain/skills/pcs-core/SKILL.md` |",
				"| `pcs-adapter-react-shadcn` | `workspace-brain` | Trigger: React 19 + shadcn/ui. | `brain/skills/pcs-adapter-react-shadcn/SKILL.md` |",
			},
		},
		{
			name: "merges extra profile skills directory when provided",
			setup: func(t *testing.T, rootDir, extraDir string) {
				writeSkill(t, filepath.Join(rootDir, ".claude", "skills", "local-skill", "SKILL.md"),
					"local-skill", "Trigger: Local repo workflow.")
				writeSkill(t, filepath.Join(extraDir, "pcs-core", "SKILL.md"),
					"pcs-core", "Trigger: Profile UX core.")
			},
			useExtra:       true,
			wantSkillNames: []string{"local-skill", "pcs-core"},
			wantContains: []string{
				"| `local-skill` | `workspace` | Trigger: Local repo workflow. | `.claude/skills/local-skill/SKILL.md` |",
				"| `pcs-core` |",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootDir := t.TempDir()
			extraDir := filepath.Join(t.TempDir(), ".claude-pcs", "skills")
			tt.setup(t, rootDir, extraDir)

			var extras []string
			if tt.useExtra {
				extras = []string{extraDir}
			}

			res, err := Refresh(rootDir, extras)
			if err != nil {
				t.Fatalf("Refresh() error: %v", err)
			}
			if res.CacheHit {
				t.Error("expected first Refresh() call to have CacheHit = false")
			}
			if len(res.Skills) != len(tt.wantSkillNames) {
				t.Fatalf("got %d skills, want %d", len(res.Skills), len(tt.wantSkillNames))
			}
			for i, wantName := range tt.wantSkillNames {
				if res.Skills[i].Name != wantName {
					t.Errorf("skill[%d].Name = %q, want %q", i, res.Skills[i].Name, wantName)
				}
			}

			b, err := os.ReadFile(res.OutputPath)
			if err != nil {
				t.Fatalf("read output registry: %v", err)
			}
			content := string(b)
			for _, sub := range tt.wantContains {
				if !strings.Contains(content, sub) {
					t.Errorf("registry missing %q in:\n%s", sub, content)
				}
			}

			// Second call with identical inputs must hit the SHA-256 fast path.
			res2, err := Refresh(rootDir, extras)
			if err != nil {
				t.Fatalf("second Refresh() error: %v", err)
			}
			if !res2.CacheHit {
				t.Error("expected second Refresh() call to report CacheHit = true")
			}
			if res2.Hash != res.Hash {
				t.Errorf("hash changed between identical runs: %s vs %s", res.Hash, res2.Hash)
			}
		})
	}
}

func writeSkill(t *testing.T, path, name, desc string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: \"" + desc + "\"\n---\n\n## Activation Contract\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
