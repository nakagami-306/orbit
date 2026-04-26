package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateClaudeSettings_NewFile(t *testing.T) {
	dir := t.TempDir()

	if err := generateClaudeSettings(dir); err != nil {
		t.Fatalf("generateClaudeSettings failed: %v", err)
	}

	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("failed to parse settings.json: %v", err)
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatal("settings.json missing 'hooks' key")
	}

	// Verify all four event keys exist
	for _, key := range []string{"SessionStart", "Stop", "PreToolUse", "PreCompact"} {
		if _, ok := hooks[key]; !ok {
			t.Errorf("hooks missing %q key", key)
		}
	}

	// Verify SessionStart has the orbit command
	sessionStart, ok := hooks["SessionStart"].([]any)
	if !ok || len(sessionStart) == 0 {
		t.Fatal("SessionStart is empty or wrong type")
	}
	entry := sessionStart[0].(map[string]any)
	innerHooks := entry["hooks"].([]any)
	hook := innerHooks[0].(map[string]any)
	cmd, ok := hook["command"].(string)
	if !ok {
		t.Fatal("SessionStart hook missing command")
	}
	if cmd == "" {
		t.Error("SessionStart hook command is empty")
	}

	// Verify Stop has a command hook referencing orbit-stop-nudge.py
	stop := hooks["Stop"].([]any)
	stopEntry := stop[0].(map[string]any)
	stopHooks := stopEntry["hooks"].([]any)
	stopHook := stopHooks[0].(map[string]any)
	stopCmd, ok := stopHook["command"].(string)
	if !ok {
		t.Error("Stop hook missing command")
	}
	if !strings.Contains(stopCmd, "orbit-stop-nudge.py") {
		t.Errorf("Stop hook command does not reference orbit-stop-nudge.py: %s", stopCmd)
	}
}

func TestGenerateClaudeSettings_MergeExisting(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write existing settings with permissions and a custom hook
	existing := map[string]any{
		"permissions": map[string]any{
			"allow": []string{"Read", "Write"},
		},
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "echo custom-hook",
						},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := generateClaudeSettings(dir); err != nil {
		t.Fatalf("generateClaudeSettings failed: %v", err)
	}

	// Read back
	result, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(result, &settings); err != nil {
		t.Fatal(err)
	}

	// Verify permissions are preserved
	perms, ok := settings["permissions"].(map[string]any)
	if !ok {
		t.Error("permissions key was lost during merge")
	} else {
		allow, ok := perms["allow"].([]any)
		if !ok || len(allow) != 2 {
			t.Error("permissions.allow was modified")
		}
	}

	hooks := settings["hooks"].(map[string]any)

	// Verify all four orbit event keys exist
	for _, key := range []string{"SessionStart", "Stop", "PreToolUse", "PreCompact"} {
		if _, ok := hooks[key]; !ok {
			t.Errorf("hooks missing %q key after merge", key)
		}
	}

	// Verify SessionStart has both the custom hook and the orbit hook
	sessionStart := hooks["SessionStart"].([]any)
	if len(sessionStart) != 2 {
		t.Errorf("expected 2 SessionStart entries (custom + orbit), got %d", len(sessionStart))
	}

	// First should be the custom hook
	firstEntry := sessionStart[0].(map[string]any)
	firstHooks := firstEntry["hooks"].([]any)
	firstHook := firstHooks[0].(map[string]any)
	if firstHook["command"] != "echo custom-hook" {
		t.Error("custom hook was not preserved as first entry")
	}
}

func TestGenerateClaudeSettings_NoDuplicate(t *testing.T) {
	dir := t.TempDir()

	// First run: create settings
	if err := generateClaudeSettings(dir); err != nil {
		t.Fatalf("first generateClaudeSettings failed: %v", err)
	}

	// Second run: should update (replace orbit hooks, not duplicate)
	if err := generateClaudeSettings(dir); err != nil {
		t.Fatalf("second generateClaudeSettings failed: %v", err)
	}

	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}

	hooks := settings["hooks"].(map[string]any)

	// Each event key should have exactly 1 entry (no duplicates)
	for _, key := range []string{"SessionStart", "Stop", "PreToolUse", "PreCompact"} {
		entries := hooks[key].([]any)
		if len(entries) != 1 {
			t.Errorf("%s has %d entries, expected 1 (no duplicates)", key, len(entries))
		}
	}
}
