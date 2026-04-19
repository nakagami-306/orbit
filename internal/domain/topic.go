package domain

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nakagami-306/orbit/internal/db"
	"github.com/nakagami-306/orbit/internal/eavt"
	"github.com/nakagami-306/orbit/internal/projection"
)

// Topic represents a topic from projections.
type Topic struct {
	EntityID          int64
	StableID          string
	ProjectID         int64
	Title             string
	Description       string
	Status            string
	OutcomeDecisionID *int64
}

// TopicService handles topic operations.
type TopicService struct {
	DB        *db.DB
	Projector *projection.Projector
}

// CreateTopic creates a new topic.
func (s *TopicService) CreateTopic(ctx context.Context, projectEntityID int64, title, description string) (stableID string, err error) {
	stableID = eavt.NewStableID()

	err = s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var branchID int64
		if err := sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectEntityID).Scan(&branchID); err != nil {
			return fmt.Errorf("get main branch: %w", err)
		}

		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "user")
		if err != nil {
			return err
		}

		topicID, err := eavt.CreateEntity(sqlTx, stableID, eavt.EntityTopic, txID)
		if err != nil {
			return err
		}

		eavt.AssertDatom(sqlTx, topicID, eavt.AttrTopicTitle, eavt.NewString(title), txID)
		eavt.AssertDatom(sqlTx, topicID, eavt.AttrTopicStatus, eavt.NewEnum("open"), txID)
		eavt.AssertDatom(sqlTx, topicID, eavt.AttrTopicProjectID, eavt.NewRef(projectEntityID), txID)

		if description != "" {
			eavt.AssertDatom(sqlTx, topicID, eavt.AttrTopicDescription, eavt.NewString(description), txID)
		}

		return s.Projector.ApplyDatoms(sqlTx, topicID, eavt.EntityTopic, branchID)
	})
	return
}

// ListTopics returns topics for a project, optionally filtered by status.
func (s *TopicService) ListTopics(ctx context.Context, projectEntityID int64, statusFilter string) ([]Topic, error) {
	query := "SELECT entity_id, stable_id, project_id, title, COALESCE(description,''), status FROM p_topics WHERE project_id = ?"
	args := []any{projectEntityID}

	if statusFilter != "" {
		query += " AND status = ?"
		args = append(args, statusFilter)
	}
	query += " ORDER BY entity_id"

	rows, err := s.DB.Conn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.EntityID, &t.StableID, &t.ProjectID, &t.Title, &t.Description, &t.Status); err != nil {
			return nil, err
		}
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

// GetTopic returns a topic by entity ID.
func (s *TopicService) GetTopic(ctx context.Context, entityID int64) (*Topic, error) {
	var t Topic
	var outcomeID sql.NullInt64
	err := s.DB.Conn().QueryRowContext(ctx,
		"SELECT entity_id, stable_id, project_id, title, COALESCE(description,''), status, outcome_decision_id FROM p_topics WHERE entity_id = ?",
		entityID,
	).Scan(&t.EntityID, &t.StableID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &outcomeID)
	if err != nil {
		return nil, fmt.Errorf("topic %d not found: %w", entityID, err)
	}
	if outcomeID.Valid {
		t.OutcomeDecisionID = &outcomeID.Int64
	}
	return &t, nil
}

// FindTopic finds a topic by stable ID prefix within a project.
func (s *TopicService) FindTopic(ctx context.Context, projectEntityID int64, prefix string) (*Topic, error) {
	var t Topic
	var outcomeID sql.NullInt64
	err := s.DB.Conn().QueryRowContext(ctx,
		"SELECT entity_id, stable_id, project_id, title, COALESCE(description,''), status, outcome_decision_id FROM p_topics WHERE project_id = ? AND stable_id = ?",
		projectEntityID, prefix,
	).Scan(&t.EntityID, &t.StableID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &outcomeID)
	if err != nil {
		return nil, fmt.Errorf("topic %q not found: %w", prefix, err)
	}
	if outcomeID.Valid {
		t.OutcomeDecisionID = &outcomeID.Int64
	}
	return &t, nil
}

