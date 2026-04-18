package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nakagami-306/orbit/internal/eavt"
	"github.com/nakagami-306/orbit/internal/projection"
	"github.com/spf13/cobra"
)

func newMilestoneCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "milestone",
		Short: "Milestone operations",
	}
	cmd.AddCommand(newMilestoneSetCmd(app))
	cmd.AddCommand(newMilestoneListCmd(app))
	return cmd
}

func newMilestoneSetCmd(app *App) *cobra.Command {
	var atDecision, description string

	cmd := &cobra.Command{
		Use:   "set <title>",
		Short: "Set a milestone at a decision point",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Resolve target decision
			var decisionEntityID int64
			if atDecision != "" {
				err = app.DB.Conn().QueryRow(
					"SELECT entity_id FROM p_decisions WHERE project_id = ? AND stable_id LIKE ?",
					info.ProjectEntityID, atDecision+"%",
				).Scan(&decisionEntityID)
				if err != nil {
					return fmt.Errorf("decision %q not found: %w", atDecision, err)
				}
			} else {
				// Use current head
				app.DB.Conn().QueryRow(
					"SELECT head_decision_id FROM p_branches WHERE entity_id = ?",
					info.BranchID,
				).Scan(&decisionEntityID)
				if decisionEntityID == 0 {
					return fmt.Errorf("no decisions exist yet")
				}
			}

			stableID := eavt.NewStableID()
			err = app.DB.Tx(context.Background(), func(sqlTx *sql.Tx) error {
				var branchID int64
				sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", info.ProjectEntityID).Scan(&branchID)

				txID, err := eavt.BeginTx(sqlTx, nil, branchID, "user")
				if err != nil {
					return err
				}

				msID, err := eavt.CreateEntity(sqlTx, stableID, eavt.EntityMilestone, txID)
				if err != nil {
					return err
				}

				eavt.AssertDatom(sqlTx, msID, eavt.AttrMilestoneTitle, eavt.NewString(title), txID)
				eavt.AssertDatom(sqlTx, msID, eavt.AttrMilestoneProjectID, eavt.NewRef(info.ProjectEntityID), txID)
				eavt.AssertDatom(sqlTx, msID, eavt.AttrMilestoneDecisionID, eavt.NewRef(decisionEntityID), txID)
				if description != "" {
					eavt.AssertDatom(sqlTx, msID, eavt.AttrMilestoneDescription, eavt.NewString(description), txID)
				}

				p := &projection.Projector{}
				return p.ApplyDatoms(sqlTx, msID, eavt.EntityMilestone, branchID)
			})
			if err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "set", "milestone_id": stableID, "title": title,
				})
			}
			fmt.Printf("Milestone %q set (%s)\n", title, stableID)
			return nil
		},
	}
	cmd.Flags().StringVar(&atDecision, "at", "", "Decision to point to (default: current head)")
	cmd.Flags().StringVar(&description, "description", "", "Milestone description")
	return cmd
}

func newMilestoneListCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List milestones",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			rows, err := app.DB.Conn().Query(`
				SELECT m.stable_id, m.title, COALESCE(m.description,''), d.instant
				FROM p_milestones m
				LEFT JOIN p_decisions d ON m.decision_id = d.entity_id
				WHERE m.project_id = ?
				ORDER BY m.entity_id
			`, info.ProjectEntityID)
			if err != nil {
				return err
			}
			defer rows.Close()

			type entry struct {
				StableID    string `json:"stable_id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Instant     string `json:"instant"`
			}
			entries := make([]entry, 0)
			for rows.Next() {
				var e entry
				var instant sql.NullString
				rows.Scan(&e.StableID, &e.Title, &e.Description, &instant)
				if instant.Valid {
					e.Instant = instant.String
				}
				entries = append(entries, e)
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			if len(entries) == 0 {
				fmt.Println("No milestones.")
				return nil
			}
			for _, e := range entries {
				instant := ""
				if e.Instant != "" {
					instant = e.Instant[:19]
				}
				fmt.Printf("%s  %s  %s\n", e.StableID, instant, e.Title)
			}
			return nil
		},
	}
}
