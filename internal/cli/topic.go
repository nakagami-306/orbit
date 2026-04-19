package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nakagami-306/orbit/internal/domain"
	"github.com/nakagami-306/orbit/internal/projection"
	"github.com/spf13/cobra"
)

func newTopicCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topic",
		Short: "Topic operations (group related threads)",
	}

	cmd.AddCommand(newTopicCreateCmd(app))
	cmd.AddCommand(newTopicListCmd(app))
	cmd.AddCommand(newTopicShowCmd(app))
	cmd.AddCommand(newTopicAddThreadCmd(app))
	cmd.AddCommand(newTopicRemoveThreadCmd(app))
	cmd.AddCommand(newTopicUpdateCmd(app))
	cmd.AddCommand(newTopicCloseCmd(app))
	return cmd
}

func topicService(app *App) *domain.TopicService {
	return &domain.TopicService{DB: app.DB, Projector: &projection.Projector{}}
}

func newTopicCreateCmd(app *App) *cobra.Command {
	var description string

	cmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new topic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := args[0]
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			svc := topicService(app)
			stableID, err := svc.CreateTopic(cmd.Context(), info.ProjectEntityID, title, description)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "created", "topic_id": stableID, "title": title,
				})
			}
			fmt.Printf("Created topic %q (%s)\n", title, stableID)
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "Topic description")
	return cmd
}

func newTopicListCmd(app *App) *cobra.Command {
	var statusFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List topics",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			svc := topicService(app)
			topics, err := svc.ListTopics(cmd.Context(), info.ProjectEntityID, statusFilter)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(topics)
			}

			if len(topics) == 0 {
				fmt.Println("No topics.")
				return nil
			}
			for _, t := range topics {
				fmt.Printf("%s  [%s]  %s\n", t.StableID, t.Status, t.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status")
	return cmd
}

func newTopicShowCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <topic-id>",
		Short: "Show topic with linked threads",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			topicPrefix := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			svc := topicService(app)
			topic, err := svc.FindTopic(cmd.Context(), info.ProjectEntityID, topicPrefix)
			if err != nil {
				return err
			}

			threads, err := svc.GetTopicThreads(cmd.Context(), topic.EntityID)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"topic":   topic,
					"threads": threads,
				})
			}

			fmt.Printf("Topic: %s [%s]\n", topic.Title, topic.Status)
			if topic.Description != "" {
				fmt.Printf("Description: %s\n", topic.Description)
			}
			fmt.Println()

			if len(threads) == 0 {
				fmt.Println("  No linked threads.")
			} else {
				fmt.Println("Linked threads:")
				for _, t := range threads {
					fmt.Printf("  %s  [%s]  %s\n", t.StableID, t.Status, t.Title)
				}
			}
			return nil
		},
	}
}

func newTopicAddThreadCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "add-thread <topic-id> <thread-id>",
		Short: "Link a thread to a topic",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			topicPrefix := args[0]
			threadPrefix := args[1]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Resolve topic
			var topicEntityID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_topics WHERE project_id = ? AND stable_id = ?",
				info.ProjectEntityID, topicPrefix,
			).Scan(&topicEntityID)
			if err != nil {
				return fmt.Errorf("topic %q not found: %w", topicPrefix, err)
			}

			// Resolve thread
			var threadEntityID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_threads WHERE project_id = ? AND stable_id = ?",
				info.ProjectEntityID, threadPrefix,
			).Scan(&threadEntityID)
			if err != nil {
				return fmt.Errorf("thread %q not found: %w", threadPrefix, err)
			}

			svc := topicService(app)
			if err := svc.AddThread(cmd.Context(), topicEntityID, threadEntityID); err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "linked", "topic_id": topicPrefix, "thread_id": threadPrefix,
				})
			}
			fmt.Printf("Linked thread %s to topic %s\n", threadPrefix, topicPrefix)
			return nil
		},
	}
}

func newTopicRemoveThreadCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "remove-thread <topic-id> <thread-id>",
		Short: "Unlink a thread from a topic",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			topicPrefix := args[0]
			threadPrefix := args[1]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Resolve topic
			var topicEntityID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_topics WHERE project_id = ? AND stable_id = ?",
				info.ProjectEntityID, topicPrefix,
			).Scan(&topicEntityID)
			if err != nil {
				return fmt.Errorf("topic %q not found: %w", topicPrefix, err)
			}

			// Resolve thread
			var threadEntityID int64
			err = app.DB.Conn().QueryRow(
				"SELECT entity_id FROM p_threads WHERE project_id = ? AND stable_id = ?",
				info.ProjectEntityID, threadPrefix,
			).Scan(&threadEntityID)
			if err != nil {
				return fmt.Errorf("thread %q not found: %w", threadPrefix, err)
			}

			svc := topicService(app)
			if err := svc.RemoveThread(cmd.Context(), topicEntityID, threadEntityID); err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action": "unlinked", "topic_id": topicPrefix, "thread_id": threadPrefix,
				})
			}
			fmt.Printf("Unlinked thread %s from topic %s\n", threadPrefix, topicPrefix)
			return nil
		},
	}
}

func newTopicUpdateCmd(app *App) *cobra.Command {
	var title, description string

	cmd := &cobra.Command{
		Use:   "update <topic-id>",
		Short: "Update a topic's title or description",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			topicPrefix := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			svc := topicService(app)
			topic, err := svc.FindTopic(cmd.Context(), info.ProjectEntityID, topicPrefix)
			if err != nil {
				return err
			}

			if err := svc.UpdateTopic(cmd.Context(), topic.EntityID, title, description); err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"action": "updated"})
			}
			fmt.Printf("Topic %q updated.\n", topic.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	return cmd
}

func newTopicCloseCmd(app *App) *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "close <topic-id>",
		Short: "Close (abandon) a topic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			topicPrefix := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			svc := topicService(app)
			topic, err := svc.FindTopic(cmd.Context(), info.ProjectEntityID, topicPrefix)
			if err != nil {
				return err
			}

			if err := svc.CloseTopic(cmd.Context(), topic.EntityID, reason); err != nil {
				return err
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"action": "closed"})
			}
			fmt.Println("Topic closed.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&reason, "reason", "r", "", "Reason for closing")
	return cmd
}
