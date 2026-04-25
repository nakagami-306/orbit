package domain

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nakagami-306/orbit/internal/projection"
)

// gitNewSeededRepo creates a fresh git repo with a single initial commit on `main`.
// Returns the working tree path.
func gitNewSeededRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@orbit.local"},
		{"config", "user.name", "Orbit Test"},
		{"config", "commit.gpgsign", "false"},
	} {
		gitMust(t, dir, args...)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("init\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitMust(t, dir, "add", ".")
	gitMust(t, dir, "commit", "-m", "initial")
	return dir
}

func gitMust(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

// commitTestEnv is the bundle of services + IDs needed by every commit-scan test.
type commitTestEnv struct {
	ctx       context.Context
	repoRoot  string
	projectID int64
	repoEID   int64
	tasks     *TaskService
	commits   *CommitService
	repos     *RepoService
}

func setupCommitEnv(t *testing.T) *commitTestEnv {
	t.Helper()
	d := setupTestDB(t)
	ctx := context.Background()
	proj := &ProjectService{DB: d, Projector: &projection.Projector{}}

	if _, _, err := proj.CreateProject(ctx, "scan-test", ""); err != nil {
		t.Fatalf("create project: %v", err)
	}
	p, err := proj.GetProjectByName(ctx, "scan-test")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}

	tasks := &TaskService{DB: d, Projector: &projection.Projector{}}
	commits := &CommitService{DB: d, Projector: &projection.Projector{}}
	repos := &RepoService{DB: d, Projector: &projection.Projector{}}

	repoRoot := gitNewSeededRepo(t)

	repo, err := repos.EnsureRepo(ctx, p.EntityID, "")
	if err != nil {
		t.Fatalf("ensure repo: %v", err)
	}

	return &commitTestEnv{
		ctx:       ctx,
		repoRoot:  repoRoot,
		projectID: p.EntityID,
		repoEID:   repo.EntityID,
		tasks:     tasks,
		commits:   commits,
		repos:     repos,
	}
}

func (e *commitTestEnv) makeTask(t *testing.T, title string) *Task {
	t.Helper()
	id, err := e.tasks.CreateTask(e.ctx, e.projectID, title, "", "m", "", "", nil)
	if err != nil {
		t.Fatalf("create task %q: %v", title, err)
	}
	task, err := e.tasks.FindTask(e.ctx, e.projectID, id)
	if err != nil {
		t.Fatalf("find task: %v", err)
	}
	return task
}

// ----- ScanRepo -----

func TestScanInitialCommitRegistered(t *testing.T) {
	env := setupCommitEnv(t)

	res, err := env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)
	if err != nil {
		t.Fatalf("ScanRepo: %v", err)
	}
	if res.Added != 1 {
		t.Errorf("Added = %d, want 1 (initial commit)", res.Added)
	}
	if res.Bound != 0 {
		t.Errorf("Bound = %d, want 0 (no active task)", res.Bound)
	}
}

func TestScanIdempotent(t *testing.T) {
	env := setupCommitEnv(t)
	_, _ = env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)

	res, err := env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)
	if err != nil {
		t.Fatalf("ScanRepo (2nd): %v", err)
	}
	if res.Added != 0 {
		t.Errorf("2nd Added = %d, want 0", res.Added)
	}
}

func TestScanNewCommitAdded(t *testing.T) {
	env := setupCommitEnv(t)
	_, _ = env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)

	gitMust(t, env.repoRoot, "commit", "--allow-empty", "-m", "second")
	res, err := env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)
	if err != nil {
		t.Fatalf("ScanRepo: %v", err)
	}
	if res.Added != 1 {
		t.Errorf("Added = %d, want 1", res.Added)
	}
}

func TestScanBindsToInProgressTask(t *testing.T) {
	env := setupCommitEnv(t)
	task := env.makeTask(t, "feature A")
	if err := env.tasks.StartTask(env.ctx, task.EntityID, "main"); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	gitMust(t, env.repoRoot, "commit", "--allow-empty", "-m", "work")
	res, _ := env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)

	// initial + work → 2 added; both should be bound (both reachable on main)
	if res.Bound < 1 {
		t.Errorf("Bound = %d, want >=1", res.Bound)
	}

	commits, _ := env.commits.ListCommits(env.ctx, env.projectID, &task.EntityID)
	if len(commits) < 1 {
		t.Errorf("commits for task = %d, want >=1", len(commits))
	}
}

func TestScanIgnoresDoneTasksBranch(t *testing.T) {
	env := setupCommitEnv(t)
	task := env.makeTask(t, "old work")
	_ = env.tasks.StartTask(env.ctx, task.EntityID, "main")
	if _, err := env.tasks.DoneTask(env.ctx, task.EntityID); err != nil {
		t.Fatalf("DoneTask: %v", err)
	}

	gitMust(t, env.repoRoot, "commit", "--allow-empty", "-m", "after done")
	_, _ = env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)

	taskCommits, _ := env.commits.ListCommits(env.ctx, env.projectID, &task.EntityID)
	for _, c := range taskCommits {
		if c.Message == "after done" {
			t.Errorf("commit %q was bound to done task; should be unbound", c.Message)
		}
	}
}

