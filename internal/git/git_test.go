package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo creates a fresh git repository in a temp dir with a configured identity
// and a single initial commit on the "main" branch. Returns the working dir.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	mustRun(t, dir, "init", "-b", "main")
	mustRun(t, dir, "config", "user.email", "test@orbit.local")
	mustRun(t, dir, "config", "user.name", "Orbit Test")
	mustRun(t, dir, "config", "commit.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# init\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, dir, "add", ".")
	mustRun(t, dir, "commit", "-m", "initial")
	return dir
}

func mustRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

// emptyCommit creates a commit with no changes for ordering tests.
func emptyCommit(t *testing.T, dir, msg string) string {
	t.Helper()
	mustRun(t, dir, "commit", "--allow-empty", "-m", msg)
	return mustRun(t, dir, "rev-parse", "HEAD")
}

// ----- IsRepo -----

func TestIsRepo(t *testing.T) {
	repo := newRepo(t)
	if !IsRepo(repo) {
		t.Errorf("IsRepo(%s) = false, want true", repo)
	}

	notRepo := t.TempDir()
	if IsRepo(notRepo) {
		t.Errorf("IsRepo(%s) = true, want false", notRepo)
	}
}

// ----- ToplevelDir -----

func TestToplevelDirFromSubdir(t *testing.T) {
	repo := newRepo(t)
	sub := filepath.Join(repo, "nested", "deep")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	got, err := ToplevelDir(sub)
	if err != nil {
		t.Fatalf("ToplevelDir: %v", err)
	}
	// git may return symlink-resolved path; compare basenames.
	if filepath.Base(got) != filepath.Base(repo) {
		t.Errorf("ToplevelDir basename = %s, want %s", filepath.Base(got), filepath.Base(repo))
	}
}

func TestToplevelDirNotARepo(t *testing.T) {
	notRepo := t.TempDir()
	_, err := ToplevelDir(notRepo)
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("err = %v, want ErrNotARepo", err)
	}
}

// ----- CurrentBranch -----

func TestCurrentBranch(t *testing.T) {
	repo := newRepo(t)
	br, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if br != "main" {
		t.Errorf("branch = %q, want main", br)
	}
}

func TestCurrentBranchDetachedHEAD(t *testing.T) {
	repo := newRepo(t)
	sha := mustRun(t, repo, "rev-parse", "HEAD")
	mustRun(t, repo, "checkout", "--detach", sha)
	_, err := CurrentBranch(repo)
	if !errors.Is(err, ErrDetachedHEAD) {
		t.Errorf("err = %v, want ErrDetachedHEAD", err)
	}
}

// ----- RemoteURL -----

func TestRemoteURLAbsent(t *testing.T) {
	repo := newRepo(t)
	url, err := RemoteURL(repo, "origin")
	if err != nil {
		t.Fatalf("RemoteURL: %v", err)
	}
	if url != "" {
		t.Errorf("url = %q, want empty for missing remote", url)
	}
}

func TestRemoteURLConfigured(t *testing.T) {
	repo := newRepo(t)
	mustRun(t, repo, "remote", "add", "origin", "https://example.com/test.git")
	url, err := RemoteURL(repo, "origin")
	if err != nil {
		t.Fatalf("RemoteURL: %v", err)
	}
	if url != "https://example.com/test.git" {
		t.Errorf("url = %q, want https://example.com/test.git", url)
	}
}

// ----- AllCommitSHAs -----

func TestAllCommitSHAsLinear(t *testing.T) {
	repo := newRepo(t)
	emptyCommit(t, repo, "second")
	emptyCommit(t, repo, "third")

	shas, err := AllCommitSHAs(repo)
	if err != nil {
		t.Fatalf("AllCommitSHAs: %v", err)
	}
	if len(shas) != 3 {
		t.Errorf("got %d shas, want 3: %v", len(shas), shas)
	}
}

// ----- BranchTips -----

