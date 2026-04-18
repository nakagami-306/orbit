package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newDecisionCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decision",
		Short: "Decision operations",
	}

	cmd.AddCommand(newDecisionLogCmd(app))
	cmd.AddCommand(newDecisionShowCmd(app))
	return cmd
}

func newDecisionLogCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "log",
		Short: "Show decision history",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			rows, err := app.DB.Conn().Query(`
				SELECT d.stable_id, d.title, d.rationale, d.author, d.instant
				FROM p_decisions d
				WHERE d.project_id = ? AND d.branch_id = ?
				ORDER BY d.tx_id DESC
			`, info.ProjectEntityID, info.BranchID)
			if err != nil {
				return err
			}
			defer rows.Close()

			type entry struct {
				StableID  string `json:"stable_id"`
				Title     string `json:"title"`
				Rationale string `json:"rationale"`
				Author    string `json:"author"`
				Instant   string `json:"instant"`
			}
			entries := make([]entry, 0)
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.StableID, &e.Title, &e.Rationale, &e.Author, &e.Instant); err != nil {
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
				fmt.Println("No decisions yet.")
				return nil
			}
			for _, e := range entries {
				fmt.Printf("%s  %s  %s\n", e.StableID, e.Instant[:19], e.Title)
				if e.Rationale != "" {
					fmt.Printf("          ↳ %s\n", e.Rationale)
				}
			}
			return nil
		},
	}
}

func newDecisionShowCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <decision-id>",
		Short: "Show decision details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			decisionPrefix := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Find decision by prefix match
			var stableID, title, rationale, context, author, instant string
			var txID int64
			err = app.DB.Conn().QueryRow(`
				SELECT d.stable_id, d.title, COALESCE(d.rationale,''), COALESCE(d.context,''),
				       COALESCE(d.author,''), d.instant, d.tx_id
				FROM p_decisions d
				WHERE d.project_id = ? AND d.branch_id = ? AND d.stable_id LIKE ?
				LIMIT 1
			`, info.ProjectEntityID, info.BranchID, decisionPrefix+"%").Scan(
				&stableID, &title, &rationale, &context, &author, &instant, &txID,
			)
			if err != nil {
				return fmt.Errorf("decision %q not found: %w", decisionPrefix, err)
			}

			// Get datoms for this transaction to show what changed
			datomRows, err := app.DB.Conn().Query(`
				SELECT e.stable_id, e.entity_type, d.a, d.v, d.op
				FROM datoms d
				JOIN entities e ON d.e = e.id
				WHERE d.tx = ?
				  AND e.entity_type IN ('section', 'project')
				ORDER BY d.e, d.a
			`, txID)
			if err != nil {
				return err
			}
			defer datomRows.Close()

			type change struct {
				EntityStableID string `json:"entity_id"`
				EntityType     string `json:"entity_type"`
				Attribute      string `json:"attribute"`
				Value          string `json:"value"`
				Op             int    `json:"op"`
			}
			changes := make([]change, 0)
			for datomRows.Next() {
				var c change
				if err := datomRows.Scan(&c.EntityStableID, &c.EntityType, &c.Attribute, &c.Value, &c.Op); err != nil {
					return err
				}
				changes = append(changes, c)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"stable_id": stableID,
					"title":     title,
					"rationale": rationale,
					"context":   context,
					"author":    author,
					"instant":   instant,
					"changes":   changes,
				})
			}

			fmt.Printf("Decision: %s\n", stableID)
			fmt.Printf("Title:    %s\n", title)
			fmt.Printf("When:     %s\n", instant[:19])
			if author != "" {
				fmt.Printf("Author:   %s\n", author)
			}
			if rationale != "" {
				fmt.Printf("Why:      %s\n", rationale)
			}
			if context != "" {
				fmt.Printf("Context:  %s\n", context)
			}
			if len(changes) > 0 {
				fmt.Println("\nChanges:")
				for _, c := range changes {
					op := "+"
					if c.Op == 0 {
						op = "-"
					}
					fmt.Printf("  %s [%s] %s.%s\n", op, c.EntityStableID, c.EntityType, c.Attribute)
				}
			}
			return nil
		},
	}
}
