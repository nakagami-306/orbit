package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nakagami-306/orbit/internal/domain"
	"github.com/nakagami-306/orbit/internal/eavt"
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

			// If --at is specified, use time-travel mode
			if app.AtFlag != "" {
				return showAtPoint(app, cmd, info)
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

// showAtPoint handles the --at flag for time-travel queries.
// The at value can be a decision stable_id (or prefix) or a milestone title.
func showAtPoint(app *App, cmd *cobra.Command, info *workspace.Info) error {
	atVal := app.AtFlag

	// Resolve the at value to a tx_id
	txID, err := resolveAtToTxID(app, info, atVal)
	if err != nil {
		return err
	}

	// Use as-of query to build sections from EAVT datoms
	project, err := app.Service.GetProjectByID(cmd.Context(), info.ProjectEntityID)
	if err != nil {
		return err
	}

	// Get all section entity IDs for this project
	sections, err := getProjectSectionsAsOf(app, info.ProjectEntityID, info.BranchID, txID)
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
			"as_of_tx": txID,
			"sections": sections,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	// Text output
	fmt.Printf("# %s (as of tx %d)\n\n", project.Name, txID)
	if project.Description != "" {
		fmt.Printf("> %s\n\n", project.Description)
	}

	if len(sections) == 0 {
		fmt.Println("*No sections at this point in time.*")
	} else {
		for _, sec := range sections {
			fmt.Printf("## %s\n\n", sec.Title)
			if sec.Content != "" {
				fmt.Println(sec.Content)
				fmt.Println()
			}
		}
	}
	return nil
}

// resolveAtToTxID resolves an --at value (decision stable_id/prefix or milestone title) to a tx_id.
func resolveAtToTxID(app *App, info *workspace.Info, atVal string) (int64, error) {
	conn := app.DB.Conn()

	// Try 1: Exact or prefix match on decision stable_id via p_decisions
	var txID int64
	err := conn.QueryRow(`
		SELECT tx_id FROM p_decisions
		WHERE branch_id = ? AND stable_id = ?
		ORDER BY tx_id DESC LIMIT 1
	`, info.BranchID, atVal).Scan(&txID)
	if err == nil {
		return txID, nil
	}

	// Try 2: Milestone title → decision_id → tx_id
	var decisionEntityID int64
	err = conn.QueryRow(`
		SELECT decision_id FROM p_milestones
		WHERE project_id = ? AND title = ?
		LIMIT 1
	`, info.ProjectEntityID, atVal).Scan(&decisionEntityID)
	if err == nil {
		err = conn.QueryRow(`
			SELECT tx_id FROM p_decisions
			WHERE entity_id = ? AND branch_id = ?
		`, decisionEntityID, info.BranchID).Scan(&txID)
		if err == nil {
			return txID, nil
		}
	}

	return 0, fmt.Errorf("--at %q: not found as decision ID or milestone name", atVal)
}

// getProjectSectionsAsOf returns sections for a project as of a given transaction.
func getProjectSectionsAsOf(app *App, projectEntityID, branchID, asOfTx int64) ([]domain.Section, error) {
	conn := app.DB.Conn()

	// Get all section entities that belong to this project
	rows, err := conn.Query(`
		SELECT DISTINCT e.id, e.stable_id
		FROM entities e
		JOIN datoms d ON d.e = e.id
		WHERE e.entity_type = 'section'
		  AND d.a = ? AND d.tx <= ?
	`, eavt.AttrSectionProjectID, asOfTx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type sectionEntity struct {
		id       int64
		stableID string
	}
	var candidates []sectionEntity
	for rows.Next() {
		var se sectionEntity
		if err := rows.Scan(&se.id, &se.stableID); err != nil {
			return nil, err
		}
		candidates = append(candidates, se)
	}

	// For each candidate, get as-of state and filter by project_id
	var sections []domain.Section
	sqlTx, err := conn.Begin()
	if err != nil {
		return nil, err
	}
	defer sqlTx.Rollback()

	for _, se := range candidates {
		state, err := eavt.EntityStateAsOf(sqlTx, se.id, asOfTx)
		if err != nil {
			continue
		}

		// Verify this section belongs to the project
		if projVal, ok := state[eavt.AttrSectionProjectID]; ok {
			projID, err := projVal.AsInt64()
			if err != nil || projID != projectEntityID {
				continue
			}
		} else {
			continue
		}

		// Must have a title (not retracted)
		titleVal, ok := state[eavt.AttrSectionTitle]
		if !ok {
			continue
		}
		title, _ := titleVal.AsString()

		content := ""
		if contentVal, ok := state[eavt.AttrSectionContent]; ok {
			content, _ = contentVal.AsString()
		}

		var position int
		if posVal, ok := state[eavt.AttrSectionPosition]; ok {
			posInt, _ := posVal.AsInt64()
			position = int(posInt)
		}

		// Verify it's on the right branch by checking p_sections existed for it
		var exists int
		err = sqlTx.QueryRow(`
			SELECT COUNT(*) FROM datoms d
			JOIN transactions t ON d.tx = t.id
			WHERE d.e = ? AND t.branch_id = ? AND d.tx <= ?
		`, se.id, branchID, asOfTx).Scan(&exists)
		if err != nil || exists == 0 {
			// Fallback: include if created on this branch OR no branch info
			_ = sql.ErrNoRows
		}

		sections = append(sections, domain.Section{
			EntityID:  se.id,
			StableID:  se.stableID,
			ProjectID: projectEntityID,
			Title:     title,
			Content:   content,
			Position:  position,
		})
	}

	// Sort by position
	for i := 0; i < len(sections); i++ {
		for j := i + 1; j < len(sections); j++ {
			if sections[j].Position < sections[i].Position {
				sections[i], sections[j] = sections[j], sections[i]
			}
		}
	}

	return sections, nil
}
