package projection

import (
	"database/sql"
	"fmt"

	"github.com/nakagami-306/orbit/internal/eavt"
)

// Projector updates materialized view tables from EAVT datoms.
type Projector struct{}

// ApplyDatoms updates the appropriate p_* table for a given entity.
// Called within the same SQL transaction as the datom writes.
func (p *Projector) ApplyDatoms(sqlTx *sql.Tx, entityID int64, entityType eavt.EntityType, branchID int64) error {
	state, err := eavt.EntityState(sqlTx, entityID)
	if err != nil {
		return fmt.Errorf("get entity state for projection: %w", err)
	}

	stableID, err := eavt.GetStableID(sqlTx, entityID)
	if err != nil {
		return fmt.Errorf("get stable_id for projection: %w", err)
	}

	switch entityType {
	case eavt.EntityProject:
		return p.applyProject(sqlTx, entityID, stableID, state)
	case eavt.EntitySection:
		return p.applySection(sqlTx, entityID, stableID, branchID, state)
	case eavt.EntityDecision:
		return p.applyDecision(sqlTx, entityID, stableID, branchID, state)
	case eavt.EntityBranch:
		return p.applyBranch(sqlTx, entityID, stableID, state)
	case eavt.EntityThread:
		return p.applyThread(sqlTx, entityID, stableID, state)
	case eavt.EntityEntry:
		return p.applyEntry(sqlTx, entityID, stableID, state)
	case eavt.EntityTask:
		return p.applyTask(sqlTx, entityID, stableID, state)
	case eavt.EntityMilestone:
		return p.applyMilestone(sqlTx, entityID, stableID, state)
	case eavt.EntityTopic:
		return p.applyTopic(sqlTx, entityID, stableID, state)
	case eavt.EntityTopicThread:
		return p.applyTopicThread(sqlTx, entityID, state)
	default:
		return fmt.Errorf("unknown entity type: %s", entityType)
	}
}

func (p *Projector) applyProject(sqlTx *sql.Tx, entityID int64, stableID string, state map[string]eavt.Value) error {
	name, _ := state[eavt.AttrProjectName].AsString()
	desc := valStr(state, eavt.AttrProjectDescription)
	status := valStr(state, eavt.AttrProjectStatus)
	if status == "" {
		status = "active"
	}

	_, err := sqlTx.Exec(`
		INSERT INTO p_projects (entity_id, stable_id, name, description, status)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(entity_id) DO UPDATE SET
			name=excluded.name, description=excluded.description, status=excluded.status
	`, entityID, stableID, name, desc, status)
	return err
}

func (p *Projector) applySection(sqlTx *sql.Tx, entityID int64, stableID string, branchID int64, state map[string]eavt.Value) error {
	title := valStr(state, eavt.AttrSectionTitle)
	content := valStr(state, eavt.AttrSectionContent)
	position := valInt(state, eavt.AttrSectionPosition)
	projectID := valInt(state, eavt.AttrSectionProjectID)

	_, err := sqlTx.Exec(`
		INSERT INTO p_sections (entity_id, branch_id, stable_id, project_id, title, content, position)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_id, branch_id) DO UPDATE SET
			title=excluded.title, content=excluded.content, position=excluded.position
	`, entityID, branchID, stableID, projectID, title, content, position)
	return err
}

// MarkStale marks sections that reference the changed section as stale.
func (p *Projector) MarkStale(sqlTx *sql.Tx, changedSectionID int64, branchID int64, decisionID int64) error {
	decStableID, _ := eavt.GetStableID(sqlTx, decisionID)
	changedStableID, _ := eavt.GetStableID(sqlTx, changedSectionID)
	staleReason := fmt.Sprintf(`{"decision":"%s","changed_section":"%s"}`, decStableID, changedStableID)

	_, err := sqlTx.Exec(`
		UPDATE p_sections SET is_stale = 1, stale_reason = ?
		WHERE branch_id = ? AND entity_id IN (
			SELECT from_section FROM p_section_refs
			WHERE to_section = ? AND branch_id = ?
		)
	`, staleReason, branchID, changedSectionID, branchID)
	return err
}

