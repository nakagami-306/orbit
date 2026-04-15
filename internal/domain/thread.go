package domain

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/eavt"
	"github.com/nakagami-306/orbit/internal/projection"
)

// Thread represents a thread from projections.
type Thread struct {
	EntityID          int64
	StableID          string
	ProjectID         int64
	Title             string
	Question          string
	Status            string
	OutcomeDecisionID *int64
}

// Entry represents a thread entry from projections.
type Entry struct {
	EntityID    int64
	StableID    string
	ThreadID    int64
	Type        string
	Content     string
	TargetID    *int64
	Stance      string
	Author      string
	IsRetracted bool
	Instant     string
}

// ThreadService handles thread and entry operations.
type ThreadService struct {
	DB        *db.DB
	Projector *projection.Projector
}

// CreateThread creates a new thread.
func (s *ThreadService) CreateThread(ctx context.Context, projectEntityID int64, title, question, author string) (threadStableID string, err error) {
	threadStableID = eavt.NewStableID()

	err = s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		// Get main branch for the project
		var branchID int64
		if err := sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectEntityID).Scan(&branchID); err != nil {
			return err
		}

		txID, err := eavt.BeginTx(sqlTx, nil, branchID, author)
		if err != nil {
			return err
		}

		threadID, err := eavt.CreateEntity(sqlTx, threadStableID, eavt.EntityThread, txID)
		if err != nil {
			return err
		}

		eavt.AssertDatom(sqlTx, threadID, eavt.AttrThreadTitle, eavt.NewString(title), txID)
		eavt.AssertDatom(sqlTx, threadID, eavt.AttrThreadQuestion, eavt.NewString(question), txID)
		eavt.AssertDatom(sqlTx, threadID, eavt.AttrThreadStatus, eavt.NewEnum("open"), txID)
		eavt.AssertDatom(sqlTx, threadID, eavt.AttrThreadProjectID, eavt.NewRef(projectEntityID), txID)

		return s.Projector.ApplyDatoms(sqlTx, threadID, eavt.EntityThread, branchID)
	})
	return
}

// AddEntry adds an entry to a thread.
func (s *ThreadService) AddEntry(ctx context.Context, threadEntityID int64, entryType, content, author string, targetID *int64, stance string) (entryStableID string, err error) {
	entryStableID = eavt.NewStableID()

	err = s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		// Get thread's project and branch
		var projectID, branchID int64
		sqlTx.QueryRow("SELECT project_id FROM p_threads WHERE entity_id = ?", threadEntityID).Scan(&projectID)
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectID).Scan(&branchID)

		txID, err := eavt.BeginTx(sqlTx, nil, branchID, author)
		if err != nil {
			return err
		}

		entryID, err := eavt.CreateEntity(sqlTx, entryStableID, eavt.EntityEntry, txID)
		if err != nil {
			return err
		}

		eavt.AssertDatom(sqlTx, entryID, eavt.AttrEntryThreadID, eavt.NewRef(threadEntityID), txID)
		eavt.AssertDatom(sqlTx, entryID, eavt.AttrEntryType, eavt.NewEnum(entryType), txID)
		eavt.AssertDatom(sqlTx, entryID, eavt.AttrEntryContent, eavt.NewString(content), txID)
		eavt.AssertDatom(sqlTx, entryID, eavt.AttrEntryAuthor, eavt.NewString(author), txID)
		if targetID != nil {
			eavt.AssertDatom(sqlTx, entryID, eavt.AttrEntryTargetID, eavt.NewRef(*targetID), txID)
		}
		if stance != "" {
			eavt.AssertDatom(sqlTx, entryID, eavt.AttrEntryStance, eavt.NewEnum(stance), txID)
		}

		return s.Projector.ApplyDatoms(sqlTx, entryID, eavt.EntityEntry, branchID)
	})
	return
}

// CloseThread sets a thread's status to abandoned.
func (s *ThreadService) CloseThread(ctx context.Context, threadEntityID int64, reason, author string) error {
	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var projectID, branchID int64
		sqlTx.QueryRow("SELECT project_id FROM p_threads WHERE entity_id = ?", threadEntityID).Scan(&projectID)
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectID).Scan(&branchID)

		txID, err := eavt.BeginTx(sqlTx, nil, branchID, author)
		if err != nil {
			return err
		}

		eavt.RetractDatom(sqlTx, threadEntityID, eavt.AttrThreadStatus, eavt.NewEnum("open"), txID)
		eavt.AssertDatom(sqlTx, threadEntityID, eavt.AttrThreadStatus, eavt.NewEnum("abandoned"), txID)

		return s.Projector.ApplyDatoms(sqlTx, threadEntityID, eavt.EntityThread, branchID)
	})
}

