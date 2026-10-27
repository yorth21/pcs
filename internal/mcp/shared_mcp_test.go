package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// stubEngram makes LookupEngramBinary resolve to path, or to nothing when path
// is empty, without consulting the host PATH or fallback locations.
func stubEngram(t *testing.T, path string) {
	t.Helper()
	origLookPath, origFallback := LookPathFunc, EngramFallbackPaths
	t.Cleanup(func() { LookPathFunc, EngramFallbackPaths = origLookPath, origFallback })
	EngramFallbackPaths = nil
	LookPathFunc = func(file string) (string, error) {
		if file == "engram" && path != "" {
			return path, nil
		}
		return "", errors.New("not found")
	}
}

func readSharedServers(t *testing.T, path string) map[string]any {
	t.Helper()
	root, _, _, _, err := readJSONObjectWithPerm(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	servers, _ := root["mcpServers"].(map[string]any)
	return servers
}

func TestGenerateSharedMCPFileProvidesEngram(t *testing.T) {
	mainServer := `"main-server": {"command": "custom-mcp"}`
	customEngram := `"engram": {"command": "/custom/engram", "args": ["mcp"]}`
	tests := []struct {
		name        string
		mainJSON    string
		engramPath  string
		wantServers []string
		wantMissing bool
		// wantEngramCommand is checked when engram must be present.
		wantEngramCommand string
	}{
		{name: "main without engram gains the resolved engram", mainJSON: `{"mcpServers": {` + mainServer + `}}`, engramPath: "/fake/bin/engram", wantServers: []string{"engram", "main-server"}, wantEngramCommand: "/fake/bin/engram"},
		{name: "main engram is mirrored unchanged", mainJSON: `{"mcpServers": {` + mainServer + `, ` + customEngram + `}}`, engramPath: "/fake/bin/engram", wantServers: []string{"engram", "main-server"}, wantEngramCommand: "/custom/engram"},
		{name: "unresolved engram is not written", mainJSON: `{"mcpServers": {` + mainServer + `}}`, wantServers: []string{"main-server"}, wantMissing: true},
		{name: "defaults use the resolved engram", mainJSON: `{}`, engramPath: "/fake/bin/engram", wantServers: []string{"codegraph", "context7", "engram"}, wantEngramCommand: "/fake/bin/engram"},
		{name: "defaults without engram binary omit engram", mainJSON: `{}`, wantServers: []string{"codegraph", "context7"}, wantMissing: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubEngram(t, tt.engramPath)
			dir := t.TempDir()
			mainPath := filepath.Join(dir, ".claude.json")
			if err := os.WriteFile(mainPath, []byte(tt.mainJSON), 0o600); err != nil {
				t.Fatal(err)
			}
			sharedPath := filepath.Join(dir, "profile", "shared-mcp.json")

			missing, err := GenerateSharedMCPFile(sharedPath, mainPath)
			if err != nil {
				t.Fatalf("GenerateSharedMCPFile() error: %v", err)
			}
			if missing != tt.wantMissing {
				t.Errorf("engramMissing = %v, want %v", missing, tt.wantMissing)
			}
			servers := readSharedServers(t, sharedPath)
			var names []string
			for name := range servers {
				names = append(names, name)
			}
			slices.Sort(names)
			if !reflect.DeepEqual(names, tt.wantServers) {
				t.Errorf("shared servers = %v, want %v", names, tt.wantServers)
			}
			if tt.wantEngramCommand != "" {
				engram, _ := servers["engram"].(map[string]any)
				if engram["command"] != tt.wantEngramCommand {
					t.Errorf("engram command = %v, want %s", engram["command"], tt.wantEngramCommand)
				}
			}
			if after, _ := os.ReadFile(mainPath); string(after) != tt.mainJSON {
				t.Errorf("main .claude.json changed: %s", after)
			}
		})
	}
}

func TestSharedMCPDriftIgnoresPCSProvidedEngram(t *testing.T) {
	stubEngram(t, "/fake/bin/engram")
	dir := t.TempDir()
	mainPath := filepath.Join(dir, ".claude.json")
	if err := os.WriteFile(mainPath, []byte(`{"mcpServers": {"main-server": {"command": "custom-mcp"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	sharedPath := filepath.Join(dir, "shared-mcp.json")
	if _, err := GenerateSharedMCPFile(sharedPath, mainPath); err != nil {
		t.Fatal(err)
	}
	drift, err := SharedMCPDrift(sharedPath, mainPath)
	if err != nil || len(drift) != 0 {
		t.Fatalf("SharedMCPDrift() = %v, %v; want no drift for the pcs-provided engram", drift, err)
	}

	// A genuinely extra server is still reported.
	if err := os.WriteFile(sharedPath, []byte(`{"mcpServers": {"main-server": {"command": "custom-mcp"}, "engram": {}, "stray": {}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	drift, err = SharedMCPDrift(sharedPath, mainPath)
	if err != nil || !reflect.DeepEqual(drift, []string{"stray"}) {
		t.Fatalf("SharedMCPDrift() = %v, %v; want [stray]", drift, err)
	}
}
