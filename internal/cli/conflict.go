package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nakagami-306/orbit/internal/workspace"
	"github.com/spf13/cobra"
)

func newConflictCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conflict",
		Short: "Conflict operations",
	}
	cmd.AddCommand(newConflictListCmd(app))
	cmd.AddCommand(newConflictShowCmd(app))
	cmd.AddCommand(newConflictResolveCmd(app))
	return cmd
}

func newConflictListCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List unresolved conflicts",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			rows, err := app.DB.Conn().Query(`
				SELECT c.stable_id, c.field, c.status, s.title
				FROM p_conflicts c
				JOIN p_sections s ON c.section_id = s.entity_id AND s.branch_id = c.branch_id
				WHERE c.project_id = ? AND c.branch_id = ?
				ORDER BY c.entity_id
			`, info.ProjectEntityID, info.BranchID)
			if err != nil {
				return err
			}
			defer rows.Close()

			type entry struct {
				StableID string `json:"stable_id"`
				Field    string `json:"field"`
				Status   string `json:"status"`
				Section  string `json:"section"`
			}
			var entries []entry
			for rows.Next() {
				var e entry
				rows.Scan(&e.StableID, &e.Field, &e.Status, &e.Section)
				entries = append(entries, e)
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			if len(entries) == 0 {
				fmt.Println("No conflicts.")
				return nil
			}
			for _, e := range entries {
				fmt.Printf("%s  [%s]  %s.%s\n", e.StableID[:8], e.Status, e.Section, e.Field)
			}
			return nil
		},
	}
}

func newConflictShowCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <conflict-id>",
		Short: "Show conflict details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prefix := args[0]

			var stableID, field, status, baseValue string
			var sectionID int64
			err := app.DB.Conn().QueryRow(`
				SELECT stable_id, section_id, field, status, COALESCE(base_value,'')
				FROM p_conflicts WHERE stable_id LIKE ?
			`, prefix+"%").Scan(&stableID, &sectionID, &field, &status, &baseValue)
			if err != nil {
				return fmt.Errorf("conflict %q not found: %w", prefix, err)
			}

			// Get section title
			var sectionTitle string
			app.DB.Conn().QueryRow("SELECT title FROM p_sections WHERE entity_id = ? LIMIT 1", sectionID).Scan(&sectionTitle)

			// Get sides
			sideRows, _ := app.DB.Conn().Query(`
				SELECT cs.branch_id, COALESCE(b.name, b.stable_id), cs.value
				FROM p_conflict_sides cs
				JOIN p_branches b ON cs.branch_id = b.entity_id
				WHERE cs.conflict_id = (SELECT entity_id FROM entities WHERE stable_id = ?)
			`, stableID)

			type side struct {
				Branch string `json:"branch"`
				Value  string `json:"value"`
			}
			var sides []side
			if sideRows != nil {
				for sideRows.Next() {
					var s side
					var branchID int64
					sideRows.Scan(&branchID, &s.Branch, &s.Value)
					sides = append(sides, s)
				}
				sideRows.Close()
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"stable_id": stableID, "section": sectionTitle, "field": field,
					"status": status, "base": baseValue, "sides": sides,
				})
			}

			fmt.Printf("Conflict: %s [%s]\n", stableID[:8], status)
			fmt.Printf("Section:  %s.%s\n", sectionTitle, field)
			if baseValue != "" {
				fmt.Printf("Base:     %s\n", baseValue)
			}
			for _, s := range sides {
				fmt.Printf("  %s: %s\n", s.Branch, s.Value)
			}
			return nil
		},
	}
}

func newConflictResolveCmd(app *App) *cobra.Command {
	var content, rationale string

	cmd := &cobra.Command{
		Use:   "resolve <conflict-id>",
		Short: "Resolve a conflict",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prefix := args[0]

			if content == "" {
				return fmt.Errorf("--content is required")
			}
			if rationale == "" {
				return fmt.Errorf("-r (rationale) is required")
			}

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Get conflict info
			var conflictEntityID, sectionID int64
			err = app.DB.Conn().QueryRow(`
				SELECT entity_id, section_id FROM p_conflicts
				WHERE stable_id LIKE ? AND status = 'unresolved'
			`, prefix+"%").Scan(&conflictEntityID, &sectionID)
			if err != nil {
				return fmt.Errorf("unresolved conflict %q not found: %w", prefix, err)
			}

			// Use EditSection to create a resolution decision
			decSID, err := app.Service.EditSection(
				cmd.Context(), sectionID, info.BranchID, info.ProjectEntityID,
				content, "Conflict resolution", rationale, "user",
			)
			if err != nil {
				return err
			}

			// Mark conflict as resolved
			app.DB.Conn().Exec(`
				UPDATE p_conflicts SET status = 'resolved', resolution = ?, resolution_rationale = ?
				WHERE entity_id = ?
			`, content, rationale, conflictEntityID)

			if info.Path != "" {
				workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, info.BranchID, info.Path)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "resolved", "decision_id": decSID[:8],
				})
			}
			fmt.Printf("Conflict resolved — Decision %s\n", decSID[:8])
			return nil
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "Resolution content (required)")
	cmd.Flags().StringVarP(&rationale, "rationale", "r", "", "Resolution rationale (required)")
	return cmd
}
