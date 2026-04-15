package eavt

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// EntityType identifies the kind of entity.
type EntityType string

const (
	EntityProject   EntityType = "project"
	EntitySection   EntityType = "section"
	EntityDecision  EntityType = "decision"
	EntityThread    EntityType = "thread"
	EntityEntry     EntityType = "entry"
	EntityTask      EntityType = "task"
	EntityMilestone EntityType = "milestone"
	EntityBranch    EntityType = "branch"
)

// NewStableID generates a new UUID v7 for use as a stable entity identifier.
func NewStableID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// CreateEntity inserts a new entity and returns its internal auto-increment ID.
func CreateEntity(tx *sql.Tx, stableID string, entityType EntityType, txID int64) (int64, error) {
	res, err := tx.Exec(
		"INSERT INTO entities (stable_id, entity_type, created_tx) VALUES (?, ?, ?)",
		stableID, string(entityType), txID,
	)
	if err != nil {
		return 0, fmt.Errorf("create entity: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get entity id: %w", err)
	}
	return id, nil
}

// GetEntityByStableID looks up an entity by its stable UUID.
func GetEntityByStableID(tx *sql.Tx, stableID string) (int64, EntityType, error) {
	var id int64
	var et string
	err := tx.QueryRow(
		"SELECT id, entity_type FROM entities WHERE stable_id = ?", stableID,
	).Scan(&id, &et)
	if err != nil {
		return 0, "", fmt.Errorf("get entity by stable_id %q: %w", stableID, err)
	}
	return id, EntityType(et), nil
}

// GetStableID returns the stable UUID for an internal entity ID.
func GetStableID(tx *sql.Tx, internalID int64) (string, error) {
	var sid string
	err := tx.QueryRow("SELECT stable_id FROM entities WHERE id = ?", internalID).Scan(&sid)
	if err != nil {
		return "", fmt.Errorf("get stable_id for %d: %w", internalID, err)
	}
	return sid, nil
}
