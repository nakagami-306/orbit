package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newLogCmd(app *App) *cobra.Command {
	var since, until string

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Cross-project activity log",
		RunE: func(cmd *cobra.Command, args []string) error {
			query := `
				SELECT p.name, d.stable_id, d.title, d.author, d.instant
				FROM p_decisions d
				JOIN p_projects p ON d.project_id = p.entity_id
				WHERE 1=1
			`
			qargs := []any{}

			if app.ProjectFlag != "" {
				query += " AND p.name = ?"
				qargs = append(qargs, app.ProjectFlag)
			}
			if since != "" {
				query += " AND d.instant >= ?"
				qargs = append(qargs, since)
			}
			if until != "" {
				query += " AND d.instant <= ?"
				qargs = append(qargs, until)
			}
			query += " ORDER BY d.instant DESC LIMIT 50"

			rows, err := app.DB.Conn().Query(query, qargs...)
			if err != nil {
				return err
			}
			defer rows.Close()

			type entry struct {
				Project  string `json:"project"`
				StableID string `json:"stable_id"`
				Title    string `json:"title"`
				Author   string `json:"author"`
				Instant  string `json:"instant"`
			}
			var entries []entry
			for rows.Next() {
				var e entry
				rows.Scan(&e.Project, &e.StableID, &e.Title, &e.Author, &e.Instant)
				entries = append(entries, e)
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			if len(entries) == 0 {
				fmt.Println("No activity.")
				return nil
			}
			for _, e := range entries {
				instant := e.Instant
				if len(instant) > 19 {
					instant = instant[:19]
				}
				fmt.Printf("%s  %-15s  %s  %s\n", instant, e.Project, e.StableID[:8], e.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Start date (ISO 8601)")
	cmd.Flags().StringVar(&until, "until", "", "End date (ISO 8601)")
	return cmd
}
