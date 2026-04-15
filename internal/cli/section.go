package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nakagami-306/orbit/internal/workspace"
	"github.com/spf13/cobra"
)

func newSectionCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "section",
		Short: "Section operations",
	}

	cmd.AddCommand(newSectionAddCmd(app))
	cmd.AddCommand(newSectionLogCmd(app))
	return cmd
}

func newSectionAddCmd(app *App) *cobra.Command {
	var content, title, rationale string
	var position int

	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a new section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sectionTitle := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			if content == "" {
				return fmt.Errorf("--content is required")
			}

			decisionTitle := title
			if decisionTitle == "" {
				decisionTitle = fmt.Sprintf("Add section: %s", sectionTitle)
			}
			decisionRationale := rationale
			if decisionRationale == "" {
				decisionRationale = fmt.Sprintf("Added section %q", sectionTitle)
			}

			// Auto-position: count existing sections
			if position == 0 {
				sections, _ := app.Service.GetSections(cmd.Context(), info.ProjectEntityID, info.BranchID)
				position = len(sections)
			}

			secSID, decSID, err := app.Service.AddSection(
				cmd.Context(), info.ProjectEntityID, info.BranchID,
				sectionTitle, content, position,
				decisionTitle, decisionRationale, "user",
			)
			if err != nil {
				return err
			}

			// Re-render
			if info.Path != "" {
				workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, info.BranchID, info.Path)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action":      "added",
					"section":     sectionTitle,
					"section_id":  secSID[:8],
					"decision_id": decSID[:8],
				})
			}
			fmt.Printf("Added section %q (%s) — Decision %s\n", sectionTitle, secSID[:8], decSID[:8])
			return nil
		},
	}

	cmd.Flags().StringVar(&content, "content", "", "Section content (required)")
	cmd.Flags().StringVarP(&title, "title", "t", "", "Decision title")
	cmd.Flags().StringVarP(&rationale, "rationale", "r", "", "Decision rationale")
	cmd.Flags().IntVar(&position, "position", 0, "Section position")

	return cmd
}