// AddThread links a thread to a topic via a topic_thread entity.
func (s *TopicService) AddThread(ctx context.Context, topicEntityID, threadEntityID int64) error {
	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var projectID, branchID int64
		sqlTx.QueryRow("SELECT project_id FROM p_topics WHERE entity_id = ?", topicEntityID).Scan(&projectID)
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectID).Scan(&branchID)

		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "user")
		if err != nil {
			return err
		}

		linkStableID := eavt.NewStableID()
		linkID, err := eavt.CreateEntity(sqlTx, linkStableID, eavt.EntityTopicThread, txID)
		if err != nil {
			return err
		}

		eavt.AssertDatom(sqlTx, linkID, eavt.AttrTopicThreadTopicID, eavt.NewRef(topicEntityID), txID)
		eavt.AssertDatom(sqlTx, linkID, eavt.AttrTopicThreadThreadID, eavt.NewRef(threadEntityID), txID)

		return s.Projector.ApplyDatoms(sqlTx, linkID, eavt.EntityTopicThread, branchID)
	})
}

// RemoveThread removes the link between a topic and a thread.
func (s *TopicService) RemoveThread(ctx context.Context, topicEntityID, threadEntityID int64) error {
	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		_, err := sqlTx.Exec("DELETE FROM topic_threads WHERE topic_id = ? AND thread_id = ?", topicEntityID, threadEntityID)
		return err
	})
}

// UpdateTopic updates a topic's title and/or description.
func (s *TopicService) UpdateTopic(ctx context.Context, topicEntityID int64, newTitle, newDescription string) error {
	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var projectID, branchID int64
		sqlTx.QueryRow("SELECT project_id FROM p_topics WHERE entity_id = ?", topicEntityID).Scan(&projectID)
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectID).Scan(&branchID)

		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "user")
		if err != nil {
			return err
		}

		state, _ := eavt.EntityState(sqlTx, topicEntityID)

		if newTitle != "" {
			if old, ok := state[eavt.AttrTopicTitle]; ok {
				eavt.RetractDatom(sqlTx, topicEntityID, eavt.AttrTopicTitle, old, txID)
			}
			eavt.AssertDatom(sqlTx, topicEntityID, eavt.AttrTopicTitle, eavt.NewString(newTitle), txID)
		}
		if newDescription != "" {
			if old, ok := state[eavt.AttrTopicDescription]; ok {
				eavt.RetractDatom(sqlTx, topicEntityID, eavt.AttrTopicDescription, old, txID)
			}
			eavt.AssertDatom(sqlTx, topicEntityID, eavt.AttrTopicDescription, eavt.NewString(newDescription), txID)
		}

		return s.Projector.ApplyDatoms(sqlTx, topicEntityID, eavt.EntityTopic, branchID)
	})
}

// CloseTopic sets a topic's status to abandoned.
func (s *TopicService) CloseTopic(ctx context.Context, topicEntityID int64, reason string) error {
	return s.DB.Tx(ctx, func(sqlTx *sql.Tx) error {
		var projectID, branchID int64
		sqlTx.QueryRow("SELECT project_id FROM p_topics WHERE entity_id = ?", topicEntityID).Scan(&projectID)
		sqlTx.QueryRow("SELECT entity_id FROM p_branches WHERE project_id = ? AND is_main = 1", projectID).Scan(&branchID)

		txID, err := eavt.BeginTx(sqlTx, nil, branchID, "user")
		if err != nil {
			return err
		}

		eavt.RetractDatom(sqlTx, topicEntityID, eavt.AttrTopicStatus, eavt.NewEnum("open"), txID)
		eavt.AssertDatom(sqlTx, topicEntityID, eavt.AttrTopicStatus, eavt.NewEnum("abandoned"), txID)

		return s.Projector.ApplyDatoms(sqlTx, topicEntityID, eavt.EntityTopic, branchID)
	})
}

// GetTopicThreads returns all threads linked to a topic.
func (s *TopicService) GetTopicThreads(ctx context.Context, topicEntityID int64) ([]Thread, error) {
	rows, err := s.DB.Conn().QueryContext(ctx, `
		SELECT t.entity_id, t.stable_id, t.project_id, t.title, COALESCE(t.question,''), t.status
		FROM p_threads t
		JOIN topic_threads tt ON t.entity_id = tt.thread_id
		WHERE tt.topic_id = ?
		ORDER BY t.entity_id
	`, topicEntityID)
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
