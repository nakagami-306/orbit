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

func newThreadCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Thread operations (discussions)",
	}

	cmd.AddCommand(newThreadCreateCmd(app))
	cmd.AddCommand(newThreadListCmd(app))
	cmd.AddCommand(newThreadShowCmd(app))
	cmd.AddCommand(newThreadAddCmd(app))
	cmd.AddCommand(newThreadCloseCmd(app))
	return cmd
}

func threadService(app *App) *domain.ThreadService {
	return &domain.ThreadService{DB: app.DB, Projector: &projection.Projector{}}
}

func newThreadCreateCmd(app *App) *cobra.Command {
	var question string

	cmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new discussion thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := args[0]
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			svc := threadService(app)
			stableID, err := svc.CreateThread(cmd.Context(), info.ProjectEntityID, title, question, "user")
			if err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "created", "thread_id": stableID[:8], "title": title,
				})
			}
			fmt.Printf("Created thread %q (%s)\n", title, stableID[:8])
			return nil
		},
	}
	cmd.Flags().StringVarP(&question, "question", "q", "", "What is being discussed")
	return cmd
}

func newThreadListCmd(app *App) *cobra.Command {
	var statusFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List threads",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			svc := threadService(app)
			threads, err := svc.ListThreads(cmd.Context(), info.ProjectEntityID, statusFilter)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(threads)
			}

			if len(threads) == 0 {
				fmt.Println("No threads.")
				return nil
			}
			for _, t := range threads {
				fmt.Printf("%s  [%s]  %s\n", t.StableID[:8], t.Status, t.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status")
	return cmd
}

func newThreadShowCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <thread-id>",
		Short: "Show thread with entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			threadPrefix := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Find thread
			var threadEntityID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_threads WHERE project_id = ? AND stable_id LIKE ?",
				info.ProjectEntityID, threadPrefix+"%",
			).Scan(&threadEntityID)
			if err != nil {
				return fmt.Errorf("thread %q not found: %w", threadPrefix, err)
			}

			svc := threadService(app)
			thread, err := svc.GetThread(cmd.Context(), threadEntityID)
			if err != nil {
				return err
			}
			entries, err := svc.GetEntries(cmd.Context(), threadEntityID)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"thread":  thread,
					"entries": entries,
				})
			}

			fmt.Printf("Thread: %s [%s]\n", thread.Title, thread.Status)
			if thread.Question != "" {
				fmt.Printf("Question: %s\n", thread.Question)
			}
			fmt.Println()

			for _, e := range entries {
				prefix := fmt.Sprintf("  [%s]", e.Type)
				if e.IsRetracted {
					prefix += " ~~retracted~~"
				}
				if e.Stance != "" {
					prefix += fmt.Sprintf(" (%s)", e.Stance)
				}
				fmt.Printf("%s %s\n", prefix, e.Content)
			}
			return nil
		},
	}
}

func newThreadAddCmd(app *App) *cobra.Command {
	var entryType, content, stance string
	var targetPrefix string

	cmd := &cobra.Command{
		Use:   "add <thread-id>",
		Short: "Add an entry to a thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			threadPrefix := args[0]

			if entryType == "" {
				return fmt.Errorf("--type is required")
			}
			if content == "" {
				return fmt.Errorf("--content is required")
			}

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			var threadEntityID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_threads WHERE project_id = ? AND stable_id LIKE ?",
				info.ProjectEntityID, threadPrefix+"%",
			).Scan(&threadEntityID)
			if err != nil {
				return fmt.Errorf("thread %q not found: %w", threadPrefix, err)
			}

			// Resolve target if specified
			var targetID *int64
			if targetPrefix != "" {
				var tid int64
				err = app.DB.Conn().QueryRow(
					"SELECT entity_id FROM p_entries WHERE thread_id = ? AND stable_id LIKE ?",
					threadEntityID, targetPrefix+"%",
				).Scan(&tid)
				if err != nil {
					return fmt.Errorf("target entry %q not found: %w", targetPrefix, err)
				}
				targetID = &tid
			}

			// Validate argument type
			if entryType == "argument" {
				if targetID == nil {
					return fmt.Errorf("--target is required for argument entries")
				}
				if stance == "" {
					return fmt.Errorf("--stance is required for argument entries")
				}
			}

			svc := threadService(app)
			stableID, err := svc.AddEntry(cmd.Context(), threadEntityID, entryType, content, "user", targetID, stance)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "added", "entry_id": stableID[:8], "type": entryType,
				})
			}
			fmt.Printf("Added %s entry (%s)\n", entryType, stableID[:8])
			return nil
		},
	}

	cmd.Flags().StringVar(&entryType, "type", "", "Entry type: note/finding/option/argument/conclusion")
	cmd.Flags().StringVar(&content, "content", "", "Entry content")
	cmd.Flags().StringVar(&targetPrefix, "target", "", "Target entry ID (for argument)")
	cmd.Flags().StringVar(&stance, "stance", "", "Stance: for/against/neutral (for argument)")
	return cmd
}

func newThreadCloseCmd(app *App) *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "close <thread-id>",
		Short: "Close (abandon) a thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			threadPrefix := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			var threadEntityID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_threads WHERE project_id = ? AND stable_id LIKE ?",
				info.ProjectEntityID, threadPrefix+"%",
			).Scan(&threadEntityID)
			if err != nil {
				return fmt.Errorf("thread %q not found: %w", threadPrefix, err)
			}

			svc := threadService(app)
			if err := svc.CloseThread(cmd.Context(), threadEntityID, reason, "user"); err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"action": "closed"})
			}
			fmt.Println("Thread closed.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&reason, "reason", "r", "", "Reason for closing")
	return cmd
}

func newDecideCmd(app *App) *cobra.Command {
	var title, rationale, content, sectionFlag string

	cmd := &cobra.Command{
		Use:   "decide <thread-id>",
		Short: "Converge a thread into a Decision and update State",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			threadPrefix := args[0]

			if title == "" {
				return fmt.Errorf("-t (title) is required")
			}
			if rationale == "" {
				return fmt.Errorf("-r (rationale) is required")
			}

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			var threadEntityID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_threads WHERE project_id = ? AND stable_id LIKE ?",
				info.ProjectEntityID, threadPrefix+"%",
			).Scan(&threadEntityID)
			if err != nil {
				return fmt.Errorf("thread %q not found: %w", threadPrefix, err)
			}

			// Resolve section if specified
			var sectionEntityID int64
			if sectionFlag != "" {
				err = app.DB.Conn().QueryRow(
					"SELECT entity_id FROM p_sections WHERE project_id = ? AND branch_id = ? AND (title = ? OR stable_id LIKE ?)",
					info.ProjectEntityID, info.BranchID, sectionFlag, sectionFlag+"%",
				).Scan(&sectionEntityID)
				if err != nil {
					return fmt.Errorf("section %q not found: %w", sectionFlag, err)
				}
			}

			svc := threadService(app)
			decSID, err := svc.Decide(cmd.Context(), threadEntityID, info.ProjectEntityID, info.BranchID, sectionEntityID, content, title, rationale, "user")
			if err != nil {
				return err
			}

			if info.Path != "" {
				workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, info.BranchID, info.Path)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "decided", "decision_id": decSID[:8],
				})
			}
			fmt.Printf("Thread decided — Decision %s\n", decSID[:8])
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "Decision title (required)")
	cmd.Flags().StringVarP(&rationale, "rationale", "r", "", "Decision rationale (required)")
	cmd.Flags().StringVar(&content, "content", "", "New section content")
	cmd.Flags().StringVarP(&sectionFlag, "section", "s", "", "Section to update")
	return cmd
}
