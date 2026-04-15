package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nakagami-306/orbit/internal/workspace"
	"github.com/spf13/cobra"
)

func newShowCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current project state",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Freshness check: re-render .orbit/state.md if needed
			if info.Path != "" {
				workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, info.BranchID, info.Path)
			}

			project, err := app.Service.GetProjectByID(cmd.Context(), info.ProjectEntityID)
			if err != nil {
				return err
			}

			sections, err := app.Service.GetSections(cmd.Context(), info.ProjectEntityID, info.BranchID)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				data := map[string]any{
					"project": map[string]any{
						"name":        project.Name,
						"stable_id":   project.StableID,
						"description": project.Description,
						"status":      project.Status,
					},
					"sections": sections,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(data)
			}

			// Text output
			fmt.Printf("# %s\n\n", project.Name)
			if project.Description != "" {
				fmt.Printf("> %s\n\n", project.Description)
			}

			if len(sections) == 0 {
				fmt.Println("*No sections yet. Use `orbit edit` or `orbit section add` to add content.*")
			} else {
				for _, sec := range sections {
					if sec.IsStale {
						fmt.Printf("## %s ⚠ stale\n\n", sec.Title)
					} else {
						fmt.Printf("## %s\n\n", sec.Title)
					}
					if sec.Content != "" {
						fmt.Println(sec.Content)
						fmt.Println()
					}
				}
			}
			return nil
		},
	}
}
