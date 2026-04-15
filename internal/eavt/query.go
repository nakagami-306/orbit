package eavt

import (
	"database/sql"
	"fmt"
)

// EntityState returns the current attribute values for an entity.
// It finds the latest asserted value for each attribute that has not been retracted.
func EntityState(sqlTx *sql.Tx, entityID int64) (map[string]Value, error) {
	return entityStateUpTo(sqlTx, entityID, 0)
}

// EntityStateAsOf returns attribute values for an entity as of a given transaction.
func EntityStateAsOf(sqlTx *sql.Tx, entityID int64, asOfTx int64) (map[string]Value, error) {
	return entityStateUpTo(sqlTx, entityID, asOfTx)
}

func entityStateUpTo(sqlTx *sql.Tx, entityID int64, asOfTx int64) (map[string]Value, error) {
	var query string
	var args []any

	if asOfTx > 0 {
		// Time-travel query: only consider datoms with tx <= asOfTx
		query = `
			WITH ranked AS (
				SELECT a, v, tx, op,
					ROW_NUMBER() OVER (PARTITION BY a ORDER BY tx DESC) as rn
				FROM datoms
				WHERE e = ? AND tx <= ?
			)
			SELECT a, v FROM ranked WHERE rn = 1 AND op = 1
		`
		args = []any{entityID, asOfTx}
	} else {
		// Current state: latest datom per attribute
		query = `
			WITH ranked AS (
				SELECT a, v, tx, op,
					ROW_NUMBER() OVER (PARTITION BY a ORDER BY tx DESC) as rn
				FROM datoms
				WHERE e = ?
			)
			SELECT a, v FROM ranked WHERE rn = 1 AND op = 1
		`
		args = []any{entityID}
	}

	rows, err := sqlTx.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("entity state query: %w", err)
	}
	defer rows.Close()

	state := make(map[string]Value)
	for rows.Next() {
		var attr, encoded string
		if err := rows.Scan(&attr, &encoded); err != nil {
			return nil, fmt.Errorf("scan entity state: %w", err)
		}
		val, err := DecodeValue(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode value for attr %q: %w", attr, err)
		}
		state[attr] = val
	}
	return state, rows.Err()
}

// EntitiesByAttribute finds all entity IDs that currently have the given attribute=value.
func EntitiesByAttribute(sqlTx *sql.Tx, attr string, val Value) ([]int64, error) {
	encoded, err := val.Encode()
	if err != nil {
		return nil, err
	}

	query := `
		WITH ranked AS (
			SELECT e, tx, op,
				ROW_NUMBER() OVER (PARTITION BY e ORDER BY tx DESC) as rn
			FROM datoms
			WHERE a = ? AND v = ?
		)
		SELECT e FROM ranked WHERE rn = 1 AND op = 1
	`
	rows, err := sqlTx.Query(query, attr, encoded)
	if err != nil {
		return nil, fmt.Errorf("entities by attribute: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DatomsForTx returns all datoms written in a specific transaction.
func DatomsForTx(sqlTx *sql.Tx, txID int64) ([]Datom, error) {
	rows, err := sqlTx.Query(
		"SELECT e, a, v, tx, op FROM datoms WHERE tx = ? ORDER BY rowid",
		txID,
	)
	if err != nil {
		return nil, fmt.Errorf("datoms for tx: %w", err)
	}
	defer rows.Close()
	return scanDatoms(rows)
}

// DatomsForEntity returns all datoms (assert + retract) for an entity in tx order.
func DatomsForEntity(sqlTx *sql.Tx, entityID int64) ([]Datom, error) {
	rows, err := sqlTx.Query(
		"SELECT e, a, v, tx, op FROM datoms WHERE e = ? ORDER BY tx, rowid",
		entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("datoms for entity: %w", err)
	}
	defer rows.Close()
	return scanDatoms(rows)
}

func scanDatoms(rows *sql.Rows) ([]Datom, error) {
	var datoms []Datom
	for rows.Next() {
		var d Datom
		var encoded string
		var op int
		if err := rows.Scan(&d.E, &d.A, &encoded, &d.Tx, &op); err != nil {
			return nil, err
		}
		val, err := DecodeValue(encoded)
		if err != nil {
			return nil, err
		}
		d.V = val
		d.Op = Op(op)
		datoms = append(datoms, d)
	}
	return datoms, rows.Err()
}