func (p *Projector) applyDecision(sqlTx *sql.Tx, entityID int64, stableID string, branchID int64, state map[string]eavt.Value) error {
	title := valStr(state, eavt.AttrDecisionTitle)
	rationale := valStr(state, eavt.AttrDecisionRationale)
	ctx := valStr(state, eavt.AttrDecisionContext)
	author := valStr(state, eavt.AttrDecisionAuthor)
	projectID := valInt(state, eavt.AttrDecisionProjectID)
	sourceThread := valIntPtr(state, eavt.AttrDecisionSourceThread)
	sourceTopic := valIntPtr(state, eavt.AttrDecisionSourceTopic)

	// Get the tx_id and instant for this decision
	var txID int64
	var instant string
	err := sqlTx.QueryRow(
		"SELECT id, instant FROM transactions WHERE decision_id = ? ORDER BY id DESC LIMIT 1",
		entityID,
	).Scan(&txID, &instant)
	if err != nil {
		// Fallback: use latest tx
		sqlTx.QueryRow("SELECT max(id), max(instant) FROM transactions").Scan(&txID, &instant)
	}

	_, err = sqlTx.Exec(`
		INSERT INTO p_decisions (entity_id, branch_id, stable_id, project_id, title, rationale, context, author, source_thread_id, source_topic_id, tx_id, instant)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_id, branch_id) DO UPDATE SET
			title=excluded.title, rationale=excluded.rationale, context=excluded.context,
			author=excluded.author, source_thread_id=excluded.source_thread_id,
			source_topic_id=excluded.source_topic_id, tx_id=excluded.tx_id, instant=excluded.instant
	`, entityID, branchID, stableID, projectID, title, rationale, ctx, author, sourceThread, sourceTopic, txID, instant)
	if err != nil {
		return err
	}

	// Update parent links
	if parents, ok := state[eavt.AttrDecisionParents]; ok {
		if ids, err := parents.AsRefSet(); err == nil {
			for _, parentID := range ids {
				sqlTx.Exec(`
					INSERT OR IGNORE INTO p_decision_parents (decision_id, parent_id) VALUES (?, ?)
				`, entityID, parentID)
			}
		}
	}

	return nil
}

func (p *Projector) applyBranch(sqlTx *sql.Tx, entityID int64, stableID string, state map[string]eavt.Value) error {
	name := valStr(state, eavt.AttrBranchName)
	projectID := valInt(state, eavt.AttrBranchProjectID)
	status := valStr(state, eavt.AttrBranchStatus)
	if status == "" {
		status = "active"
	}
	isMain := valBool(state, eavt.AttrBranchIsMain)
	headDecision := valIntPtr(state, eavt.AttrBranchHeadDecision)

	isMainInt := 0
	if isMain {
		isMainInt = 1
	}

	_, err := sqlTx.Exec(`
		INSERT INTO p_branches (entity_id, stable_id, project_id, name, head_decision_id, status, is_main)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_id) DO UPDATE SET
			name=excluded.name, head_decision_id=excluded.head_decision_id,
			status=excluded.status, is_main=excluded.is_main
	`, entityID, stableID, projectID, name, headDecision, status, isMainInt)
	return err
}

func (p *Projector) applyThread(sqlTx *sql.Tx, entityID int64, stableID string, state map[string]eavt.Value) error {
	title := valStr(state, eavt.AttrThreadTitle)
	question := valStr(state, eavt.AttrThreadQuestion)
	status := valStr(state, eavt.AttrThreadStatus)
	if status == "" {
		status = "open"
	}
	projectID := valInt(state, eavt.AttrThreadProjectID)
	outcomeDecision := valIntPtr(state, eavt.AttrThreadOutcomeDecision)

	_, err := sqlTx.Exec(`
		INSERT INTO p_threads (entity_id, stable_id, project_id, title, question, status, outcome_decision_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_id) DO UPDATE SET
			title=excluded.title, question=excluded.question, status=excluded.status,
			outcome_decision_id=excluded.outcome_decision_id
	`, entityID, stableID, projectID, title, question, status, outcomeDecision)
	return err
}

func (p *Projector) applyEntry(sqlTx *sql.Tx, entityID int64, stableID string, state map[string]eavt.Value) error {
	threadID := valInt(state, eavt.AttrEntryThreadID)
	entryType := valStr(state, eavt.AttrEntryType)
	content := valStr(state, eavt.AttrEntryContent)
	author := valStr(state, eavt.AttrEntryAuthor)
	targetID := valIntPtr(state, eavt.AttrEntryTargetID)
	stance := valStr(state, eavt.AttrEntryStance)
	isRetracted := valBool(state, eavt.AttrEntryIsRetracted)

	isRetractedInt := 0
	if isRetracted {
		isRetractedInt = 1
	}

	// Get creation instant from the entity's created_tx
	var instant string
	sqlTx.QueryRow("SELECT instant FROM transactions WHERE id = (SELECT created_tx FROM entities WHERE id = ?)", entityID).Scan(&instant)
	if instant == "" {
		instant = "unknown"
	}

	_, err := sqlTx.Exec(`
		INSERT INTO p_entries (entity_id, stable_id, thread_id, entry_type, content, target_id, stance, author, is_retracted, instant)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_id) DO UPDATE SET
			content=excluded.content, is_retracted=excluded.is_retracted
	`, entityID, stableID, threadID, entryType, content, targetID, stance, author, isRetractedInt, instant)
	return err
}