// Decide converges a thread into a decision and updates state.
func (s *ThreadService) Decide(ctx context.Context, threadEntityID int64, projectEntityID, branchID int64, sectionEntityID int64, newContent, decisionTitle, rationale, author string) (decisionStableID string, err error) {
	decisionStableID = eavt.NewStableID()

	err = s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		// Get current head
		var currentHead *int64
		var headID int64
		if err := sqlTx.QueryRow("SELECT head_decision_id FROM p_branches WHERE entity_id = ?", branchID).Scan(&headID); err == nil && headID > 0 {
			currentHead = &headID
		}

		// Create decision
		decisionEntityID, err := eavt.CreateEntity(sqlTx, decisionStableID, eavt.EntityDecision, 0)
		if err != nil {
			return err
		}

		txID, err := eavt.BeginTx(sqlTx, &decisionEntityID, branchID, author)
		if err != nil {
			return err
		}

		// Update section content if provided
		if sectionEntityID > 0 && newContent != "" {
			state, _ := eavt.EntityState(sqlTx, sectionEntityID)
			if oldContent, ok := state[eavt.AttrSectionContent]; ok {
				eavt.RetractDatom(sqlTx, sectionEntityID, eavt.AttrSectionContent, oldContent, txID)
			}
			eavt.AssertDatom(sqlTx, sectionEntityID, eavt.AttrSectionContent, eavt.NewString(newContent), txID)
		}

		// Decision datoms
		eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionTitle, eavt.NewString(decisionTitle), txID)
		eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionRationale, eavt.NewString(rationale), txID)
		eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionAuthor, eavt.NewString(author), txID)
		eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionProjectID, eavt.NewRef(projectEntityID), txID)
		eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionSourceThread, eavt.NewRef(threadEntityID), txID)
		if currentHead != nil {
			eavt.AssertDatom(sqlTx, decisionEntityID, eavt.AttrDecisionParents, eavt.NewRefSet([]int64{*currentHead}), txID)
		}

		// Update thread status to decided
		eavt.RetractDatom(sqlTx, threadEntityID, eavt.AttrThreadStatus, eavt.NewEnum("open"), txID)
		eavt.AssertDatom(sqlTx, threadEntityID, eavt.AttrThreadStatus, eavt.NewEnum("decided"), txID)
		eavt.AssertDatom(sqlTx, threadEntityID, eavt.AttrThreadOutcomeDecision, eavt.NewRef(decisionEntityID), txID)

		// Update branch head
		if currentHead != nil {
			eavt.RetractDatom(sqlTx, branchID, eavt.AttrBranchHeadDecision, eavt.NewRef(*currentHead), txID)
		}
		eavt.AssertDatom(sqlTx, branchID, eavt.AttrBranchHeadDecision, eavt.NewRef(decisionEntityID), txID)

		// Apply projections
		if sectionEntityID > 0 {
			s.Projector.ApplyDatoms(sqlTx, sectionEntityID, eavt.EntitySection, branchID)
		}
		s.Projector.ApplyDatoms(sqlTx, decisionEntityID, eavt.EntityDecision, branchID)
		s.Projector.ApplyDatoms(sqlTx, threadEntityID, eavt.EntityThread, branchID)
		s.Projector.ApplyDatoms(sqlTx, branchID, eavt.EntityBranch, branchID)

		if sectionEntityID > 0 {
			s.Projector.MarkStale(sqlTx, sectionEntityID, branchID, decisionEntityID)
		}

		return nil
	})
	return
}

// GetThread returns a thread by entity ID.
func (s *ThreadService) GetThread(ctx context.Context, entityID int64) (*Thread, error) {
	var t Thread
	var outcomeID sql.NullInt64
	err := s.DB.Conn().QueryRowContext(ctx,
		"SELECT entity_id, stable_id, project_id, title, COALESCE(question,''), status, outcome_decision_id FROM p_threads WHERE entity_id = ?",
		entityID,
	).Scan(&t.EntityID, &t.StableID, &t.ProjectID, &t.Title, &t.Question, &t.Status, &outcomeID)
	if err != nil {
		return nil, fmt.Errorf("thread %d not found: %w", entityID, err)
	}
	if outcomeID.Valid {
		t.OutcomeDecisionID = &outcomeID.Int64
	}
	return &t, nil
}

// ListThreads returns threads for a project, optionally filtered by status.
func (s *ThreadService) ListThreads(ctx context.Context, projectEntityID int64, statusFilter string) ([]Thread, error) {
	var rows *sql.Rows
	var err error
	if statusFilter != "" {
		rows, err = s.DB.Conn().QueryContext(ctx,
			"SELECT entity_id, stable_id, project_id, title, COALESCE(question,''), status FROM p_threads WHERE project_id = ? AND status = ?",
			projectEntityID, statusFilter)
	} else {
		rows, err = s.DB.Conn().QueryContext(ctx,
			"SELECT entity_id, stable_id, project_id, title, COALESCE(question,''), status FROM p_threads WHERE project_id = ?",
			projectEntityID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []Thread
	for rows.Next() {
		var t Thread
		if err := rows.Scan(&t.EntityID, &t.StableID, &t.ProjectID, &t.Title, &t.Question, &t.Status); err != nil {
			return nil, err
		}
		threads = append(threads, t)
	}
	return threads, rows.Err()
}

// GetEntries returns all entries for a thread.
func (s *ThreadService) GetEntries(ctx context.Context, threadEntityID int64) ([]Entry, error) {
	rows, err := s.DB.Conn().QueryContext(ctx, `
		SELECT entity_id, stable_id, thread_id, entry_type, COALESCE(content,''),
		       target_id, COALESCE(stance,''), COALESCE(author,''), is_retracted, instant
		FROM p_entries WHERE thread_id = ? ORDER BY entity_id
	`, threadEntityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var targetID sql.NullInt64
		var isRetracted int
		if err := rows.Scan(&e.EntityID, &e.StableID, &e.ThreadID, &e.Type, &e.Content,
			&targetID, &e.Stance, &e.Author, &isRetracted, &e.Instant); err != nil {
			return nil, err
		}
		if targetID.Valid {
			e.TargetID = &targetID.Int64
		}
		e.IsRetracted = isRetracted == 1
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
