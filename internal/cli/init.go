package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	orbitdb "github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/domain"
	"github.com/nakagami-306/orbit/internal/projection"
	"github.com/nakagami-306/orbit/internal/workspace"
	"github.com/spf13/cobra"
)

func newInitCmd(app *App) *cobra.Command {
	var description string
	var link string

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Initialize a new project or link to an existing one",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := workspace.DBPath()
			d, err := orbitdb.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer d.Close()
			app.DB = d

			svc := &domain.ProjectService{
				DB:        d,
				Projector: &projection.Projector{},
			}

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			ctx := context.Background()

			if link != "" {
				// Link mode: connect cwd to existing project
				p, err := svc.GetProjectByName(ctx, link)
				if err != nil {
					return fmt.Errorf("project %q not found: %w", link, err)
				}
				branchID, err := svc.GetMainBranch(ctx, p.EntityID)
				if err != nil {
					return err
				}
				if err := workspace.Register(d, p.EntityID, p.StableID, branchID, cwd); err != nil {
					return err
				}
				if err := workspace.RenderState(d.Conn(), p.EntityID, branchID, cwd); err != nil {
					return err
				}

				// Generate .claude/settings.json with hooks
				if err := generateClaudeSettings(cwd); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to generate .claude/settings.json: %v\n", err)
				}

				if app.Format == "json" {
					return json.NewEncoder(os.Stdout).Encode(map[string]any{
						"action": "linked", "project": p.Name, "stable_id": p.StableID, "path": cwd,
					})
				}
				fmt.Printf("Linked to project %q in %s\n", p.Name, cwd)
				return nil
			}

			// Create mode
			if len(args) == 0 {
				return fmt.Errorf("project name required: orbit init <name>")
			}
			name := args[0]

			projectStableID, branchID, err := svc.CreateProject(ctx, name, description)
			if err != nil {
				return err
			}

			// Get project entity ID for workspace registration
			p, err := svc.GetProjectByName(ctx, name)
			if err != nil {
				return err
			}

			if err := workspace.Register(d, p.EntityID, projectStableID, branchID, cwd); err != nil {
				return err
			}
			if err := workspace.RenderState(d.Conn(), p.EntityID, branchID, cwd); err != nil {
				return err
			}

			// Generate .claude/settings.json with hooks
			if err := generateClaudeSettings(cwd); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to generate .claude/settings.json: %v\n", err)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "created", "project": name, "stable_id": projectStableID, "path": cwd,
				})
			}
			fmt.Printf("Created project %q (%s)\n", name, projectStableID)
			fmt.Printf("Initialized .orbit/ in %s\n", cwd)
			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Project description")
	cmd.Flags().StringVar(&link, "link", "", "Link to existing project by name")

	return cmd
}

// orbitHookCommand generates the Python one-liner command pattern used by Orbit hooks.
func orbitHookCommand(scriptName string) string {
	return `PYTHONIOENCODING=utf-8 python -c "import json,os;p=json.load(open(os.path.expanduser('~/.claude/plugins/installed_plugins.json')))['plugins']['orbit@orbit'][0]['installPath'];exec(open(os.path.join(p,'hooks','` + scriptName + `'),encoding='utf-8').read())"`
}

