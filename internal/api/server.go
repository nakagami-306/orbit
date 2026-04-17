package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	orbitdb "github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/domain"
	"github.com/nakagami-306/orbit/internal/projection"
	"github.com/nakagami-306/orbit/internal/workspace"
)

// Server is the Orbit Web UI API server.
type Server struct {
	db       *orbitdb.DB
	projects *domain.ProjectService
	threads  *domain.ThreadService
	tasks    *domain.TaskService
	branches *domain.BranchService
	mux      *http.ServeMux
	server   *http.Server
	addr     string
}

// NewServer creates a new API server.
func NewServer(addr string) (*Server, error) {
	dbPath := workspace.DBPath()
	d, err := orbitdb.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	proj := &projection.Projector{}
	s := &Server{
		db:       d,
		projects: &domain.ProjectService{DB: d, Projector: proj},
		threads:  &domain.ThreadService{DB: d, Projector: proj},
		tasks:    &domain.TaskService{DB: d, Projector: proj},
		branches: &domain.BranchService{DB: d, Projector: proj},
		addr:     addr,
	}

	s.mux = http.NewServeMux()
	s.registerRoutes()

	s.server = &http.Server{
		Addr:    addr,
		Handler: cors(s.mux),
	}

	return s, nil
}

// Start starts the HTTP server in the background and returns the actual address.
func (s *Server) Start() (string, error) {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}
	actualAddr := ln.Addr().String()

	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	return actualAddr, nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.server.Shutdown(ctx)
	s.db.Close()
	return err
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Projects
	s.mux.HandleFunc("GET /api/projects", s.handleListProjects)
	s.mux.HandleFunc("GET /api/projects/{id}", s.handleGetProject)

	// DAG
	s.mux.HandleFunc("GET /api/projects/{id}/dag", s.handleGetDAG)

	// Decisions
	s.mux.HandleFunc("GET /api/projects/{id}/decisions/{did}", s.handleGetDecision)

	// Sections
	s.mux.HandleFunc("GET /api/projects/{id}/sections", s.handleListSections)
	s.mux.HandleFunc("GET /api/projects/{id}/sections/{sid}", s.handleGetSection)

	// Threads
	s.mux.HandleFunc("GET /api/projects/{id}/threads", s.handleListThreads)
	s.mux.HandleFunc("GET /api/projects/{id}/threads/{tid}", s.handleGetThread)

	// Branches
	s.mux.HandleFunc("GET /api/projects/{id}/branches", s.handleListBranches)

	// Milestones
	s.mux.HandleFunc("GET /api/projects/{id}/milestones", s.handleListMilestones)

	// Conflicts
	s.mux.HandleFunc("GET /api/projects/{id}/conflicts", s.handleListConflicts)

	// Tasks (cross-project)
	s.mux.HandleFunc("GET /api/tasks", s.handleListTasks)
	s.mux.HandleFunc("PATCH /api/tasks/{id}", s.handleUpdateTask)

	// Static file serving (embedded frontend)
	if useEmbed {
		sub, err := fs.Sub(distFS, "dist")
		if err != nil {
			log.Printf("Warning: could not mount embedded frontend: %v", err)
			return
		}
		fileServer := http.FileServer(http.FS(sub))
		s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			// SPA fallback: serve index.html for non-file paths
			path := r.URL.Path
			if path != "/" {
				// Try to serve the file directly
				f, err := sub.Open(strings.TrimPrefix(path, "/"))
				if err == nil {
					f.Close()
					fileServer.ServeHTTP(w, r)
					return
				}
			}
			// Serve index.html for SPA routes
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})
	}
}

// --- PID file management ---

// PidFilePath returns the path for the PID file.
func PidFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".orbit", "ui.pid")
}

// WritePidFile writes the current process PID and address to the PID file.
func WritePidFile(pid int, addr string) error {
	content := fmt.Sprintf("%d\n%s\n", pid, addr)
	return os.WriteFile(PidFilePath(), []byte(content), 0644)
}

// ReadPidFile reads the PID and address from the PID file.
func ReadPidFile() (pid int, addr string, err error) {
	data, err := os.ReadFile(PidFilePath())
	if err != nil {
		return 0, "", err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return 0, "", fmt.Errorf("invalid pid file format")
	}
	pid, err = strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, "", fmt.Errorf("invalid pid: %w", err)
	}
	addr = strings.TrimSpace(lines[1])
	return pid, addr, nil
}

// RemovePidFile removes the PID file.
func RemovePidFile() error {
	return os.Remove(PidFilePath())
}

// LogFilePath returns the path for the UI server log file.
func LogFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".orbit", "ui.log")
}

// OpenLogFile opens the log file for appending.
func OpenLogFile() (*os.File, error) {
	return os.OpenFile(LogFilePath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

// --- Middleware ---

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// resolveProjectID finds a project entity_id from a stable_id prefix.
func (s *Server) resolveProjectID(stableIDPrefix string) (int64, error) {
	var entityID int64
	err := s.db.Conn().QueryRow(
		"SELECT entity_id FROM p_projects WHERE stable_id LIKE ?",
		stableIDPrefix+"%",
	).Scan(&entityID)
	if err != nil {
		return 0, fmt.Errorf("project %q not found", stableIDPrefix)
	}
	return entityID, nil
}

// resolveBranchID resolves a branch from query param or defaults to main.
func (s *Server) resolveBranchID(r *http.Request, projectEntityID int64) (int64, error) {
	branchName := r.URL.Query().Get("branch")
	if branchName == "" || branchName == "main" {
		return s.projects.GetMainBranch(r.Context(), projectEntityID)
	}
	var branchID int64
	err := s.db.Conn().QueryRow(
		"SELECT entity_id FROM p_branches WHERE project_id = ? AND name = ?",
		projectEntityID, branchName,
	).Scan(&branchID)
	if err != nil {
		return 0, fmt.Errorf("branch %q not found", branchName)
	}
	return branchID, nil
}