func (p *Projector) applyTask(sqlTx *sql.Tx, entityID int64, stableID string, state map[string]eavt.Value) error {
	title := valStr(state, eavt.AttrTaskTitle)
	desc := valStr(state, eavt.AttrTaskDescription)
	status := valStr(state, eavt.AttrTaskStatus)
	if status == "" {
		status = "todo"
	}
	priority := valStr(state, eavt.AttrTaskPriority)
	if priority == "" {
		priority = "medium"
	}
	assignee := valStr(state, eavt.AttrTaskAssignee)
	projectID := valInt(state, eavt.AttrTaskProjectID)
	sourceType := valStr(state, eavt.AttrTaskSourceType)
	sourceID := valIntPtr(state, eavt.AttrTaskSourceID)

	_, err := sqlTx.Exec(`
		INSERT INTO p_tasks (entity_id, stable_id, project_id, title, description, status, priority, assignee, source_type, source_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_id) DO UPDATE SET
			title=excluded.title, description=excluded.description, status=excluded.status,
			priority=excluded.priority, assignee=excluded.assignee
	`, entityID, stableID, projectID, title, desc, status, priority, assignee, sourceType, sourceID)
	return err
}

func (p *Projector) applyMilestone(sqlTx *sql.Tx, entityID int64, stableID string, state map[string]eavt.Value) error {
	title := valStr(state, eavt.AttrMilestoneTitle)
	desc := valStr(state, eavt.AttrMilestoneDescription)
	projectID := valInt(state, eavt.AttrMilestoneProjectID)
	decisionID := valInt(state, eavt.AttrMilestoneDecisionID)

	_, err := sqlTx.Exec(`
		INSERT INTO p_milestones (entity_id, stable_id, project_id, title, description, decision_id)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_id) DO UPDATE SET
			title=excluded.title, description=excluded.description, decision_id=excluded.decision_id
	`, entityID, stableID, projectID, title, desc, decisionID)
	return err
}

func (p *Projector) applyTopic(sqlTx *sql.Tx, entityID int64, stableID string, state map[string]eavt.Value) error {
	title := valStr(state, eavt.AttrTopicTitle)
	desc := valStr(state, eavt.AttrTopicDescription)
	status := valStr(state, eavt.AttrTopicStatus)
	if status == "" {
		status = "open"
	}
	projectID := valInt(state, eavt.AttrTopicProjectID)
	outcomeDecision := valIntPtr(state, eavt.AttrTopicOutcomeDecision)

	_, err := sqlTx.Exec(`
		INSERT INTO p_topics (entity_id, stable_id, project_id, title, description, status, outcome_decision_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_id) DO UPDATE SET
			title=excluded.title, description=excluded.description, status=excluded.status,
			outcome_decision_id=excluded.outcome_decision_id
	`, entityID, stableID, projectID, title, desc, status, outcomeDecision)
	return err
}

func (p *Projector) applyTopicThread(sqlTx *sql.Tx, entityID int64, state map[string]eavt.Value) error {
	topicID := valInt(state, eavt.AttrTopicThreadTopicID)
	threadID := valInt(state, eavt.AttrTopicThreadThreadID)

	_, err := sqlTx.Exec(`
		INSERT OR IGNORE INTO topic_threads (topic_id, thread_id)
		VALUES (?, ?)
	`, topicID, threadID)
	return err
}

// Helper functions to extract typed values from state maps

func valStr(state map[string]eavt.Value, attr string) string {
	if v, ok := state[attr]; ok {
		s, _ := v.AsString()
		return s
	}
	return ""
}

func valInt(state map[string]eavt.Value, attr string) int64 {
	if v, ok := state[attr]; ok {
		i, _ := v.AsInt64()
		return i
	}
	return 0
}

func valBool(state map[string]eavt.Value, attr string) bool {
	if v, ok := state[attr]; ok {
		b, _ := v.AsBool()
		return b
	}
	return false
}

func valIntPtr(state map[string]eavt.Value, attr string) *int64 {
	if v, ok := state[attr]; ok {
		i, err := v.AsInt64()
		if err == nil {
			return &i
		}
	}
	return nil
}
