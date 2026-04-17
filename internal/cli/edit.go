package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nakagami-306/orbit/internal/workspace"
	"github.com/spf13/cobra"
)

func newEditCmd(app *App) *cobra.Command {
	var title, rationale, content, sectionFlag, oldStr, newStr string
	var useStdin bool

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit project state and create a Decision",
		RunE: func(cmd *cobra.Command, args []string) error {
			if useStdin && content != "" {
				return fmt.Errorf("--stdin and --content are mutually exclusive")
			}
			if useStdin {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				content = strings.TrimRight(string(data), "\n\r")
			}

			if title == "" {
				return fmt.Errorf("-t (title) is required")
			}
			if rationale == "" {
				return fmt.Errorf("-r (rationale) is required")
			}

			// Validate: --content/--stdin/--old+--new, mutually exclusive
			hasContent := content != ""
			hasPatch := oldStr != "" || newStr != ""
			if hasContent && hasPatch {
				return fmt.Errorf("--content and --old/--new are mutually exclusive")
			}

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			sections, err := app.Service.GetSections(cmd.Context(), info.ProjectEntityID, info.BranchID)
			if err != nil {
				return err
			}

			// If no sections exist and no section flag, create an initial section
			if len(sections) == 0 && sectionFlag == "" {
				if content == "" {
					return fmt.Errorf("--content is required for the initial edit (no sections exist yet)")
				}
				secSID, decSID, err := app.Service.AddSection(
					cmd.Context(), info.ProjectEntityID, info.BranchID,
					"State", content, 0,
					title, rationale, "user",
				)
				if err != nil {
					return err
				}

				// Re-render .orbit/state.md
				if info.Path != "" {
					workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, info.BranchID, info.Path)
				}

				if app.Format == "json" {
					return json.NewEncoder(os.Stdout).Encode(map[string]any{
						"action":      "created_section",
						"section_id":  secSID[:8],
						"decision_id": decSID[:8],
					})
				}
				fmt.Printf("Created section \"State\" with Decision %s\n", decSID[:8])
				return nil
			}

			// Find target section
			if sectionFlag == "" && len(sections) == 1 {
				sectionFlag = sections[0].Title
			}
			if sectionFlag == "" {
				return fmt.Errorf("-s (section) is required when multiple sections exist")
			}

			var targetSection *struct {
				EntityID int64
				Title    string
			}
			for _, sec := range sections {
				if sec.Title == sectionFlag || sec.StableID == sectionFlag || fmt.Sprintf("%d", sec.EntityID) == sectionFlag {
					targetSection = &struct {
						EntityID int64
						Title    string
					}{sec.EntityID, sec.Title}
					break
				}
			}
			if targetSection == nil {
				return fmt.Errorf("section %q not found", sectionFlag)
			}

			// If patch mode, resolve the new content
			if hasPatch {
				sec, err := app.Service.GetSection(cmd.Context(), targetSection.EntityID, info.BranchID)
				if err != nil {
					return fmt.Errorf("read section: %w", err)
				}
				content, err = applyPatch(sec.Content, oldStr, newStr)
				if err != nil {
					return err
				}
			}

			if content == "" {
				return fmt.Errorf("--content, --stdin, or --old/--new is required")
			}

			decSID, err := app.Service.EditSection(
				cmd.Context(),
				targetSection.EntityID, info.BranchID, info.ProjectEntityID,
				content, title, rationale, "user",
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
					"action":      "edited",
					"section":     targetSection.Title,
					"decision_id": decSID[:8],
				})
			}
			fmt.Printf("Edited section %q — Decision %s\n", targetSection.Title, decSID[:8])
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "Decision title (required)")
	cmd.Flags().StringVarP(&rationale, "rationale", "r", "", "Decision rationale (required)")
	cmd.Flags().StringVar(&content, "content", "", "New content (full replace)")
	cmd.Flags().StringVarP(&sectionFlag, "section", "s", "", "Section to edit")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read content from stdin")
	cmd.Flags().StringVar(&oldStr, "old", "", "Text to find in section (for patch mode)")
	cmd.Flags().StringVar(&newStr, "new", "", "Replacement text (for patch mode)")

	return cmd
}
