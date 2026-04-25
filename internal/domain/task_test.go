package domain

import (
	"context"
	"strings"
	"testing"

	"github.com/nakagami-306/orbit/internal/projection"
)

// taskTestEnv is a minimal task-only setup (no git repo needed).
type taskTestEnv struct {
	ctx       context.Context
	projectID int64
	tasks     *TaskService
}

func setupTaskEnv(t *testing.T) *taskTestEnv {
	t.Helper()
	d := setupTestDB(t)
	ctx := context.Background()

	proj := &ProjectService{DB: d, Projector: &projection.Projector{}}
	if _, _, err := proj.CreateProject(ctx, "task-test", ""); err != nil {
		t.Fatalf("create project: %v", err)
	}
	p, err := proj.GetProjectByName(ctx, "task-test")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	tasks := &TaskService{DB: d, Projector: &projection.Projector{}}
	return &taskTestEnv{ctx: ctx, projectID: p.EntityID, tasks: tasks}
}

func (e *taskTestEnv) newTask(t *testing.T, title string) *Task {
	t.Helper()
	id, err := e.tasks.CreateTask(e.ctx, e.projectID, title, "", "m", "", "", nil)
	if err != nil {
		t.Fatalf("create task %q: %v", title, err)
	}
	task, err := e.tasks.FindTask(e.ctx, e.projectID, id)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	return task
}

func TestStartTaskRecordsBranch(t *testing.T) {
	env := setupTaskEnv(t)
	task := env.newTask(t, "feature")

	if err := env.tasks.StartTask(env.ctx, task.EntityID, "feat/auth"); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	got, _ := env.tasks.FindTask(env.ctx, env.projectID, task.StableID)
	if got.Status != "in-progress" {
		t.Errorf("status = %s, want in-progress", got.Status)
	}
	if got.GitBranch != "feat/auth" {
		t.Errorf("git_branch = %q, want feat/auth", got.GitBranch)
	}
}

func TestStartTaskRejectsEmptyBranch(t *testing.T) {
	env := setupTaskEnv(t)
	task := env.newTask(t, "x")

	err := env.tasks.StartTask(env.ctx, task.EntityID, "")
	if err == nil {
		t.Fatal("expected error for empty branch (detached HEAD)")
	}
	if !strings.Contains(err.Error(), "detached") && !strings.Contains(err.Error(), "branch name") {
		t.Errorf("unhelpful error: %v", err)
	}
}

func TestStartTaskRejectsDuplicateActiveBranch(t *testing.T) {
	env := setupTaskEnv(t)
	taskA := env.newTask(t, "A")
	taskB := env.newTask(t, "B")

	if err := env.tasks.StartTask(env.ctx, taskA.EntityID, "feat/x"); err != nil {
		t.Fatalf("StartTask A: %v", err)
	}
	err := env.tasks.StartTask(env.ctx, taskB.EntityID, "feat/x")
	if err == nil {
		t.Fatal("expected duplicate-branch error")
	}
	if !strings.Contains(err.Error(), "already owned") {
		t.Errorf("unhelpful error: %v", err)
	}
}

func TestStartTaskAllowsBranchReuseAfterDone(t *testing.T) {
	env := setupTaskEnv(t)
	first := env.newTask(t, "old")
	if err := env.tasks.StartTask(env.ctx, first.EntityID, "feat/r"); err != nil {
		t.Fatalf("StartTask first: %v", err)
	}
	if _, err := env.tasks.DoneTask(env.ctx, first.EntityID); err != nil {
		t.Fatalf("DoneTask first: %v", err)
	}

	second := env.newTask(t, "new")
	if err := env.tasks.StartTask(env.ctx, second.EntityID, "feat/r"); err != nil {
		t.Fatalf("StartTask second on reused branch: %v", err)
	}
}

func TestStartTaskReassignmentOverwrites(t *testing.T) {
	env := setupTaskEnv(t)
	task := env.newTask(t, "x")

	if err := env.tasks.StartTask(env.ctx, task.EntityID, "feat/old"); err != nil {
		t.Fatalf("StartTask first: %v", err)
	}
	// Re-start with a different branch name (warning is informational; should succeed)
	if err := env.tasks.StartTask(env.ctx, task.EntityID, "feat/new"); err != nil {
		t.Fatalf("StartTask reassign: %v", err)
	}
	got, _ := env.tasks.FindTask(env.ctx, env.projectID, task.StableID)
	if got.GitBranch != "feat/new" {
		t.Errorf("git_branch = %q, want feat/new", got.GitBranch)
	}
}

func TestDoneTaskReturnsBranch(t *testing.T) {
	env := setupTaskEnv(t)
	task := env.newTask(t, "x")
	if err := env.tasks.StartTask(env.ctx, task.EntityID, "feat/done-test"); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	branch, err := env.tasks.DoneTask(env.ctx, task.EntityID)
	if err != nil {
		t.Fatalf("DoneTask: %v", err)
	}
	if branch != "feat/done-test" {
		t.Errorf("branch = %q, want feat/done-test", branch)
	}

	got, _ := env.tasks.FindTask(env.ctx, env.projectID, task.StableID)
	if got.Status != "done" {
		t.Errorf("status = %s, want done", got.Status)
	}
}

func TestDoneTaskFromTodoFails(t *testing.T) {
	env := setupTaskEnv(t)
	task := env.newTask(t, "x")
	_, err := env.tasks.DoneTask(env.ctx, task.EntityID)
	if err == nil {
		t.Fatal("expected todo→done transition to fail")
	}
}
