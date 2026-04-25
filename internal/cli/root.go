package cli

import (
	"context"
	"fmt"
	"os"

	orbitdb "github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/domain"
	"github.com/nakagami-306/orbit/internal/projection"
	"github.com/nakagami-306/orbit/internal/workspace"
	"github.com/spf13/cobra"
)

const banner = `
                                      .=+++=..
                                    .#@@@@@@@#-
                                   .#@@@@@@@@@%.
                                   .@@@@@@@@@@@.
                                   .%@@@@%@@@@%.
                                    .#@+#:@@@%-
                                      .+@:++..               .......
                .....                 .+=.                .-*%@@@@@#*.
             .*%@@@@%#.              .=@=.               .%@@@@@@@@@@@+.
            .*@@@@@@@@#.          ........             .#@@@@@@@@@@@@@@@-
            .*@@@@@@@%*.       .=@@@@@@@@@-.          .*@@@@@@@@@@@@@@@@@:
            .*@@@@*=*#*=====..=@@@@@@@@@@@@%-.      ...*@@@@@@@@@@@@@@@@@:
             .#@@@@@@@...++*@+#@@@@@@@@@@@@%=..+++++@@**@@@@@@@@@@@@@@@@@:
              ..====:.      .*@@@@@@@@@@@@@@@@@@@@@@@@%+%@@@@@@@@@@@@@@@+:
                            .*@@@@@@@@@@@@@@*:::::......:@@@@@@@@@@@@@@+:
                            .*=@@@@@@@@@@@@@@=.          .:@@@@@@@@@@#..
                            -*@@#+@@@@@@+#@@*.             ..*%%%%%-.
                          .+@@@:@@@@@%=@@+*+.
                        .+@@@#- -+#@@@#+@@@+.
                 ......=%@@@#-    ......#@@@%=.
               .%%@@@@@@%-@%.           .%@@@%-
             :#@@@@@@@@@@@@:             .@@@@-:-@@@@@*::.
           .=@@@@@@@@@@@@@@@*.            :**@@@@@@@@@@@@@%-..
           -@@@@@@@@@@@@@@@@@+.          .@@@@@@@@@@@@@@@@@@@=.
           -@@@@@@@@@@@@@@@@@+.        .+@@@@@@@@@@@@@@@@@@@@@#.
           -@@@@@@@@@@@@@@@@@+.       .+%@@@@@@@@@@@@@@@@@@@@@@#.
           :+@@@@@@@@@@@@@@@*:        .+@@@@@@@@@@@@@@@@@@@@@@@@.
            .+@@@@@@@@@@@@@#.        .=@@@@@@@@@@@@@@@@@@@@@@@@@#.
             ..*@@@@@@@@@#..         .=@@@@@@@@@@@@@@@@@@@@@@@@@#.
                .-+++++=..           .=@@@@@@@@@@@@@@@@@@@@@@@@@#.
                                      .+@@@@@@@@@@@@@@@@@@@@@@@@.
                                       -#@@@@@@@@@@@@@@@@@@@@@@-.
                                        .%@@@@@@@@@@@@@@@@@@@@:
                                         .%%@@@@@@@@@@@@@@@@%=.
                                           .=#@@@@@@@@@@@#*.
                                              ..-+++++=..

  ORBIT — Version-control your design decisions.
`

// App holds shared state for all CLI commands.
type App struct {
	DB        *orbitdb.DB
	Service   *domain.ProjectService
	Format    string
	ProjectFlag string
	BranchFlag  string
	AtFlag      string
}

func NewRootCmd() *cobra.Command {
	app := &App{}

	root := &cobra.Command{
		Use:           "orbit",
		Short:         "Project state version control",
		Long:          "Orbit — version-control your project's design state, decisions, and progress.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(banner)
			cmd.Usage()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip DB open for commands that handle their own DB
			if cmd.Name() == "init" {
				return nil
			}
			// ui and its subcommands manage their own lifecycle
			for c := cmd; c != nil; c = c.Parent() {
				if c.Name() == "ui" {
					return nil
				}
			}
			dbPath := workspace.DBPath()
			d, err := orbitdb.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			app.DB = d
			app.Service = &domain.ProjectService{
				DB:        d,
				Projector: &projection.Projector{},
			}

			// Auto-scan: pull-mode git scan on every command.
			// No-op if cwd is not inside a git repo or no Orbit project resolves.
			// Errors are logged to stderr but never block the actual command.
			autoScan(cmd, app)
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if app.DB != nil {
				app.DB.Close()
			}
		},
	}

	root.PersistentFlags().StringVar(&app.Format, "format", "text", "Output format: text or json")
	root.PersistentFlags().StringVar(&app.ProjectFlag, "project", "", "Project name (default: resolve from .orbit/)")
	root.PersistentFlags().StringVarP(&app.BranchFlag, "branch", "b", "", "Branch name")
	root.PersistentFlags().StringVar(&app.AtFlag, "at", "", "Time travel: decision ID or milestone name")

	root.AddCommand(newInitCmd(app))
	root.AddCommand(newShowCmd(app))
	root.AddCommand(newStatusCmd(app))
	root.AddCommand(newEditCmd(app))
	root.AddCommand(newSectionCmd(app))
	root.AddCommand(newProjectCmd(app))
	root.AddCommand(newDecisionCmd(app))
	root.AddCommand(newRevertCmd(app))
	root.AddCommand(newThreadCmd(app))
	root.AddCommand(newDecideCmd(app))
	root.AddCommand(newBranchCmd(app))
	root.AddCommand(newConflictCmd(app))
	root.AddCommand(newTaskCmd(app))
	root.AddCommand(newTopicCmd(app))
	root.AddCommand(newMilestoneCmd(app))
	root.AddCommand(newDiffCmd(app))
	root.AddCommand(newLogCmd(app))
	root.AddCommand(newUICmd(app))
	root.AddCommand(newCommitCmd(app))
	root.AddCommand(newSyncCmd(app))

	return root
}

// autoScan runs the git commit scan as a side-effect of any orbit command.
// Skipped for `sync` (which already scans), `init` (no project yet), `ui`,
// and when a `--project` flag is set (which decouples cwd from the project,
// so scanning the cwd's git tree against an unrelated project is meaningless).
// Failures are logged but never propagated.
func autoScan(cmd *cobra.Command, app *App) {
	switch cmd.Name() {
	case "sync", "init":
		return
	}
	if app.ProjectFlag != "" {
		return
	}
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "ui" {
			return
		}
	}
	info, err := app.resolveProject()
	if err != nil {
		return
	}
	if _, err := runScanForProject(cmd.Context(), app, info.ProjectEntityID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-scan failed: %v\n", err)
	}
}

// resolveProject resolves the project entity ID and branch ID from flags or cwd.
func (app *App) resolveProject() (*workspace.Info, error) {
	if app.ProjectFlag != "" {
		ctx := context.Background()
		p, err := app.Service.GetProjectByName(ctx, app.ProjectFlag)
		if err != nil {
			return nil, err
		}
		branchID, err := app.Service.GetMainBranch(ctx, p.EntityID)
		if err != nil {
			return nil, err
		}
		return &workspace.Info{
			ProjectEntityID: p.EntityID,
			ProjectStableID: p.StableID,
			BranchID:        branchID,
		}, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get cwd: %w", err)
	}
	return workspace.Resolve(app.DB, cwd)
}
