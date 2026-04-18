package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newProjectCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Project operations",
	}

	cmd.AddCommand(newProjectListCmd(app))
	return cmd
}

func newProjectListCmd(app *App) *cobra.Command {
	var statusFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := app.Service.ListProjects(cmd.Context(), statusFilter)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(projects)
			}

			if len(projects) == 0 {
				fmt.Println("No projects found.")
				return nil
			}

			for _, p := range projects {
				fmt.Printf("%-20s  [%s]  %s\n", p.Name, p.Status, p.StableID)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status")
	return cmd
}
