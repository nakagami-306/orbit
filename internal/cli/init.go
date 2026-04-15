package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	orbitdb "github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/domain"
	"github.com/nakagami-306/orbit/internal/projection"
	"github.com/nakagami-306/orbit/internal/workspace"
	"github.com/spf13/cobra"
)

func newInitCmd(app *App) *cobra.Command {
	var description string
	var link string

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Initialize a new project or link to an existing one",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := workspace.DBPath()
			d, err := orbitdb.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer d.Close()
			app.DB = d

			svc := &domain.ProjectService{
				DB:        d,
				Projector: &projection.Projector{},
			}

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			ctx := context.Background()

			if link != "" {
				// Link mode: connect cwd to existing project
				p, err := svc.GetProjectByName(ctx, link)
				if err != nil {
					return fmt.Errorf("project %q not found: %w", link, err)
				}
				branchID, err := svc.GetMainBranch(ctx, p.EntityID)
				if err != nil {
					return err
				}
				if err := workspace.Register(d, p.EntityID, p.StableID, branchID, cwd); err != nil {
					return err
				}
				if err := workspace.RenderState(d.Conn(), p.EntityID, branchID, cwd); err != nil {
					return err
				}

				if app.Format == "json" {
					return json.NewEncoder(os.Stdout).Encode(map[string]any{
						"action": "linked", "project": p.Name, "stable_id": p.StableID, "path": cwd,
					})
				}
				fmt.Printf("Linked to project %q in %s\n", p.Name, cwd)
				return nil
			}

			// Create mode
			if len(args) == 0 {
				return fmt.Errorf("project name required: orbit init <name>")
			}
			name := args[0]

			projectStableID, branchID, err := svc.CreateProject(ctx, name, description)
			if err != nil {
				return err
			}

			// Get project entity ID for workspace registration
			p, err := svc.GetProjectByName(ctx, name)
			if err != nil {
				return err
			}

			if err := workspace.Register(d, p.EntityID, projectStableID, branchID, cwd); err != nil {
				return err
			}
			if err := workspace.RenderState(d.Conn(), p.EntityID, branchID, cwd); err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "created", "project": name, "stable_id": projectStableID, "path": cwd,
				})
			}
			fmt.Printf("Created project %q (%s)\n", name, projectStableID[:8])
			fmt.Printf("Initialized .orbit/ in %s\n", cwd)
			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Project description")
	cmd.Flags().StringVar(&link, "link", "", "Link to existing project by name")

	return cmd
}