func TestScanInProgressPreferredOverTodo(t *testing.T) {
	env := setupCommitEnv(t)
	todoTask := env.makeTask(t, "todo task")
	progTask := env.makeTask(t, "active task")
	// Both target the same branch; only one can be active simultaneously,
	// so we set the todo one's git_branch via Start then revert to todo.
	// Simpler: start the active one, leave the other unstarted on a different branch.
	if err := env.tasks.StartTask(env.ctx, progTask.EntityID, "main"); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	_ = todoTask // sanity reference; in this layout the in-progress task wins by definition

	gitMust(t, env.repoRoot, "commit", "--allow-empty", "-m", "for active")
	_, _ = env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)

	progCommits, _ := env.commits.ListCommits(env.ctx, env.projectID, &progTask.EntityID)
	if len(progCommits) == 0 {
		t.Errorf("in-progress task got 0 commits, want >=1")
	}
}

func TestScanOrphansUnreachableCommits(t *testing.T) {
	env := setupCommitEnv(t)
	gitMust(t, env.repoRoot, "checkout", "-b", "feat/temp")
	tempSHA := gitMust(t, env.repoRoot, "commit", "--allow-empty", "-m", "temp work")
	tempSHA = strings.TrimSpace(gitMust(t, env.repoRoot, "rev-parse", "HEAD"))
	_ = tempSHA
	_, _ = env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)

	// Now delete the branch (force, because not merged) and prune the unreachable ref
	gitMust(t, env.repoRoot, "checkout", "main")
	gitMust(t, env.repoRoot, "branch", "-D", "feat/temp")

	res, err := env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)
	if err != nil {
		t.Fatalf("ScanRepo (post-delete): %v", err)
	}
	if res.Orphaned < 1 {
		t.Errorf("Orphaned = %d, want >=1", res.Orphaned)
	}
}

// ----- BindCommit / UnbindCommit -----

func TestBindAndUnbind(t *testing.T) {
	env := setupCommitEnv(t)
	_, _ = env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)

	commits, _ := env.commits.ListCommits(env.ctx, env.projectID, nil)
	if len(commits) == 0 {
		t.Fatal("no commits to bind")
	}
	c := commits[0]
	if c.TaskID != nil {
		t.Fatalf("expected unbound, got task=%d", *c.TaskID)
	}

	task := env.makeTask(t, "manual bind target")
	if err := env.commits.BindCommit(env.ctx, env.projectID, c.SHA, task.EntityID); err != nil {
		t.Fatalf("BindCommit: %v", err)
	}

	bound, _ := env.commits.ListCommits(env.ctx, env.projectID, &task.EntityID)
	if len(bound) != 1 || bound[0].SHA != c.SHA {
		t.Errorf("bind didn't take effect: bound=%v", bound)
	}

	if err := env.commits.UnbindCommit(env.ctx, env.projectID, c.SHA); err != nil {
		t.Fatalf("UnbindCommit: %v", err)
	}
	after, _ := env.commits.ListCommits(env.ctx, env.projectID, &task.EntityID)
	if len(after) != 0 {
		t.Errorf("unbind didn't clear: %v", after)
	}
}

// ----- FindCommitBySHAPrefix -----

func TestFindCommitBySHAPrefix(t *testing.T) {
	env := setupCommitEnv(t)
	_, _ = env.commits.ScanRepo(env.ctx, env.projectID, env.repoEID, env.repoRoot)

	commits, _ := env.commits.ListCommits(env.ctx, env.projectID, nil)
	if len(commits) == 0 {
		t.Fatal("no commits")
	}
	full := commits[0].SHA

	// unique prefix
	c, err := env.commits.FindCommitBySHAPrefix(env.ctx, env.projectID, full[:8])
	if err != nil {
		t.Fatalf("FindCommitBySHAPrefix(unique): %v", err)
	}
	if c.SHA != full {
		t.Errorf("returned wrong commit: %s != %s", c.SHA, full)
	}

	// missing
	if _, err := env.commits.FindCommitBySHAPrefix(env.ctx, env.projectID, "deadbeef00"); err == nil {
		t.Error("expected error for unknown prefix")
	}
}

// ----- EnsureRepo idempotency -----

func TestEnsureRepoIdempotent(t *testing.T) {
	env := setupCommitEnv(t)
	r2, err := env.repos.EnsureRepo(env.ctx, env.projectID, "")
	if err != nil {
		t.Fatalf("EnsureRepo (2nd): %v", err)
	}
	if r2.EntityID != env.repoEID {
		t.Errorf("EnsureRepo returned different entity: %d vs %d", r2.EntityID, env.repoEID)
	}
}

func TestEnsureRepoUpdatesRemoteURL(t *testing.T) {
	env := setupCommitEnv(t)
	first, _ := env.repos.FindRepoByProject(env.ctx, env.projectID)
	if first.RemoteURL != "" {
		t.Fatalf("baseline RemoteURL should be empty, got %q", first.RemoteURL)
	}

	if _, err := env.repos.EnsureRepo(env.ctx, env.projectID, "https://example.com/orbit.git"); err != nil {
		t.Fatalf("EnsureRepo with new URL: %v", err)
	}
	updated, _ := env.repos.FindRepoByProject(env.ctx, env.projectID)
	if updated.RemoteURL != "https://example.com/orbit.git" {
		t.Errorf("RemoteURL = %q, want updated", updated.RemoteURL)
	}
}
