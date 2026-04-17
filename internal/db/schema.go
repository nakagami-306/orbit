package db

// SchemaSQL contains the complete DDL for the Orbit database.
// Three layers: event store, projections, workspace.
const SchemaSQL = `
-- =============================================================
-- Layer 1: Event Store (append-only, source of truth)
-- =============================================================

CREATE TABLE entities (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    stable_id   TEXT NOT NULL UNIQUE,
    entity_type TEXT NOT NULL,
    created_tx  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE transactions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    decision_id INTEGER,
    branch_id   INTEGER NOT NULL,
    instant     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    author      TEXT
);

CREATE TABLE datoms (
    e       INTEGER NOT NULL,
    a       TEXT NOT NULL,
    v       TEXT NOT NULL,
    tx      INTEGER NOT NULL,
    op      INTEGER NOT NULL DEFAULT 1,
    FOREIGN KEY (e) REFERENCES entities(id),
    FOREIGN KEY (tx) REFERENCES transactions(id)
);

CREATE INDEX idx_eavt ON datoms (e, a, v, tx);
CREATE INDEX idx_aevt ON datoms (a, e, v, tx);
CREATE INDEX idx_avet ON datoms (a, v, e, tx);
CREATE INDEX idx_txea ON datoms (tx, e, a);

CREATE TABLE operations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    op_type     TEXT NOT NULL,
    tx_id       INTEGER,
    parent_op   INTEGER,
    instant     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    metadata    TEXT,
    FOREIGN KEY (tx_id) REFERENCES transactions(id),
    FOREIGN KEY (parent_op) REFERENCES operations(id)
);

-- Immutability triggers
CREATE TRIGGER prevent_datom_update
BEFORE UPDATE ON datoms
BEGIN
    SELECT RAISE(ABORT, 'datoms table is append-only: UPDATE is prohibited');
END;

CREATE TRIGGER prevent_datom_delete
BEFORE DELETE ON datoms
BEGIN
    SELECT RAISE(ABORT, 'datoms table is append-only: DELETE is prohibited');
END;

CREATE TRIGGER prevent_tx_update
BEFORE UPDATE ON transactions
BEGIN
    SELECT RAISE(ABORT, 'transactions table is immutable: UPDATE is prohibited');
END;

CREATE TRIGGER prevent_tx_delete
BEFORE DELETE ON transactions
BEGIN
    SELECT RAISE(ABORT, 'transactions table is immutable: DELETE is prohibited');
END;

-- =============================================================
-- Layer 2: Projections (materialized views for query performance)
-- =============================================================

CREATE TABLE p_projects (
    entity_id   INTEGER PRIMARY KEY,
    stable_id   TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'active',
    tags        TEXT,
    FOREIGN KEY (entity_id) REFERENCES entities(id)
);

CREATE TABLE p_sections (
    entity_id       INTEGER NOT NULL,
    branch_id       INTEGER NOT NULL,
    stable_id       TEXT NOT NULL,
    project_id      INTEGER NOT NULL,
    title           TEXT NOT NULL,
    content         TEXT,
    position        INTEGER NOT NULL DEFAULT 0,
    is_stale        INTEGER NOT NULL DEFAULT 0,
    stale_reason    TEXT,
    last_decision_id INTEGER,
    PRIMARY KEY (entity_id, branch_id),
    FOREIGN KEY (entity_id) REFERENCES entities(id),
    FOREIGN KEY (branch_id) REFERENCES entities(id),
    FOREIGN KEY (project_id) REFERENCES entities(id)
);

CREATE TABLE p_section_refs (
    from_section INTEGER NOT NULL,
    to_section   INTEGER NOT NULL,
    branch_id    INTEGER NOT NULL,
    PRIMARY KEY (from_section, to_section, branch_id),
    FOREIGN KEY (from_section) REFERENCES entities(id),
    FOREIGN KEY (to_section) REFERENCES entities(id)
);

CREATE TABLE p_decisions (
    entity_id        INTEGER NOT NULL,
    branch_id        INTEGER NOT NULL,
    stable_id        TEXT NOT NULL,
    project_id       INTEGER NOT NULL,
    title            TEXT NOT NULL,
    rationale        TEXT,
    context          TEXT,
    author           TEXT,
    source_thread_id INTEGER,
    source_topic_id  INTEGER,
    tx_id            INTEGER NOT NULL,
    instant          TEXT NOT NULL,
    PRIMARY KEY (entity_id, branch_id),
    FOREIGN KEY (entity_id) REFERENCES entities(id),
    FOREIGN KEY (tx_id) REFERENCES transactions(id)
);

CREATE TABLE p_decision_parents (
    decision_id INTEGER NOT NULL,
    parent_id   INTEGER NOT NULL,
    PRIMARY KEY (decision_id, parent_id),
    FOREIGN KEY (decision_id) REFERENCES entities(id),
    FOREIGN KEY (parent_id) REFERENCES entities(id)
);

CREATE TABLE p_branches (
    entity_id        INTEGER PRIMARY KEY,
    stable_id        TEXT NOT NULL,
    project_id       INTEGER NOT NULL,
    name             TEXT,
    head_decision_id INTEGER,
    status           TEXT NOT NULL DEFAULT 'active',
    is_main          INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (entity_id) REFERENCES entities(id)
);

CREATE TABLE p_threads (
    entity_id            INTEGER PRIMARY KEY,
    stable_id            TEXT NOT NULL,
    project_id           INTEGER NOT NULL,
    title                TEXT NOT NULL,
    question             TEXT,
    status               TEXT NOT NULL DEFAULT 'open',
    outcome_decision_id  INTEGER,
    FOREIGN KEY (entity_id) REFERENCES entities(id)
);

CREATE TABLE p_entries (
    entity_id    INTEGER PRIMARY KEY,
    stable_id    TEXT NOT NULL,
    thread_id    INTEGER NOT NULL,
    entry_type   TEXT NOT NULL,
    content      TEXT,
    target_id    INTEGER,
    stance       TEXT,
    author       TEXT,
    is_retracted INTEGER NOT NULL DEFAULT 0,
    instant      TEXT NOT NULL,
    FOREIGN KEY (entity_id) REFERENCES entities(id),
    FOREIGN KEY (thread_id) REFERENCES entities(id)
);

CREATE TABLE p_tasks (
    entity_id   INTEGER PRIMARY KEY,
    stable_id   TEXT NOT NULL,
    project_id  INTEGER NOT NULL,
    title       TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'todo',
    priority    TEXT DEFAULT 'medium',
    assignee    TEXT,
    source_type TEXT,
    source_id   INTEGER,
    due_date    TEXT,
    tags        TEXT,
    FOREIGN KEY (entity_id) REFERENCES entities(id)
);

CREATE TABLE p_milestones (
    entity_id   INTEGER PRIMARY KEY,
    stable_id   TEXT NOT NULL,
    project_id  INTEGER NOT NULL,
    title       TEXT NOT NULL,
    description TEXT,
    decision_id INTEGER NOT NULL,
    FOREIGN KEY (entity_id) REFERENCES entities(id),
    FOREIGN KEY (decision_id) REFERENCES entities(id)
);

CREATE TABLE p_topics (
    entity_id            INTEGER PRIMARY KEY,
    stable_id            TEXT NOT NULL,
    project_id           INTEGER NOT NULL,
    title                TEXT NOT NULL,
    description          TEXT,
    status               TEXT NOT NULL DEFAULT 'open',
    outcome_decision_id  INTEGER,
    FOREIGN KEY (entity_id) REFERENCES entities(id)
);

CREATE TABLE topic_threads (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id  INTEGER NOT NULL,
    thread_id INTEGER NOT NULL,
    UNIQUE(topic_id, thread_id),
    FOREIGN KEY (topic_id) REFERENCES p_topics(entity_id),
    FOREIGN KEY (thread_id) REFERENCES p_threads(entity_id)
);

CREATE TABLE p_conflicts (
    entity_id              INTEGER PRIMARY KEY,
    stable_id              TEXT NOT NULL,
    project_id             INTEGER NOT NULL,
    branch_id              INTEGER NOT NULL,
    section_id             INTEGER NOT NULL,
    field                  TEXT NOT NULL DEFAULT 'content',
    base_value             TEXT,
    merge_decision_id      INTEGER NOT NULL,
    status                 TEXT NOT NULL DEFAULT 'unresolved',
    resolution             TEXT,
    resolution_rationale   TEXT,
    resolution_decision_id INTEGER,
    FOREIGN KEY (entity_id) REFERENCES entities(id),
    FOREIGN KEY (section_id) REFERENCES entities(id)
);

CREATE TABLE p_conflict_sides (
    conflict_id INTEGER NOT NULL,
    branch_id   INTEGER NOT NULL,
    value       TEXT,
    PRIMARY KEY (conflict_id, branch_id),
    FOREIGN KEY (conflict_id) REFERENCES entities(id)
);

-- =============================================================
-- Layer 3: Workspace (directory <-> project mapping)
-- =============================================================

CREATE TABLE workspaces (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id        INTEGER NOT NULL,
    path              TEXT NOT NULL UNIQUE,
    current_branch_id INTEGER NOT NULL,
    state_hash        TEXT,
    FOREIGN KEY (project_id) REFERENCES entities(id),
    FOREIGN KEY (current_branch_id) REFERENCES entities(id)
);
`
