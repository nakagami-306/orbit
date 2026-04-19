package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newDiffCmd(app *App) *cobra.Command {
	var sectionFilter string

	cmd := &cobra.Command{
		Use:   "diff <point-a> <point-b>",
		Short: "Show state differences between two points",
		Long:  "Points can be branch names, decision ID prefixes, or milestone names.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pointA := args[0]
			pointB := args[1]

			info, err := app.resolveProject()
			if err != nil {
				return err
			}

			// Resolve each point to a branch ID for section lookup
			branchA, err := resolvePoint(app, info.ProjectEntityID, pointA)
			if err != nil {
				return fmt.Errorf("point-a %q: %w", pointA, err)
			}
			branchB, err := resolvePoint(app, info.ProjectEntityID, pointB)
			if err != nil {
				return fmt.Errorf("point-b %q: %w", pointB, err)
			}

			// Get sections for each point
			sectionsA, err := app.Service.GetSections(cmd.Context(), info.ProjectEntityID, branchA)
			if err != nil {
				return err
			}
			sectionsB, err := app.Service.GetSections(cmd.Context(), info.ProjectEntityID, branchB)
			if err != nil {
				return err
			}

			// Build maps
			mapA := make(map[int64]struct{ Title, Content string })
			for _, s := range sectionsA {
				mapA[s.EntityID] = struct{ Title, Content string }{s.Title, s.Content}
			}
			mapB := make(map[int64]struct{ Title, Content string })
			for _, s := range sectionsB {
				mapB[s.EntityID] = struct{ Title, Content string }{s.Title, s.Content}
			}

			type diffEntry struct {
				Section string `json:"section"`
				Type    string `json:"type"` // added, removed, modified, unchanged
				Before  string `json:"before,omitempty"`
				After   string `json:"after,omitempty"`
			}
			diffs := make([]diffEntry, 0)

			// Check sections in A
			for id, a := range mapA {
				if sectionFilter != "" && a.Title != sectionFilter {
					continue
				}
				if b, ok := mapB[id]; ok {
					if a.Content != b.Content {
						diffs = append(diffs, diffEntry{Section: a.Title, Type: "modified", Before: a.Content, After: b.Content})
					}
				} else {
					diffs = append(diffs, diffEntry{Section: a.Title, Type: "removed", Before: a.Content})
				}
			}

			// Check sections only in B
			for id, b := range mapB {
				if sectionFilter != "" && b.Title != sectionFilter {
					continue
				}
				if _, ok := mapA[id]; !ok {
					diffs = append(diffs, diffEntry{Section: b.Title, Type: "added", After: b.Content})
				}
			}

			if app.Format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(diffs)
			}

			if len(diffs) == 0 {
				fmt.Println("No differences.")
				return nil
			}
			for _, d := range diffs {
				switch d.Type {
				case "added":
					fmt.Printf("+ Section %q (added)\n", d.Section)
					for _, line := range strings.Split(d.After, "\n") {
						fmt.Printf("  + %s\n", line)
					}
				case "removed":
					fmt.Printf("- Section %q (removed)\n", d.Section)
					for _, line := range strings.Split(d.Before, "\n") {
						fmt.Printf("  - %s\n", line)
					}
				case "modified":
					fmt.Printf("~ Section %q (modified)\n", d.Section)
					fmt.Printf("  Before: %s\n", truncate(d.Before, 80))
					fmt.Printf("  After:  %s\n", truncate(d.After, 80))
				}
				fmt.Println()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sectionFilter, "section", "", "Filter to a specific section")
	return cmd
}

// resolvePoint resolves a branch name, decision prefix, or milestone name to a branch entity ID.
func resolvePoint(app *App, projectEntityID int64, point string) (int64, error) {
	conn := app.DB.Conn()

	// Try branch name
	var branchID int64
	err := conn.QueryRow(
		"SELECT entity_id FROM p_branches WHERE project_id = ? AND (name = ? OR stable_id = ?)",
		projectEntityID, point, point,
	).Scan(&branchID)
	if err == nil {
		return branchID, nil
	}

	// Try as decision — find the branch it belongs to
	err = conn.QueryRow(
		"SELECT branch_id FROM p_decisions WHERE project_id = ? AND stable_id = ?",
		projectEntityID, point,
	).Scan(&branchID)
	if err == nil {
		return branchID, nil
	}

	return 0, fmt.Errorf("could not resolve %q to a branch or decision", point)
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
