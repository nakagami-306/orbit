package workspace

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RenderState generates .orbit/state.md from projection tables.
func RenderState(conn *sql.DB, projectEntityID, branchID int64, dir string) error {
	orbitDir := filepath.Join(dir, ".orbit")
	if err := os.MkdirAll(orbitDir, 0755); err != nil {
		return err
	}

	// Get project info
	var name, description, status string
	err := conn.QueryRow(
		"SELECT name, COALESCE(description,''), status FROM p_projects WHERE entity_id = ?",
		projectEntityID,
	).Scan(&name, &description, &status)
	if err != nil {
		return fmt.Errorf("get project for render: %w", err)
	}

	// Get branch name and head decision
	var branchName string
	var headDecisionID sql.NullInt64
	conn.QueryRow(
		"SELECT COALESCE(name,'(unnamed)'), head_decision_id FROM p_branches WHERE entity_id = ?",
		branchID,
	).Scan(&branchName, &headDecisionID)

	// Get head decision stable ID for header
	headStableID := "none"
	if headDecisionID.Valid {
		conn.QueryRow("SELECT stable_id FROM entities WHERE id = ?", headDecisionID.Int64).Scan(&headStableID)
	}

	// Get sections
	rows, err := conn.Query(`
		SELECT title, COALESCE(content,''), is_stale
		FROM p_sections
		WHERE project_id = ? AND branch_id = ?
		ORDER BY position
	`, projectEntityID, branchID)
	if err != nil {
		return fmt.Errorf("get sections for render: %w", err)
	}
	defer rows.Close()

	type sectionData struct {
		Title   string
		Content string
		IsStale bool
	}
	var sections []sectionData
	for rows.Next() {
		var s sectionData
		var stale int
		if err := rows.Scan(&s.Title, &s.Content, &stale); err != nil {
			return err
		}
		s.IsStale = stale == 1
		sections = append(sections, s)
	}

	// Build markdown
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<!-- orbit:generated | %s | branch:%s | head:%s -->\n",
		time.Now().UTC().Format(time.RFC3339), branchName, headStableID))
	b.WriteString(fmt.Sprintf("# %s\n\n", name))
	if description != "" {
		b.WriteString(fmt.Sprintf("> %s\n\n", description))
	}

	if len(sections) == 0 {
		b.WriteString("*No sections yet. Use `orbit edit` or `orbit section add` to add content.*\n")
	} else {
		for _, sec := range sections {
			if sec.IsStale {
				b.WriteString(fmt.Sprintf("## %s ⚠ stale\n\n", sec.Title))
			} else {
				b.WriteString(fmt.Sprintf("## %s\n\n", sec.Title))
			}
			if sec.Content != "" {
				b.WriteString(sec.Content)
				b.WriteString("\n\n")
			}
		}
	}

	content := b.String()

	// Write file
	statePath := filepath.Join(orbitDir, "state.md")
	if err := os.WriteFile(statePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write state.md: %w", err)
	}

	// Update hash
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))[:16]
	UpdateStateHash(conn, dir, hash)

	return nil
}
