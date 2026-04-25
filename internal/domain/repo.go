package domain

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/eavt"
	"github.com/nakagami-306/orbit/internal/projection"
)

// Repo is the projection-shaped view of a repository entity.
type Repo struct {
	EntityID  int64
	StableID  string
	ProjectID int64
	UUID      string
	RemoteURL string
}

// RepoService manages the (project ↔ repo) relationship and Repo entities.
type RepoService struct {
	DB        *db.DB
	Projector *projection.Projector
}

// EnsureRepo guarantees that a Repo entity exists for the given project.
// Idempotent: if one already exists, it returns it; otherwise creates one.
// remoteURL is recorded as an observation but is not the identity key.
func (s *RepoService) EnsureRepo(ctx context.Context, projectEntityID int64, remoteURL string) (*Repo, error) {
	if existing, err := s.FindRepoByProject(ctx, projectEntityID); err == nil {
		// If remote_url changed, refresh as an observation.
		if existing.RemoteURL != remoteURL {
			if err := s.updateRemoteURL(ctx, existing.EntityID, projectEntityID, existing.RemoteURL, remoteURL); err != nil {
				return nil, err
			}
			existing.RemoteURL = remoteURL
		}
		return existing, nil
	}

	stableID := eavt.NewStableID()
	repoUUID := eavt.NewStableID() // independent UUID for cross-system stability

	err := s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var branchID int64
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectEntityID).Scan(&branchID)

		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "system")
		if err != nil {
			return err
		}
		repoID, err := eavt.CreateEntity(sqlTx, stableID, eavt.EntityRepo, txID)
		if err != nil {
			return err
		}
		eavt.AssertDatom(sqlTx, repoID, eavt.AttrRepoUUID, eavt.NewString(repoUUID), txID)
		eavt.AssertDatom(sqlTx, repoID, eavt.AttrRepoProjectID, eavt.NewRef(projectEntityID), txID)
		if remoteURL != "" {
			eavt.AssertDatom(sqlTx, repoID, eavt.AttrRepoRemoteURL, eavt.NewString(remoteURL), txID)
		}
		return s.Projector.ApplyDatoms(sqlTx, repoID, eavt.EntityRepo, branchID)
	})
	if err != nil {
		return nil, fmt.Errorf("create repo: %w", err)
	}
	return s.FindRepoByProject(ctx, projectEntityID)
}

// FindRepoByProject returns the (single) Repo for a project, or sql.ErrNoRows.
func (s *RepoService) FindRepoByProject(ctx context.Context, projectEntityID int64) (*Repo, error) {
	var r Repo
	err := s.DB.Conn().QueryRowContext(ctx,
		"SELECT entity_id, stable_id, project_id, uuid, COALESCE(remote_url,'') FROM p_repos WHERE project_id = ? ORDER BY entity_id LIMIT 1",
		projectEntityID,
	).Scan(&r.EntityID, &r.StableID, &r.ProjectID, &r.UUID, &r.RemoteURL)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *RepoService) updateRemoteURL(ctx context.Context, repoEntityID, projectEntityID int64, oldURL, newURL string) error {
	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var branchID int64
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectEntityID).Scan(&branchID)
		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "system")
		if err != nil {
			return err
		}
		if oldURL != "" {
			eavt.RetractDatom(sqlTx, repoEntityID, eavt.AttrRepoRemoteURL, eavt.NewString(oldURL), txID)
		}
		if newURL != "" {
			eavt.AssertDatom(sqlTx, repoEntityID, eavt.AttrRepoRemoteURL, eavt.NewString(newURL), txID)
		}
		return s.Projector.ApplyDatoms(sqlTx, repoEntityID, eavt.EntityRepo, branchID)
	})
}
