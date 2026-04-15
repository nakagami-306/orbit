package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nakagami-306/orbit/internal/eavt"
	"github.com/nakagami-306/orbit/internal/workspace"
	"github.com/spf13/cobra"
)

func newRevertCmd(app *App) *cobra.Command {
	var rationale string

	cmd := &cobra.Command{
		Use:   "revert <decision-id>",
		Short: "Revert a decision (create compensation decision)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			decisionPrefix := args[0]

			if rationale == "" {
				return fmt.Errorf("-r (rationale) is required")
			}

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Find the decision's tx_id
			var targetTxID int64
			var targetTitle string
			var targetDecisionEntityID int64
			err = app.DB.Conn().QueryRow(`
				SELECT d.entity_id, d.title, d.tx_id
				FROM p_decisions d
				WHERE d.project_id = ? AND d.branch_id = ? AND d.stable_id LIKE ?
				LIMIT 1
			`, info.ProjectEntityID, info.BranchID, decisionPrefix+"%").Scan(
				&targetDecisionEntityID, &targetTitle, &targetTxID,
			)
			if err != nil {
				return fmt.Errorf("decision %q not found: %w", decisionPrefix, err)
			}

			// Get datoms from the target transaction (only section/project changes)
			datomRows, err := app.DB.Conn().Query(`
				SELECT d.e, d.a, d.v, d.op, e.entity_type
				FROM datoms d
				JOIN entities e ON d.e = e.id
				WHERE d.tx = ?
				  AND e.entity_type IN ('section', 'project')
			`, targetTxID)
			if err != nil {
				return err
			}

			type revertDatom struct {
				E          int64
				A          string
				V          string
				Op         int
				EntityType string
			}
			var toRevert []revertDatom
			for datomRows.Next() {
				var rd revertDatom
				if err := datomRows.Scan(&rd.E, &rd.A, &rd.V, &rd.Op, &rd.EntityType); err != nil {
					datomRows.Close()
					return err
				}
				toRevert = append(toRevert, rd)
			}
			datomRows.Close()

			if len(toRevert) == 0 {
				return fmt.Errorf("no revertable changes found in decision %q", decisionPrefix)
			}

			// Create compensation decision
			compensationStableID := eavt.NewStableID()

			err = app.DB.Tx(context.Background(), func(sqlTx *sql.Tx) error {
				// Get current head
				var currentHead int64
				sqlTx.QueryRow("SELECT head_decision_id FROM p_branches WHERE entity_id = ?", info.BranchID).Scan(&currentHead)

				// Create compensation decision entity
				compDecisionID, err := eavt.CreateEntity(sqlTx, compensationStableID, eavt.EntityDecision, 0)
				if err != nil {
					return err
				}

				// Create EAVT transaction
				txID, err := eavt.BeginTx(sqlTx, &compDecisionID, info.BranchID, "user")
				if err != nil {
					return err
				}

				// Invert each datom: assert becomes retract, retract becomes assert
				for _, rd := range toRevert {
					val, err := eavt.DecodeValue(rd.V)
					if err != nil {
						return err
					}
					if rd.Op == 1 {
						// Was asserted -> retract it
						if err := eavt.RetractDatom(sqlTx, rd.E, rd.A, val, txID); err != nil {
							return err
						}
					} else {
						// Was retracted -> re-assert it
						if err := eavt.AssertDatom(sqlTx, rd.E, rd.A, val, txID); err != nil {
							return err
						}
					}
				}

				// Assert compensation decision datoms
				compTitle := fmt.Sprintf("Revert: %s", targetTitle)
				eavt.AssertDatom(sqlTx, compDecisionID, eavt.AttrDecisionTitle, eavt.NewString(compTitle), txID)
				eavt.AssertDatom(sqlTx, compDecisionID, eavt.AttrDecisionRationale, eavt.NewString(rationale), txID)
				eavt.AssertDatom(sqlTx, compDecisionID, eavt.AttrDecisionAuthor, eavt.NewString("user"), txID)
				eavt.AssertDatom(sqlTx, compDecisionID, eavt.AttrDecisionProjectID, eavt.NewRef(info.ProjectEntityID), txID)
				if currentHead > 0 {
					eavt.AssertDatom(sqlTx, compDecisionID, eavt.AttrDecisionParents, eavt.NewRefSet([]int64{currentHead}), txID)
				}

				// Update branch head
				if currentHead > 0 {
					eavt.RetractDatom(sqlTx, info.BranchID, eavt.AttrBranchHeadDecision, eavt.NewRef(currentHead), txID)
				}
				eavt.AssertDatom(sqlTx, info.BranchID, eavt.AttrBranchHeadDecision, eavt.NewRef(compDecisionID), txID)

				// Re-apply projections for affected entities
				affected := make(map[int64]string)
				for _, rd := range toRevert {
					affected[rd.E] = rd.EntityType
				}
				for entityID, entityType := range affected {
					app.Service.Projector.ApplyDatoms(sqlTx, entityID, eavt.EntityType(entityType), info.BranchID)
				}
				app.Service.Projector.ApplyDatoms(sqlTx, compDecisionID, eavt.EntityDecision, info.BranchID)
				app.Service.Projector.ApplyDatoms(sqlTx, info.BranchID, eavt.EntityBranch, info.BranchID)

				return nil
			})
			if err != nil {
				return err
			}

			// Re-render
			if info.Path != "" {
				workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, info.BranchID, info.Path)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action":          "reverted",
					"reverted":        targetTitle,
					"compensation_id": compensationStableID[:8],
				})
			}
			fmt.Printf("Reverted %q — Compensation Decision %s\n", targetTitle, compensationStableID[:8])
			return nil
		},
	}

	cmd.Flags().StringVarP(&rationale, "rationale", "r", "", "Reason for reverting (required)")
	return cmd
}