// orbitHooks returns the full hooks configuration for Orbit.
func orbitHooks() map[string]any {
	return map[string]any{
		"SessionStart": []any{
			map[string]any{
				"matcher": "",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": orbitHookCommand("orbit-session-start.py"),
					},
				},
			},
		},
		"Stop": []any{
			map[string]any{
				"matcher": "",
				"hooks": []any{
					map[string]any{
						"type": "prompt",
						"prompt": "あなたはOrbitプロジェクトの議論記録監視です。直前のアシスタント応答を確認し、以下を判定してください:\n\n" +
							"1. チャット上で設計に関する議論（選択肢の提示、トレードオフの検討、問題の分析、方針の決定など）が行われたか？\n" +
							"2. その議論内容は orbit thread add で記録済みか？（直前のレスポンス内に orbit thread add の呼び出しがあれば記録済み）\n" +
							"3. open な thread が存在するか？（SessionStart で注入された状態を参照）\n\n" +
							"判定ルール:\n" +
							"- 設計議論があり、記録されていない場合 → {\"decision\": \"block\", \"reason\": \"設計に関する議論がありましたが、orbit thread add で記録されていません。finding/option/argument/conclusion のいずれかで記録してから続行してください。\"}\n" +
							"- コード実装、ファイル操作、テスト実行など議論を伴わないターンだった場合 → {\"decision\": \"approve\"}\n" +
							"- 議論はあったが既に orbit thread add で記録済みの場合 → {\"decision\": \"approve\"}\n" +
							"- 軽微な質問応答や確認だけの場合 → {\"decision\": \"approve\"}\n\n" +
							"重要: コード実装中やファイル操作中に毎回ブロックしないこと。ブロックするのは「設計上の議論が行われたのに記録されていない」場合のみ。",
					},
				},
			},
		},
		"PreToolUse": []any{
			map[string]any{
				"matcher": "Bash",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": orbitHookCommand("orbit-decide-guard.py"),
					},
				},
			},
		},
		"PreCompact": []any{
			map[string]any{
				"matcher": "",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": orbitHookCommand("orbit-pre-compact.py"),
					},
				},
			},
		},
	}
}

// isOrbitHookEntry checks if a hook entry contains an Orbit-related command.
func isOrbitHookEntry(entry map[string]any) bool {
	hooks, ok := entry["hooks"].([]any)
	if !ok {
		return false
	}
	for _, h := range hooks {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "orbit") {
			return true
		}
		if prompt, ok := hm["prompt"].(string); ok && strings.Contains(prompt, "orbit") {
			return true
		}
	}
	return false
}

// generateClaudeSettings generates or updates .claude/settings.json with Orbit hooks.
func generateClaudeSettings(cwd string) error {
	claudeDir := filepath.Join(cwd, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")

	// Ensure .claude/ directory exists
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create .claude directory: %w", err)
	}

	wantHooks := orbitHooks()

	// Check if settings.json exists
	data, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read settings.json: %w", err)
	}

	var settings map[string]any

	if os.IsNotExist(err) {
		// File doesn't exist: create new
		settings = map[string]any{
			"hooks": wantHooks,
		}
	} else {
		// File exists: parse and merge
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings.json: %w", err)
		}

		existingHooks, _ := settings["hooks"].(map[string]any)
		if existingHooks == nil {
			existingHooks = make(map[string]any)
		}

		changed := false
		for eventKey, orbitEntries := range wantHooks {
			orbitList := orbitEntries.([]any)

			existing, ok := existingHooks[eventKey]
			if !ok {
				// Event key doesn't exist: add it
				existingHooks[eventKey] = orbitList
				changed = true
				continue
			}

			existingList, ok := existing.([]any)
			if !ok {
				existingHooks[eventKey] = orbitList
				changed = true
				continue
			}

			// Remove existing Orbit entries
			var filtered []any
			for _, e := range existingList {
				em, ok := e.(map[string]any)
				if !ok {
					filtered = append(filtered, e)
					continue
				}
				if !isOrbitHookEntry(em) {
					filtered = append(filtered, e)
				}
			}

			// Append Orbit entries
			filtered = append(filtered, orbitList...)
			existingHooks[eventKey] = filtered
			changed = true
		}

		if !changed {
			fmt.Printf("Claude hooks already configured\n")
			return nil
		}

		settings["hooks"] = existingHooks
	}

	// Marshal with 2-space indent
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings.json: %w", err)
	}

	if err := os.WriteFile(settingsPath, append(out, '\n'), 0644); err != nil {
		return fmt.Errorf("write settings.json: %w", err)
	}

	fmt.Printf("Generated .claude/settings.json with hooks\n")
	return nil
}
