package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nakagami-306/orbit/internal/workspace"
	"github.com/spf13/cobra"
)

func newSectionCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "section",
		Short: "Section operations",
	}

	cmd.AddCommand(newSectionAddCmd(app))
	cmd.AddCommand(newSectionLogCmd(app))
	cmd.AddCommand(newSectionRefCmd(app))
	cmd.AddCommand(newSectionRemoveCmd(app))
	cmd.AddCommand(newSectionShowCmd(app))
	return cmd
}

func newSectionAddCmd(app *App) *cobra.Command {
	var content, title, rationale string
	var position int
	var useStdin bool

	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a new section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if useStdin && content != "" {
				return fmt.Errorf("--stdin and --content are mutually exclusive")
			}
			if useStdin {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				content = strings.TrimRight(string(data), "\n\r")
			}

			sectionTitle := args[0]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			if content == "" {
				return fmt.Errorf("--content is required")
			}

			decisionTitle := title
			if decisionTitle == "" {
				decisionTitle = fmt.Sprintf("Add section: %s", sectionTitle)
			}
			decisionRationale := rationale
			if decisionRationale == "" {
				decisionRationale = fmt.Sprintf("Added section %q", sectionTitle)
			}

			// Auto-position: count existing sections
			if position == 0 {
				sections, _ := app.Service.GetSections(cmd.Context(), info.ProjectEntityID, info.BranchID)
				position = len(sections)
			}

			secSID, decSID, err := app.Service.AddSection(
				cmd.Context(), info.ProjectEntityID, info.BranchID,
				sectionTitle, content, position,
				decisionTitle, decisionRationale, "user",
			)
			if err != nil {
				return err
			}

			// Re-render
			if info.Path != "" {
				workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, info.BranchID, info.Path)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action":      "added",
					"section":     sectionTitle,
					"section_id":  secSID,
					"decision_id": decSID,
				})
			}
			fmt.Printf("Added section %q (%s) — Decision %s\n", sectionTitle, secSID, decSID)
			return nil
		},
	}

	cmd.Flags().StringVar(&content, "content", "", "Section content (required)")
	cmd.Flags().StringVarP(&title, "title", "t", "", "Decision title")
	cmd.Flags().StringVarP(&rationale, "rationale", "r", "", "Decision rationale")
	cmd.Flags().IntVar(&position, "position", 0, "Section position")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read content from stdin")

	return cmd
}

func newSectionRefCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ref",
		Short: "Manage section references",
	}

	cmd.AddCommand(newSectionRefAddCmd(app))
	cmd.AddCommand(newSectionRefRemoveCmd(app))
	return cmd
}

func newSectionRefAddCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "add <from-section> <to-section>",
		Short: "Add a reference from one section to another",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			fromSec, err := app.Service.FindSectionByNameOrID(cmd.Context(), args[0], info.ProjectEntityID, info.BranchID)
			if err != nil {
				return fmt.Errorf("from-section: %w", err)
			}
			toSec, err := app.Service.FindSectionByNameOrID(cmd.Context(), args[1], info.ProjectEntityID, info.BranchID)
			if err != nil {
				return fmt.Errorf("to-section: %w", err)
			}

			decSID, err := app.Service.AddSectionRef(
				cmd.Context(), fromSec.EntityID, toSec.EntityID,
				info.BranchID, info.ProjectEntityID, "user",
			)
			if err != nil {
				return err
			}

			// Re-render
			if info.Path != "" {
				workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, info.BranchID, info.Path)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action":       "ref_added",
					"from_section": fromSec.Title,
					"to_section":   toSec.Title,
					"decision_id":  decSID,
				})
			}
			fmt.Printf("Added reference: %s → %s — Decision %s\n", fromSec.Title, toSec.Title, decSID)
			return nil
		},
	}
}

func newSectionRefRemoveCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <from-section> <to-section>",
		Short: "Remove a reference from one section to another",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			fromSec, err := app.Service.FindSectionByNameOrID(cmd.Context(), args[0], info.ProjectEntityID, info.BranchID)
			if err != nil {
				return fmt.Errorf("from-section: %w", err)
			}
			toSec, err := app.Service.FindSectionByNameOrID(cmd.Context(), args[1], info.ProjectEntityID, info.BranchID)
			if err != nil {
				return fmt.Errorf("to-section: %w", err)
			}

			decSID, err := app.Service.RemoveSectionRef(
				cmd.Context(), fromSec.EntityID, toSec.EntityID,
				info.BranchID, info.ProjectEntityID, "user",
			)
			if err != nil {
				return err
			}

			// Re-render
			if info.Path != "" {
				workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, info.BranchID, info.Path)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action":       "ref_removed",
					"from_section": fromSec.Title,
					"to_section":   toSec.Title,
					"decision_id":  decSID,
				})
			}
			fmt.Printf("Removed reference: %s → %s — Decision %s\n", fromSec.Title, toSec.Title, decSID)
			return nil
		},
	}
}

func newSectionRemoveCmd(app *App) *cobra.Command {
	var rationale string

	cmd := &cobra.Command{
		Use:   "remove <section>",
		Short: "Remove a section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if rationale == "" {
				return fmt.Errorf("-r (rationale) is required")
			}

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			sec, err := app.Service.FindSectionByNameOrID(cmd.Context(), args[0], info.ProjectEntityID, info.BranchID)
			if err != nil {
				return err
			}

			decSID, warnings, err := app.Service.RemoveSection(
				cmd.Context(), sec.EntityID, info.BranchID, info.ProjectEntityID,
				rationale, "user",
			)
			if err != nil {
				return err
			}

			// Re-render
			if info.Path != "" {
				workspace.RenderState(app.DB.Conn(), info.ProjectEntityID, info.BranchID, info.Path)
			}

			if app.Format == "json" {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"action":      "removed",
					"section":     sec.Title,
					"decision_id": decSID,
					"warnings":    warnings,
				})
			}

			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "warning: %s\n", w)
			}
			fmt.Printf("Removed section %q — Decision %s\n", sec.Title, decSID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&rationale, "rationale", "r", "", "Rationale for removal (required)")

	return cmd
}

func newSectionShowCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <section>",
		Short: "Show section details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			sec, err := app.Service.FindSectionByNameOrID(cmd.Context(), args[0], info.ProjectEntityID, info.BranchID)
			if err != nil {
				return err
			}

			detail, err := app.Service.GetSection(cmd.Context(), sec.EntityID, info.BranchID)
			if err != nil {
				return err
			}

			if app.Format == "json" {
				data := map[string]any{
					"stable_id":    detail.StableID,
					"title":        detail.Title,
					"content":      detail.Content,
					"position":     detail.Position,
					"is_stale":     detail.IsStale,
					"stale_reason": detail.StaleReason,
				}
				refsTo := []map[string]any{}
				for _, r := range detail.RefsTo {
					refsTo = append(refsTo, map[string]any{
						"stable_id": r.StableID,
						"title":     r.Title,
					})
				}
				refsFrom := []map[string]any{}
				for _, r := range detail.RefsFrom {
					refsFrom = append(refsFrom, map[string]any{
						"stable_id": r.StableID,
						"title":     r.Title,
					})
				}
				data["refs_to"] = refsTo
				data["refs_from"] = refsFrom
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(data)
			}

			// Text output
			fmt.Printf("## %s", detail.Title)
			if detail.IsStale {
				fmt.Print(" [stale]")
			}
			fmt.Println()
			fmt.Printf("ID: %s\n", detail.StableID)
			fmt.Println()

			if detail.Content != "" {
				fmt.Println(detail.Content)
				fmt.Println()
			}

			if len(detail.RefsTo) > 0 {
				fmt.Println("References →")
				for _, r := range detail.RefsTo {
					fmt.Printf("  • %s (%s)\n", r.Title, r.StableID)
				}
				fmt.Println()
			}

			if len(detail.RefsFrom) > 0 {
				fmt.Println("Referenced by ←")
				for _, r := range detail.RefsFrom {
					fmt.Printf("  • %s (%s)\n", r.Title, r.StableID)
				}
				fmt.Println()
			}

			if detail.IsStale && detail.StaleReason != "" {
				fmt.Printf("Stale reason: %s\n", detail.StaleReason)
			}

			return nil
		},
	}
}
