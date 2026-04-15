package workspace

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	orbitdb "github.com/nakagami-306/orbit/internal/db"
)

// Info holds resolved workspace information.
type Info struct {
	ProjectEntityID int64
	ProjectStableID string
	BranchID        int64
	Path            string
}

// DBPath returns the path to the central Orbit database.
// Uses ORBIT_DB env var if set, otherwise ~/.orbit/orbit.db.
func DBPath() string {
	if p := os.Getenv("ORBIT_DB"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".orbit", "orbit.db")
}

// Resolve finds the workspace info for the given directory.
// It looks for .orbit/config.toml and reads the project_id.
func Resolve(d *orbitdb.DB, dir string) (*Info, error) {
	configPath := filepath.Join(dir, ".orbit", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("no .orbit/config.toml in %s: %w", dir, err)
	}

	// Simple TOML parsing for project_id
	projectStableID := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "project_id") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				projectStableID = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			}
		}
	}
	if projectStableID == "" {
		return nil, fmt.Errorf("project_id not found in %s", configPath)
	}

	// Look up in workspaces table
	var info Info
	info.Path = dir
	info.ProjectStableID = projectStableID

	err = d.Conn().QueryRow(`
		SELECT w.project_id, w.current_branch_id
		FROM workspaces w
		JOIN entities e ON w.project_id = e.id
		WHERE e.stable_id = ?
		AND w.path = ?
	`, projectStableID, normalizePath(dir)).Scan(&info.ProjectEntityID, &info.BranchID)
	if err != nil {
		return nil, fmt.Errorf("workspace not found in db: %w", err)
	}

	return &info, nil
}

// Register creates a workspace mapping in the database and writes .orbit/config.toml.
func Register(d *orbitdb.DB, projectEntityID int64, projectStableID string, branchID int64, dir string) error {
	// Create .orbit directory
	orbitDir := filepath.Join(dir, ".orbit")
	if err := os.MkdirAll(orbitDir, 0755); err != nil {
		return fmt.Errorf("create .orbit dir: %w", err)
	}

	// Write config.toml
	config := fmt.Sprintf("project_id = %q\n", projectStableID)
	configPath := filepath.Join(orbitDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("write config.toml: %w", err)
	}

	// Insert workspace row
	_, err := d.Conn().Exec(`
		INSERT INTO workspaces (project_id, path, current_branch_id)
		VALUES (?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET project_id=excluded.project_id, current_branch_id=excluded.current_branch_id
	`, projectEntityID, normalizePath(dir), branchID)
	if err != nil {
		return fmt.Errorf("register workspace: %w", err)
	}

	return nil
}

// UpdateStateHash updates the hash stored for freshness checking.
func UpdateStateHash(conn *sql.DB, dir string, hash string) error {
	_, err := conn.Exec("UPDATE workspaces SET state_hash = ? WHERE path = ?", hash, normalizePath(dir))
	return err
}

// normalizePath ensures consistent path format across platforms.
func normalizePath(p string) string {
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ReplaceAll(p, "\\", "/")
	}
	return p
}