func TestBranchTipsMultiple(t *testing.T) {
	repo := newRepo(t)
	mainTip := mustRun(t, repo, "rev-parse", "HEAD")
	mustRun(t, repo, "checkout", "-b", "feat/x")
	featTip := emptyCommit(t, repo, "feat-x-1")

	tips, err := BranchTips(repo)
	if err != nil {
		t.Fatalf("BranchTips: %v", err)
	}
	if tips["main"] != mainTip {
		t.Errorf("main tip = %s, want %s", tips["main"], mainTip)
	}
	if tips["feat/x"] != featTip {
		t.Errorf("feat/x tip = %s, want %s", tips["feat/x"], featTip)
	}
}

// ----- CommitInfoFor -----

func TestCommitInfoForLinear(t *testing.T) {
	repo := newRepo(t)
	sha := emptyCommit(t, repo, "second commit")

	info, err := CommitInfoFor(repo, sha)
	if err != nil {
		t.Fatalf("CommitInfoFor: %v", err)
	}
	if info.SHA != sha {
		t.Errorf("SHA mismatch: got %s, want %s", info.SHA, sha)
	}
	if info.Subject != "second commit" {
		t.Errorf("Subject = %q", info.Subject)
	}
	if !strings.Contains(info.Author, "Orbit Test") {
		t.Errorf("Author = %q, want containing 'Orbit Test'", info.Author)
	}
	if info.AuthoredAt == "" {
		t.Error("AuthoredAt is empty")
	}
	if len(info.Parents) != 1 {
		t.Errorf("parents = %d, want 1 (linear non-initial)", len(info.Parents))
	}
}

func TestCommitInfoForInitial(t *testing.T) {
	repo := newRepo(t)
	sha := mustRun(t, repo, "rev-parse", "HEAD")

	info, err := CommitInfoFor(repo, sha)
	if err != nil {
		t.Fatalf("CommitInfoFor: %v", err)
	}
	if len(info.Parents) != 0 {
		t.Errorf("initial commit parents = %v, want []", info.Parents)
	}
}

func TestCommitInfoForMerge(t *testing.T) {
	repo := newRepo(t)
	mustRun(t, repo, "checkout", "-b", "feat/y")
	emptyCommit(t, repo, "feat-y-1")
	mustRun(t, repo, "checkout", "main")
	emptyCommit(t, repo, "main-2")
	mustRun(t, repo, "merge", "--no-ff", "feat/y", "-m", "merge feat/y")
	mergeSHA := mustRun(t, repo, "rev-parse", "HEAD")

	info, err := CommitInfoFor(repo, mergeSHA)
	if err != nil {
		t.Fatalf("CommitInfoFor: %v", err)
	}
	if len(info.Parents) != 2 {
		t.Errorf("merge commit parents = %v, want 2", info.Parents)
	}
}

// ----- BranchesContaining -----

func TestBranchesContainingFeatOnly(t *testing.T) {
	repo := newRepo(t)
	mustRun(t, repo, "checkout", "-b", "feat/z")
	featSHA := emptyCommit(t, repo, "feat-only")

	branches, err := BranchesContaining(repo, featSHA)
	if err != nil {
		t.Fatalf("BranchesContaining: %v", err)
	}
	found := false
	for _, b := range branches {
		if b == "feat/z" {
			found = true
		}
		if b == "main" {
			t.Errorf("feat-only commit should not appear in main: branches=%v", branches)
		}
	}
	if !found {
		t.Errorf("feat/z not in branches: %v", branches)
	}
}

// ----- BranchCommitsSince -----

func TestBranchCommitsSinceDiff(t *testing.T) {
	repo := newRepo(t)
	mustRun(t, repo, "checkout", "-b", "feat/diff")
	emptyCommit(t, repo, "feat-1")
	emptyCommit(t, repo, "feat-2")

	shas, err := BranchCommitsSince(repo, "feat/diff", "main")
	if err != nil {
		t.Fatalf("BranchCommitsSince: %v", err)
	}
	if len(shas) != 2 {
		t.Errorf("diff commits = %d, want 2: %v", len(shas), shas)
	}
}
