package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/nakagami-306/orbit/internal/eavt"
)

// --- Health ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Projects ---

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.projects.ListProjects(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type projectResp struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	result := make([]projectResp, len(projects))
	for i, p := range projects {
		result[i] = projectResp{
			ID:          p.StableID,
			Name:        p.Name,
			Description: p.Description,
			Status:      p.Status,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	project, err := s.projects.GetProjectByID(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	branchID, err := s.resolveBranchID(r, projectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	conn := s.db.Conn()
	var sectionCount, staleCount, decisionCount, threadCount, conflictCount, taskCount int
	conn.QueryRow("SELECT count(*) FROM p_sections WHERE project_id = ? AND branch_id = ?", projectID, branchID).Scan(&sectionCount)
	conn.QueryRow("SELECT count(*) FROM p_sections WHERE project_id = ? AND branch_id = ? AND is_stale = 1", projectID, branchID).Scan(&staleCount)
	conn.QueryRow("SELECT count(*) FROM p_decisions WHERE project_id = ? AND branch_id = ?", projectID, branchID).Scan(&decisionCount)
	conn.QueryRow("SELECT count(*) FROM p_threads WHERE project_id = ? AND status = 'open'", projectID).Scan(&threadCount)
	conn.QueryRow("SELECT count(*) FROM p_conflicts WHERE project_id = ? AND branch_id = ? AND status = 'unresolved'", projectID, branchID).Scan(&conflictCount)
	conn.QueryRow("SELECT count(*) FROM p_tasks WHERE project_id = ? AND status IN ('todo', 'in-progress')", projectID).Scan(&taskCount)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                   project.StableID,
		"name":                 project.Name,
		"description":          project.Description,
		"status":               project.Status,
		"sections":             sectionCount,
		"staleSections":        staleCount,
		"decisions":            decisionCount,
		"openThreads":          threadCount,
		"unresolvedConflicts":  conflictCount,
		"pendingTasks":         taskCount,
	})
}

// --- DAG ---

func (s *Server) handleGetDAG(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	branchID, err := s.resolveBranchID(r, projectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	conn := s.db.Conn()

	// Nodes: decisions on this branch
	rows, err := conn.Query(`
		SELECT d.stable_id, d.title, COALESCE(d.rationale,''), COALESCE(d.author,''),
		       d.instant, d.source_thread_id
		FROM p_decisions d
		WHERE d.project_id = ? AND d.branch_id = ?
		ORDER BY d.tx_id ASC
	`, projectID, branchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type dagNode struct {
		ID             string  `json:"id"`
		Title          string  `json:"title"`
		Rationale      string  `json:"rationale"`
		Author         string  `json:"author"`
		Instant        string  `json:"instant"`
		SourceThreadID *string `json:"sourceThreadId"`
		Type           string  `json:"type"`
	}
	type dagEdge struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}

	nodes := make([]dagNode, 0)
	stableIDSet := make(map[string]bool) // for identifying entity_id -> stable_id
	entityToStable := make(map[int64]string)

	for rows.Next() {
		var n dagNode
		var sourceThreadID sql.NullInt64
		if err := rows.Scan(&n.ID, &n.Title, &n.Rationale, &n.Author, &n.Instant, &sourceThreadID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if sourceThreadID.Valid {
			var threadStableID string
			conn.QueryRow("SELECT stable_id FROM entities WHERE id = ?", sourceThreadID.Int64).Scan(&threadStableID)
			if threadStableID != "" {
				n.SourceThreadID = &threadStableID
			}
		}
		n.Type = "normal"
		stableIDSet[n.ID] = true
		nodes = append(nodes, n)
	}

	// Build entity_id -> stable_id mapping for decisions in this project
	mapRows, err := conn.Query(
		"SELECT entity_id, stable_id FROM p_decisions WHERE project_id = ? AND branch_id = ?",
		projectID, branchID,
	)
	if err == nil {
		defer mapRows.Close()
		for mapRows.Next() {
			var eid int64
			var sid string
			mapRows.Scan(&eid, &sid)
			entityToStable[eid] = sid
		}
	}

	// Edges: parent relationships
	edges := make([]dagEdge, 0)
	parentCounts := make(map[string]int)

	edgeRows, err := conn.Query(`
		SELECT dp.decision_id, dp.parent_id
		FROM p_decision_parents dp
		JOIN p_decisions d ON dp.decision_id = d.entity_id AND d.branch_id = ?
		WHERE d.project_id = ?
	`, branchID, projectID)
	if err == nil {
		defer edgeRows.Close()
		for edgeRows.Next() {
			var decID, parentID int64
			edgeRows.Scan(&decID, &parentID)
			source := entityToStable[parentID]
			target := entityToStable[decID]
			if source != "" && target != "" {
				edges = append(edges, dagEdge{Source: source, Target: target})
				parentCounts[target]++
			}
		}
	}

	// Classify node types
	for i := range nodes {
		if parentCounts[nodes[i].ID] >= 2 {
			nodes[i].Type = "merge"
		}
	}
	// Mark root nodes (no parents)
	childSet := make(map[string]bool)
	for _, e := range edges {
		childSet[e.Target] = true
	}
	for i := range nodes {
		if !childSet[nodes[i].ID] {
			nodes[i].Type = "root"
		}
	}

	// Branches
	type branchInfo struct {
		ID             string  `json:"id"`
		Name           string  `json:"name"`
		IsMain         bool    `json:"isMain"`
		HeadDecisionID *string `json:"headDecisionId"`
		Status         string  `json:"status"`
	}
	branchList := make([]branchInfo, 0)
	branchRows, err := conn.Query(`
		SELECT b.stable_id, COALESCE(b.name,''), b.is_main, b.head_decision_id, b.status
		FROM p_branches b WHERE b.project_id = ?
	`, projectID)
	if err == nil {
		defer branchRows.Close()
		for branchRows.Next() {
			var b branchInfo
			var isMain int
			var headID sql.NullInt64
			branchRows.Scan(&b.ID, &b.Name, &isMain, &headID, &b.Status)
			b.IsMain = isMain == 1
			if headID.Valid {
				if sid, ok := entityToStable[headID.Int64]; ok {
					b.HeadDecisionID = &sid
				}
			}
			branchList = append(branchList, b)
		}
	}

	// Milestones
	type milestoneInfo struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		DecisionID string `json:"decisionId"`
	}
	milestones := make([]milestoneInfo, 0)
	msRows, err := conn.Query(`
		SELECT m.stable_id, m.title, m.decision_id
		FROM p_milestones m WHERE m.project_id = ?
	`, projectID)
	if err == nil {
		defer msRows.Close()
		for msRows.Next() {
			var m milestoneInfo
			var decID int64
			msRows.Scan(&m.ID, &m.Title, &decID)
			if sid, ok := entityToStable[decID]; ok {
				m.DecisionID = sid
			}
			milestones = append(milestones, m)
		}
	}

	// --- Entity nodes: Threads, Tasks, Sections ---

	type entityNode struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Type    string `json:"type"`
		Status  string `json:"status"`
		Instant string `json:"instant"`
	}

	// Threads
	threadNodes := make([]entityNode, 0)
	threadEntityToStable := make(map[int64]string)
	threadRows, err := conn.Query(`
		SELECT t.entity_id, t.stable_id, t.title, t.status, t.outcome_decision_id,
		       COALESCE((SELECT tx2.instant FROM transactions tx2 WHERE tx2.id = e.created_tx), '') AS instant
		FROM p_threads t
		JOIN entities e ON t.entity_id = e.id
		WHERE t.project_id = ?
	`, projectID)
	if err == nil {
		defer threadRows.Close()
		for threadRows.Next() {
			var eid int64
			var n entityNode
			var outcomeDecID sql.NullInt64
			threadRows.Scan(&eid, &n.ID, &n.Title, &n.Status, &outcomeDecID, &n.Instant)
			n.Type = "thread"
			threadNodes = append(threadNodes, n)
			threadEntityToStable[eid] = n.ID
		}
	}

	// Tasks
	taskNodes := make([]entityNode, 0)
	taskRows2, err := conn.Query(`
		SELECT t.entity_id, t.stable_id, t.title, t.status, COALESCE(t.priority,'medium'),
		       COALESCE((SELECT tx2.instant FROM transactions tx2 WHERE tx2.id = e.created_tx), '') AS instant
		FROM p_tasks t
		JOIN entities e ON t.entity_id = e.id
		WHERE t.project_id = ?
	`, projectID)
	if err == nil {
		defer taskRows2.Close()
		for taskRows2.Next() {
			var eid int64
			var n entityNode
			var priority string
			taskRows2.Scan(&eid, &n.ID, &n.Title, &n.Status, &priority, &n.Instant)
			n.Type = "task"
			// Encode priority into status for display: "todo (high)"
			if priority != "" && priority != "medium" {
				n.Status = n.Status + " (" + priority + ")"
			}
			taskNodes = append(taskNodes, n)
		}
	}

	// Sections
	sectionNodes := make([]entityNode, 0)
	sectionEntityToStable := make(map[int64]string)
	sectionRows, err := conn.Query(`
		SELECT s.entity_id, s.stable_id, s.title, s.is_stale,
		       COALESCE((SELECT tx2.instant FROM transactions tx2 WHERE tx2.id = e.created_tx), '') AS instant
		FROM p_sections s
		JOIN entities e ON s.entity_id = e.id
		WHERE s.project_id = ? AND s.branch_id = ?
	`, projectID, branchID)
	if err == nil {
		defer sectionRows.Close()
		for sectionRows.Next() {
			var eid int64
			var n entityNode
			var isStale int
			sectionRows.Scan(&eid, &n.ID, &n.Title, &isStale, &n.Instant)
			n.Type = "section"
			if isStale == 1 {
				n.Status = "stale"
			} else {
				n.Status = "current"
			}
			sectionNodes = append(sectionNodes, n)
			sectionEntityToStable[eid] = n.ID
		}
	}

	// --- Entity edges ---
	entityEdges := make([]dagEdge, 0)

	// Decision -> Section: find section entities modified by each decision's transaction
	for _, decNode := range nodes {
		var txID int64
		err := conn.QueryRow(
			"SELECT tx_id FROM p_decisions WHERE stable_id = ? AND branch_id = ?",
			decNode.ID, branchID,
		).Scan(&txID)
		if err != nil {
			continue
		}
		// Find section entities touched by this tx
		secEdgeRows, err := conn.Query(`
			SELECT DISTINCT e.stable_id
			FROM datoms d
			JOIN entities e ON d.e = e.id
			WHERE d.tx = ? AND e.entity_type = 'section'
		`, txID)
		if err == nil {
			for secEdgeRows.Next() {
				var secStableID string
				secEdgeRows.Scan(&secStableID)
				entityEdges = append(entityEdges, dagEdge{Source: decNode.ID, Target: secStableID})
			}
			secEdgeRows.Close()
		}
	}

	// Thread -> Decision: outcome_decision_id
	threadOutcomeRows, err := conn.Query(`
		SELECT t.stable_id, t.outcome_decision_id
		FROM p_threads t
		WHERE t.project_id = ? AND t.outcome_decision_id IS NOT NULL
	`, projectID)
	if err == nil {
		defer threadOutcomeRows.Close()
		for threadOutcomeRows.Next() {
			var threadStableID string
			var outcomeDecEntityID int64
			threadOutcomeRows.Scan(&threadStableID, &outcomeDecEntityID)
			if decStableID, ok := entityToStable[outcomeDecEntityID]; ok {
				entityEdges = append(entityEdges, dagEdge{Source: threadStableID, Target: decStableID})
			}
		}
	}

	// Decision -> Task: tasks with source_type='decision'
	decTaskRows, err := conn.Query(`
		SELECT t.stable_id, t.source_id
		FROM p_tasks t
		WHERE t.project_id = ? AND t.source_type = 'decision' AND t.source_id IS NOT NULL
	`, projectID)
	if err == nil {
		defer decTaskRows.Close()
		for decTaskRows.Next() {
			var taskStableID string
			var sourceEntityID int64
			decTaskRows.Scan(&taskStableID, &sourceEntityID)
			if decStableID, ok := entityToStable[sourceEntityID]; ok {
				entityEdges = append(entityEdges, dagEdge{Source: decStableID, Target: taskStableID})
			}
		}
	}

	// Thread -> Task: tasks with source_type='thread'
	threadTaskRows, err := conn.Query(`
		SELECT t.stable_id, t.source_id
		FROM p_tasks t
		WHERE t.project_id = ? AND t.source_type = 'thread' AND t.source_id IS NOT NULL
	`, projectID)
	if err == nil {
		defer threadTaskRows.Close()
		for threadTaskRows.Next() {
			var taskStableID string
			var sourceEntityID int64
			threadTaskRows.Scan(&taskStableID, &sourceEntityID)
			if threadStableID, ok := threadEntityToStable[sourceEntityID]; ok {
				entityEdges = append(entityEdges, dagEdge{Source: threadStableID, Target: taskStableID})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"nodes":       nodes,
		"edges":       edges,
		"branches":    branchList,
		"milestones":  milestones,
		"threads":     threadNodes,
		"tasks":       taskNodes,
		"sections":    sectionNodes,
		"entityEdges": entityEdges,
	})
}

// --- Decision Detail ---

func (s *Server) handleGetDecision(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	branchID, err := s.resolveBranchID(r, projectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	decisionPrefix := r.PathValue("did")
	conn := s.db.Conn()

	var stableID, title, rationale, decCtx, author, instant string
	var txID int64
	var sourceThreadID sql.NullInt64
	err = conn.QueryRow(`
		SELECT d.stable_id, d.title, COALESCE(d.rationale,''), COALESCE(d.context,''),
		       COALESCE(d.author,''), d.instant, d.tx_id, d.source_thread_id
		FROM p_decisions d
		WHERE d.project_id = ? AND d.branch_id = ? AND d.stable_id = ?
		LIMIT 1
	`, projectID, branchID, decisionPrefix).Scan(
		&stableID, &title, &rationale, &decCtx, &author, &instant, &txID, &sourceThreadID,
	)
	if err != nil {
		writeError(w, http.StatusNotFound, "decision not found")
		return
	}

	// Build before/after changes from datoms
	type change struct {
		EntityID   string  `json:"entityId"`
		EntityType string  `json:"entityType"`
		Attribute  string  `json:"attribute"`
		Before     *string `json:"before"`
		After      *string `json:"after"`
	}

	datomRows, err := conn.Query(`
		SELECT e.stable_id, e.entity_type, d.a, d.v, d.op
		FROM datoms d
		JOIN entities e ON d.e = e.id
		WHERE d.tx = ?
		  AND e.entity_type IN ('section', 'project')
		ORDER BY d.e, d.a, d.op
	`, txID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer datomRows.Close()

	// Group by (entity, attribute) to form before/after pairs
	type datomEntry struct {
		entityID   string
		entityType string
		attr       string
		value      string
		op         int
	}
	var rawDatoms []datomEntry
	for datomRows.Next() {
		var d datomEntry
		if err := datomRows.Scan(&d.entityID, &d.entityType, &d.attr, &d.value, &d.op); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		rawDatoms = append(rawDatoms, d)
	}

	// Pair retracts (op=0) and asserts (op=1) by (entity, attr)
	type pairKey struct {
		entityID, attr string
	}
	pairs := make(map[pairKey]*change)
	for _, d := range rawDatoms {
		key := pairKey{d.entityID, d.attr}
		c, ok := pairs[key]
		if !ok {
			c = &change{
				EntityID:   d.entityID,
				EntityType: d.entityType,
				Attribute:  d.attr,
			}
			pairs[key] = c
		}
		decoded := decodeValueForDisplay(d.value)
		if d.op == 0 {
			c.Before = &decoded
		} else {
			c.After = &decoded
		}
	}

	changes := make([]change, 0, len(pairs))
	for _, c := range pairs {
		changes = append(changes, *c)
	}

	// Related tasks
	type taskRef struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	relatedTasks := make([]taskRef, 0)

	// Find decision entity_id
	var decisionEntityID int64
	conn.QueryRow("SELECT entity_id FROM p_decisions WHERE stable_id = ? AND branch_id = ?", stableID, branchID).Scan(&decisionEntityID)

	taskRows, err := conn.Query(`
		SELECT stable_id, title, status FROM p_tasks
		WHERE source_type = 'decision' AND source_id = ?
	`, decisionEntityID)
	if err == nil {
		defer taskRows.Close()
		for taskRows.Next() {
			var t taskRef
			taskRows.Scan(&t.ID, &t.Title, &t.Status)
			relatedTasks = append(relatedTasks, t)
		}
	}

	// Source thread info
	var sourceThread *map[string]string
	if sourceThreadID.Valid {
		var tStableID, tTitle, tStatus string
		err = conn.QueryRow(
			"SELECT stable_id, title, status FROM p_threads WHERE entity_id = ?",
			sourceThreadID.Int64,
		).Scan(&tStableID, &tTitle, &tStatus)
		if err == nil {
			m := map[string]string{
				"id":     tStableID,
				"title":  tTitle,
				"status": tStatus,
			}
			sourceThread = &m
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":           stableID,
		"title":        title,
		"rationale":    rationale,
		"context":      decCtx,
		"author":       author,
		"instant":      instant,
		"changes":      changes,
		"relatedTasks": relatedTasks,
		"sourceThread": sourceThread,
	})
}

// --- Sections ---

func (s *Server) handleListSections(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	branchID, err := s.resolveBranchID(r, projectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	sections, err := s.projects.GetSections(r.Context(), projectID, branchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type sectionResp struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Content     string `json:"content"`
		Position    int    `json:"position"`
		IsStale     bool   `json:"isStale"`
		StaleReason string `json:"staleReason,omitempty"`
	}
	result := make([]sectionResp, len(sections))
	for i, sec := range sections {
		result[i] = sectionResp{
			ID:          sec.StableID,
			Title:       sec.Title,
			Content:     sec.Content,
			Position:    sec.Position,
			IsStale:     sec.IsStale,
			StaleReason: sec.StaleReason,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetSection(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	branchID, err := s.resolveBranchID(r, projectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	sectionPrefix := r.PathValue("sid")
	var sectionEntityID int64
	err = s.db.Conn().QueryRow(
		"SELECT entity_id FROM p_sections WHERE project_id = ? AND branch_id = ? AND stable_id = ?",
		projectID, branchID, sectionPrefix,
	).Scan(&sectionEntityID)
	if err != nil {
		writeError(w, http.StatusNotFound, "section not found")
		return
	}

	detail, err := s.projects.GetSection(r.Context(), sectionEntityID, branchID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	type refResp struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	refsTo := make([]refResp, len(detail.RefsTo))
	for i, ref := range detail.RefsTo {
		refsTo[i] = refResp{ID: ref.StableID, Title: ref.Title}
	}
	refsFrom := make([]refResp, len(detail.RefsFrom))
	for i, ref := range detail.RefsFrom {
		refsFrom[i] = refResp{ID: ref.StableID, Title: ref.Title}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":          detail.StableID,
		"title":       detail.Title,
		"content":     detail.Content,
		"position":    detail.Position,
		"isStale":     detail.IsStale,
		"staleReason": detail.StaleReason,
		"refsTo":      refsTo,
		"refsFrom":    refsFrom,
	})
}

// --- Threads ---

func (s *Server) handleListThreads(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	statusFilter := r.URL.Query().Get("status")
	threads, err := s.threads.ListThreads(r.Context(), projectID, statusFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type threadResp struct {
		ID                string  `json:"id"`
		Title             string  `json:"title"`
		Question          string  `json:"question"`
		Status            string  `json:"status"`
		OutcomeDecisionID *string `json:"outcomeDecisionId"`
	}
	result := make([]threadResp, len(threads))
	for i, t := range threads {
		resp := threadResp{
			ID:       t.StableID,
			Title:    t.Title,
			Question: t.Question,
			Status:   t.Status,
		}
		if t.OutcomeDecisionID != nil {
			var sid string
			s.db.Conn().QueryRow("SELECT stable_id FROM entities WHERE id = ?", *t.OutcomeDecisionID).Scan(&sid)
			if sid != "" {
				resp.OutcomeDecisionID = &sid
			}
		}
		result[i] = resp
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetThread(w http.ResponseWriter, r *http.Request) {
	threadPrefix := r.PathValue("tid")
	var threadEntityID int64
	err := s.db.Conn().QueryRow(
		"SELECT entity_id FROM p_threads WHERE stable_id = ?", threadPrefix,
	).Scan(&threadEntityID)
	if err != nil {
		writeError(w, http.StatusNotFound, "thread not found")
		return
	}

	thread, err := s.threads.GetThread(r.Context(), threadEntityID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	entries, err := s.threads.GetEntries(r.Context(), threadEntityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type entryResp struct {
		ID          string  `json:"id"`
		Type        string  `json:"type"`
		Content     string  `json:"content"`
		Author      string  `json:"author"`
		TargetID    *string `json:"targetId,omitempty"`
		Stance      string  `json:"stance,omitempty"`
		IsRetracted bool    `json:"isRetracted"`
		Instant     string  `json:"instant"`
	}
	entryList := make([]entryResp, len(entries))
	for i, e := range entries {
		resp := entryResp{
			ID:          e.StableID,
			Type:        e.Type,
			Content:     e.Content,
			Author:      e.Author,
			Stance:      e.Stance,
			IsRetracted: e.IsRetracted,
			Instant:     e.Instant,
		}
		if e.TargetID != nil {
			var sid string
			s.db.Conn().QueryRow("SELECT stable_id FROM entities WHERE id = ?", *e.TargetID).Scan(&sid)
			if sid != "" {
				resp.TargetID = &sid
			}
		}
		entryList[i] = resp
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":       thread.StableID,
		"title":    thread.Title,
		"question": thread.Question,
		"status":   thread.Status,
		"entries":  entryList,
	})
}

// --- Branches ---

func (s *Server) handleListBranches(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	branches, err := s.branches.ListBranches(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type branchResp struct {
		ID             string  `json:"id"`
		Name           string  `json:"name"`
		Status         string  `json:"status"`
		IsMain         bool    `json:"isMain"`
		HeadDecisionID *string `json:"headDecisionId"`
	}
	result := make([]branchResp, len(branches))
	for i, b := range branches {
		resp := branchResp{
			ID:     b.StableID,
			Name:   b.Name,
			Status: b.Status,
			IsMain: b.IsMain,
		}
		if b.HeadDecisionID != nil {
			var sid string
			s.db.Conn().QueryRow("SELECT stable_id FROM entities WHERE id = ?", *b.HeadDecisionID).Scan(&sid)
			if sid != "" {
				resp.HeadDecisionID = &sid
			}
		}
		result[i] = resp
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Milestones ---

func (s *Server) handleListMilestones(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	conn := s.db.Conn()
	rows, err := conn.Query(`
		SELECT m.stable_id, m.title, COALESCE(m.description,''), m.decision_id
		FROM p_milestones m WHERE m.project_id = ?
	`, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type milestoneResp struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		DecisionID  string `json:"decisionId"`
	}
	result := make([]milestoneResp, 0)
	for rows.Next() {
		var m milestoneResp
		var decID int64
		rows.Scan(&m.ID, &m.Title, &m.Description, &decID)
		var sid string
		conn.QueryRow("SELECT stable_id FROM entities WHERE id = ?", decID).Scan(&sid)
		m.DecisionID = sid
		result = append(result, m)
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Topics ---

func (s *Server) handleListTopics(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	topics, err := s.topics.ListTopics(r.Context(), projectID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type topicResp struct {
		ID          string   `json:"id"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Status      string   `json:"status"`
		ThreadIDs   []string `json:"threadIds"`
	}

	conn := s.db.Conn()
	result := make([]topicResp, len(topics))
	for i, t := range topics {
		resp := topicResp{
			ID:          t.StableID,
			Title:       t.Title,
			Description: t.Description,
			Status:      t.Status,
			ThreadIDs:   make([]string, 0),
		}

		// Get linked thread stable IDs
		rows, err := conn.Query(`
			SELECT e.stable_id
			FROM topic_threads tt
			JOIN entities e ON tt.thread_id = e.id
			WHERE tt.topic_id = ?
		`, t.EntityID)
		if err == nil {
			for rows.Next() {
				var sid string
				rows.Scan(&sid)
				resp.ThreadIDs = append(resp.ThreadIDs, sid)
			}
			rows.Close()
		}

		result[i] = resp
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetTopic(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	topicPrefix := r.PathValue("tid")
	topic, err := s.topics.FindTopic(r.Context(), projectID, topicPrefix)
	if err != nil {
		writeError(w, http.StatusNotFound, "topic not found")
		return
	}

	threads, err := s.topics.GetTopicThreads(r.Context(), topic.EntityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type threadRef struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Question string `json:"question"`
		Status   string `json:"status"`
	}
	threadList := make([]threadRef, len(threads))
	for i, t := range threads {
		threadList[i] = threadRef{
			ID:       t.StableID,
			Title:    t.Title,
			Question: t.Question,
			Status:   t.Status,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":          topic.StableID,
		"title":       topic.Title,
		"description": topic.Description,
		"status":      topic.Status,
		"threads":     threadList,
	})
}

// --- Conflicts ---

func (s *Server) handleListConflicts(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	branchID, err := s.resolveBranchID(r, projectID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	conn := s.db.Conn()
	rows, err := conn.Query(`
		SELECT c.stable_id, c.section_id, c.field, c.status,
		       COALESCE(c.resolution,''), COALESCE(c.resolution_rationale,'')
		FROM p_conflicts c
		WHERE c.project_id = ? AND c.branch_id = ?
	`, projectID, branchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type conflictResp struct {
		ID                  string `json:"id"`
		SectionID           string `json:"sectionId"`
		Field               string `json:"field"`
		Status              string `json:"status"`
		Resolution          string `json:"resolution,omitempty"`
		ResolutionRationale string `json:"resolutionRationale,omitempty"`
	}
	result := make([]conflictResp, 0)
	for rows.Next() {
		var c conflictResp
		var sectionEntityID int64
		rows.Scan(&c.ID, &sectionEntityID, &c.Field, &c.Status, &c.Resolution, &c.ResolutionRationale)
		var sid string
		conn.QueryRow("SELECT stable_id FROM entities WHERE id = ?", sectionEntityID).Scan(&sid)
		c.SectionID = sid
		result = append(result, c)
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Tasks (cross-project) ---

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	conn := s.db.Conn()

	query := `
		SELECT t.stable_id, t.title, COALESCE(t.description,''), t.status,
		       COALESCE(t.priority,'medium'), COALESCE(t.assignee,''),
		       t.project_id, p.name
		FROM p_tasks t
		JOIN p_projects p ON t.project_id = p.entity_id
		WHERE 1=1
	`
	args := []any{}

	if statusFilter := r.URL.Query().Get("status"); statusFilter != "" {
		statuses := splitTrim(statusFilter, ",")
		placeholders := make([]string, len(statuses))
		for i, st := range statuses {
			placeholders[i] = "?"
			args = append(args, st)
		}
		query += " AND t.status IN (" + joinStrings(placeholders, ",") + ")"
	}

	if projectFilter := r.URL.Query().Get("project"); projectFilter != "" {
		query += " AND p.stable_id = ?"
		args = append(args, projectFilter)
	}

	query += " ORDER BY t.entity_id"

	rows, err := conn.Query(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type taskResp struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
		Priority    string `json:"priority"`
		Assignee    string `json:"assignee"`
		ProjectID   string `json:"projectId"`
		ProjectName string `json:"projectName"`
	}
	result := make([]taskResp, 0)
	for rows.Next() {
		var t taskResp
		var projectEntityID int64
		rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.Assignee,
			&projectEntityID, &t.ProjectName)
		var sid string
		conn.QueryRow("SELECT stable_id FROM entities WHERE id = ?", projectEntityID).Scan(&sid)
		t.ProjectID = sid
		result = append(result, t)
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	taskPrefix := r.PathValue("id")

	var body struct {
		Status   string `json:"status"`
		Assignee string `json:"assignee"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Status == "" && body.Assignee == "" {
		writeError(w, http.StatusBadRequest, "status or assignee required")
		return
	}

	// Find task by stable_id prefix
	var taskEntityID int64
	err := s.db.Conn().QueryRow(
		"SELECT entity_id FROM p_tasks WHERE stable_id = ?", taskPrefix,
	).Scan(&taskEntityID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	if err := s.tasks.UpdateTask(r.Context(), taskEntityID, body.Status, body.Assignee); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// --- Value display helper ---

func decodeValueForDisplay(encoded string) string {
	val, err := eavt.DecodeValue(encoded)
	if err != nil {
		return encoded
	}
	switch val.Type {
	case eavt.TypeString, eavt.TypeEnum, eavt.TypeDateTime:
		s, _ := val.AsString()
		return s
	case eavt.TypeInt:
		i, _ := val.AsInt64()
		return strconv.FormatInt(i, 10)
	case eavt.TypeRef:
		i, _ := val.AsInt64()
		return fmt.Sprintf("ref:%d", i)
	case eavt.TypeBool:
		b, _ := val.AsBool()
		return strconv.FormatBool(b)
	case eavt.TypeRefSet:
		ids, _ := val.AsRefSet()
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = fmt.Sprintf("ref:%d", id)
		}
		return "[" + joinStrings(parts, ",") + "]"
	default:
		return encoded
	}
}

// --- String helpers ---

func splitTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(s, sep) {
		p = trimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	result := make([]string, 0)
	for {
		i := indexString(s, sep)
		if i < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:i])
		s = s[i+len(sep):]
	}
	return result
}

func indexString(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}

// --- Graph (all-branch topology) ---

func (s *Server) handleGetGraph(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	conn := s.db.Conn()

	// All decisions across all branches
	type graphNode struct {
		ID             string  `json:"id"`
		Title          string  `json:"title"`
		Author         string  `json:"author"`
		Instant        string  `json:"instant"`
		Type           string  `json:"type"`
		BranchID       string  `json:"branchId"`
		BranchName     string  `json:"branchName"`
		SourceThreadID *string `json:"sourceThreadId,omitempty"`
	}

	rows, err := conn.Query(`
		SELECT d.stable_id, d.title, COALESCE(d.author,''), d.instant,
		       d.source_thread_id, d.branch_id,
		       b.stable_id, COALESCE(b.name,'main'), b.is_main
		FROM p_decisions d
		JOIN p_branches b ON d.branch_id = b.entity_id
		WHERE d.project_id = ?
		ORDER BY d.tx_id ASC
	`, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	nodes := make([]graphNode, 0)
	entityToStable := make(map[int64]string)

	for rows.Next() {
		var n graphNode
		var sourceThreadID sql.NullInt64
		var branchEntityID int64
		var isMain int
		if err := rows.Scan(&n.ID, &n.Title, &n.Author, &n.Instant,
			&sourceThreadID, &branchEntityID, &n.BranchID, &n.BranchName, &isMain); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if sourceThreadID.Valid {
			var sid string
			conn.QueryRow("SELECT stable_id FROM entities WHERE id = ?", sourceThreadID.Int64).Scan(&sid)
			if sid != "" {
				n.SourceThreadID = &sid
			}
		}
		n.Type = "normal"
		nodes = append(nodes, n)
	}

	// Build entity_id -> stable_id for all decisions in this project
	mapRows, err := conn.Query(
		"SELECT entity_id, stable_id FROM p_decisions WHERE project_id = ?", projectID,
	)
	if err == nil {
		defer mapRows.Close()
		for mapRows.Next() {
			var eid int64
			var sid string
			mapRows.Scan(&eid, &sid)
			entityToStable[eid] = sid
		}
	}

	// Edges across all branches
	type graphEdge struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	edges := make([]graphEdge, 0)
	parentCounts := make(map[string]int)

	edgeRows, err := conn.Query(`
		SELECT dp.decision_id, dp.parent_id
		FROM p_decision_parents dp
		JOIN p_decisions d ON dp.decision_id = d.entity_id
		WHERE d.project_id = ?
	`, projectID)
	if err == nil {
		defer edgeRows.Close()
		for edgeRows.Next() {
			var decID, parentID int64
			edgeRows.Scan(&decID, &parentID)
			source := entityToStable[parentID]
			target := entityToStable[decID]
			if source != "" && target != "" {
				edges = append(edges, graphEdge{Source: source, Target: target})
				parentCounts[target]++
			}
		}
	}

	// Classify node types
	childSet := make(map[string]bool)
	for _, e := range edges {
		childSet[e.Target] = true
		if parentCounts[e.Target] >= 2 {
			// will mark as merge below
		}
	}
	for i := range nodes {
		if parentCounts[nodes[i].ID] >= 2 {
			nodes[i].Type = "merge"
		} else if !childSet[nodes[i].ID] {
			nodes[i].Type = "root"
		}
	}

	// Branches
	type branchInfo struct {
		ID             string  `json:"id"`
		Name           string  `json:"name"`
		IsMain         bool    `json:"isMain"`
		HeadDecisionID *string `json:"headDecisionId"`
		Status         string  `json:"status"`
	}
	branchList := make([]branchInfo, 0)
	branchRows, err := conn.Query(`
		SELECT b.stable_id, COALESCE(b.name,''), b.is_main, b.head_decision_id, b.status
		FROM p_branches b WHERE b.project_id = ?
	`, projectID)
	if err == nil {
		defer branchRows.Close()
		for branchRows.Next() {
			var b branchInfo
			var isMain int
			var headID sql.NullInt64
			branchRows.Scan(&b.ID, &b.Name, &isMain, &headID, &b.Status)
			b.IsMain = isMain == 1
			if headID.Valid {
				if sid, ok := entityToStable[headID.Int64]; ok {
					b.HeadDecisionID = &sid
				}
			}
			branchList = append(branchList, b)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"decisions": nodes,
		"edges":     edges,
		"branches":  branchList,
	})
}

// --- Git integration: commits & repos ---

type commitResp struct {
	ID         string   `json:"id"`
	SHA        string   `json:"sha"`
	Message    string   `json:"message"`
	Author     string   `json:"author"`
	AuthoredAt string   `json:"authored_at"`
	Parents    []string `json:"parents"`
	TaskID     *string  `json:"task_id,omitempty"`
	RepoID     string   `json:"repo_id"`
	Status     string   `json:"status"`
}

func (s *Server) handleListCommits(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var taskFilter *int64
	if t := r.URL.Query().Get("task"); t != "" {
		var taskEntityID int64
		if err := s.db.Conn().QueryRow(
			"SELECT entity_id FROM p_tasks WHERE project_id = ? AND stable_id = ?",
			projectID, t,
		).Scan(&taskEntityID); err == nil {
			taskFilter = &taskEntityID
		}
	}

	commits, err := s.commits.ListCommits(r.Context(), projectID, taskFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Map entity_id -> stable_id for task and repo references
	entityToStable := map[int64]string{}
	rows, _ := s.db.Conn().Query("SELECT id, stable_id FROM entities")
	for rows.Next() {
		var id int64
		var sid string
		rows.Scan(&id, &sid)
		entityToStable[id] = sid
	}
	rows.Close()

	out := make([]commitResp, 0, len(commits))
	for _, c := range commits {
		cr := commitResp{
			ID:         c.StableID,
			SHA:        c.SHA,
			Message:    c.Message,
			Author:     c.Author,
			AuthoredAt: c.AuthoredAt,
			Parents:    c.Parents,
			RepoID:     entityToStable[c.RepoID],
			Status:     c.Status,
		}
		if c.TaskID != nil {
			tid := entityToStable[*c.TaskID]
			cr.TaskID = &tid
		}
		out = append(out, cr)
	}
	writeJSON(w, http.StatusOK, out)
}

type repoResp struct {
	ID        string `json:"id"`
	UUID      string `json:"uuid"`
	RemoteURL string `json:"remote_url"`
}

func (s *Server) handleListRepos(w http.ResponseWriter, r *http.Request) {
	projectID, err := s.resolveProjectID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	rows, err := s.db.Conn().QueryContext(r.Context(),
		"SELECT stable_id, uuid, COALESCE(remote_url,'') FROM p_repos WHERE project_id = ? ORDER BY entity_id",
		projectID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	out := []repoResp{}
	for rows.Next() {
		var rr repoResp
		if err := rows.Scan(&rr.ID, &rr.UUID, &rr.RemoteURL); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, rr)
	}
	writeJSON(w, http.StatusOK, out)
}
