package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newSectionLogCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "log <section>",
		Short: "Show decisions that changed a section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sectionName := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Find section by name or prefix
			var sectionEntityID int64
			err = app.DB.Conn().QueryRow(`
				SELECT entity_id FROM p_sections
				WHERE project_id = ? AND branch_id = ?
				  AND (title = ? OR stable_id = ?)
				LIMIT 1
			`, info.ProjectEntityID, info.BranchID, sectionName, sectionName).Scan(&sectionEntityID)
			if err != nil {
				return fmt.Errorf("section %q not found: %w", sectionName, err)
			}

			// Find all decisions that touched this section via datoms
			rows, err := app.DB.Conn().Query(`
				SELECT DISTINCT pd.stable_id, pd.title, pd.rationale, pd.instant
				FROM datoms d
				JOIN transactions t ON d.tx = t.id
				JOIN p_decisions pd ON t.decision_id = pd.entity_id AND pd.branch_id = ?
				WHERE d.e = ?
				ORDER BY t.id DESC
			`, info.BranchID, sectionEntityID)
			if err != nil {
				return err
			}
			defer rows.Close()

			type entry struct {
				StableID  string `json:"stable_id"`
				Title     string `json:"title"`
				Rationale string `json:"rationale"`
				Instant   string `json:"instant"`
			}
			entries := make([]entry, 0)
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.StableID, &e.Title, &e.Rationale, &e.Instant); err != nil {
					return err
				}
				entries = append(entries, e)
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			if len(entries) == 0 {
				fmt.Printf("No decisions have modified section %q.\n", sectionName)
				return nil
			}
			fmt.Printf("Decisions that modified section %q:\n\n", sectionName)
			for _, e := range entries {
				fmt.Printf("  %s  %s  %s\n", e.StableID, e.Instant[:19], e.Title)
				if e.Rationale != "" {
					fmt.Printf("            ↳ %s\n", e.Rationale)
				}
			}
			return nil
		},
	}
}
