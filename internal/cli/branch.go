package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nakagami-306/orbit/internal/domain"
	"github.com/nakagami-306/orbit/internal/projection"
	"github.com/nakagami-306/orbit/internal/workspace"
	"github.com/spf13/cobra"
)

func newBranchCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Branch operations",
	}
	cmd.AddCommand(newBranchCreateCmd(app))
	cmd.AddCommand(newBranchListCmd(app))
	cmd.AddCommand(newBranchSwitchCmd(app))
	cmd.AddCommand(newBranchNameCmd(app))
	cmd.AddCommand(newBranchMergeCmd(app))
	cmd.AddCommand(newBranchAbandonCmd(app))
	return cmd
}

func branchService(app *App) *domain.BranchService {
	return &domain.BranchService{DB: app.DB, Projector: &projection.Projector{}}
}

func newBranchCreateCmd(app *App) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			svc := branchService(app)
			stableID, err := svc.CreateBranch(cmd.Context(), info.ProjectEntityID, info.BranchID, name)
			if err != nil {
				return err
			}

			displayName := name
			if displayName == "" {
				displayName = stableID
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "created", "branch_id": stableID, "name": name,
				})
			}
			fmt.Printf("Created branch %s (%s)\n", displayName, stableID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Branch name (optional, anonymous if omitted)")
	return cmd
}

func newBranchListCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List branches",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			svc := branchService(app)
			branches, err := svc.ListBranches(cmd.Context(), info.ProjectEntityID)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(branches)
			}

			for _, b := range branches {
				marker := " "
				if b.EntityID == info.BranchID {
					marker = "*"
				}
				name := b.Name
				if name == "" {
					name = b.StableID
				}
				fmt.Printf(" %s %-20s [%s]", marker, name, b.Status)
				if b.IsMain {
					fmt.Print(" (main)")
				}
				fmt.Println()
			}
			return nil
		},
	}
}

func newBranchSwitchCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "switch <branch>",
		Short: "Switch to a branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Find branch by name or prefix
			var branchID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_branches WHERE project_id = ? AND (name = ? OR stable_id = ?)",
				info.ProjectEntityID, branchName, branchName,
			).Scan(&branchID)
			if err != nil {
				return fmt.Errorf("branch %q not found: %w", branchName, err)
			}

			svc := branchService(app)
			if err := svc.SwitchBranch(cmd.Context(), info.Path, branchID); err != nil {
				return err
			}

			// Re-render state.md for new branch
			if info.Path != "" {
				workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, branchID, info.Path)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"action": "switched", "branch": branchName})
			}
			fmt.Printf("Switched to branch %q\n", branchName)
			return nil
		},
	}
}

func newBranchNameCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "name <branch> <name>",
		Short: "Name an anonymous branch",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchPrefix := args[0]
			newName := args[1]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			var branchID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_branches WHERE project_id = ? AND (name = ? OR stable_id = ?)",
				info.ProjectEntityID, branchPrefix, branchPrefix,
			).Scan(&branchID)
			if err != nil {
				return fmt.Errorf("branch %q not found: %w", branchPrefix, err)
			}

			svc := branchService(app)
			if err := svc.NameBranch(cmd.Context(), branchID, newName); err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"action": "named", "name": newName})
			}
			fmt.Printf("Branch named %q\n", newName)
			return nil
		},
	}
}

func newBranchMergeCmd(app *App) *cobra.Command {
	var into string

	cmd := &cobra.Command{
		Use:   "merge <source>",
		Short: "Merge a branch into target (default: main)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceRef := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Resolve source branch
			var sourceID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_branches WHERE project_id = ? AND (name = ? OR stable_id = ?)",
				info.ProjectEntityID, sourceRef, sourceRef,
			).Scan(&sourceID)
			if err != nil {
				return fmt.Errorf("source branch %q not found: %w", sourceRef, err)
			}

			// Resolve target branch
			targetID := info.BranchID
			if into != "" {
				err = app.DB.Conn().QueryRow(
					"SELECT entity_id FROM p_branches WHERE project_id = ? AND (name = ? OR stable_id = ?)",
					info.ProjectEntityID, into, into,
				).Scan(&targetID)
				if err != nil {
					return fmt.Errorf("target branch %q not found: %w", into, err)
				}
			}

			svc := branchService(app)
			decSID, conflicts, err := svc.MergeBranch(cmd.Context(), sourceID, targetID, info.ProjectEntityID, "user")
			if err != nil {
				return err
			}

			if info.Path != "" {
				workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, targetID, info.Path)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "merged", "decision_id": decSID, "conflicts": conflicts,
				})
			}
			fmt.Printf("Merged — Decision %s\n", decSID)
			if conflicts > 0 {
				fmt.Printf("⚠ %d conflict(s). Use `orbit conflict list` to view.\n", conflicts)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&into, "into", "", "Target branch (default: current)")
	return cmd
}

func newBranchAbandonCmd(app *App) *cobra.Command {
	var rationale string

	cmd := &cobra.Command{
		Use:   "abandon <branch>",
		Short: "Abandon a branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchRef := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			var branchID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_branches WHERE project_id = ? AND (name = ? OR stable_id = ?)",
				info.ProjectEntityID, branchRef, branchRef,
			).Scan(&branchID)
			if err != nil {
				return fmt.Errorf("branch %q not found: %w", branchRef, err)
			}

			svc := branchService(app)
			if err := svc.AbandonBranch(cmd.Context(), branchID, rationale); err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"action": "abandoned"})
			}
			fmt.Printf("Branch %q abandoned.\n", branchRef)
			return nil
		},
	}
	cmd.Flags().StringVarP(&rationale, "rationale", "r", "", "Reason for abandoning")
	return cmd
}
