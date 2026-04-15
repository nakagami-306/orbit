package eavt

import (
	"database/sql"
	"fmt"
)

// BeginTx creates a new EAVT transaction record and returns its ID.
// decisionID can be nil for system operations (e.g., project creation).
func BeginTx(sqlTx *sql.Tx, decisionID *int64, branchID int64, author string) (int64, error) {
	res, err := sqlTx.Exec(
		"INSERT INTO transactions (decision_id, branch_id, author) VALUES (?, ?, ?)",
		decisionID, branchID, author,
	)
	if err != nil {
		return 0, fmt.Errorf("begin eavt tx: %w", err)
	}
	txID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get tx id: %w", err)
	}
	return txID, nil
}

// AssertDatom writes an assert (op=1) datom to the store.
func AssertDatom(sqlTx *sql.Tx, entityID int64, attr string, val Value, txID int64) error {
	encoded, err := val.Encode()
	if err != nil {
		return fmt.Errorf("assert datom: %w", err)
	}
	_, err = sqlTx.Exec(
		"INSERT INTO datoms (e, a, v, tx, op) VALUES (?, ?, ?, ?, 1)",
		entityID, attr, encoded, txID,
	)
	if err != nil {
		return fmt.Errorf("assert datom [e=%d a=%s]: %w", entityID, attr, err)
	}
	return nil
}

// RetractDatom writes a retract (op=0) datom to the store.
func RetractDatom(sqlTx *sql.Tx, entityID int64, attr string, val Value, txID int64) error {
	encoded, err := val.Encode()
	if err != nil {
		return fmt.Errorf("retract datom: %w", err)
	}
	_, err = sqlTx.Exec(
		"INSERT INTO datoms (e, a, v, tx, op) VALUES (?, ?, ?, ?, 0)",
		entityID, attr, encoded, txID,
	)
	if err != nil {
		return fmt.Errorf("retract datom [e=%d a=%s]: %w", entityID, attr, err)
	}
	return nil
}

// GetTxInstant returns the timestamp of a transaction.
func GetTxInstant(sqlTx *sql.Tx, txID int64) (string, error) {
	var instant string
	err := sqlTx.QueryRow("SELECT instant FROM transactions WHERE id = ?", txID).Scan(&instant)
	if err != nil {
		return "", fmt.Errorf("get tx instant: %w", err)
	}
	return instant, nil
}
