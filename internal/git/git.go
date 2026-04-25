// Package git provides a thin wrapper over the git CLI for read-only operations.
// All functions accept a working directory and shell out to `git`.
package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNotARepo is returned when the directory is not inside a git repository.
var ErrNotARepo = errors.New("not a git repository")

// ErrDetachedHEAD is returned when HEAD is detached (no symbolic branch).
var ErrDetachedHEAD = errors.New("detached HEAD")

// CommitInfo describes a single git commit.
type CommitInfo struct {
	SHA        string
	Subject    string
	Author     string
	AuthoredAt string
	Parents    []string
}

// run executes git in dir with args and returns stdout (trimmed of trailing newline).
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// IsRepo reports whether dir is inside a git working tree.
func IsRepo(dir string) bool {
	out, err := run(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// ToplevelDir returns the root directory of the git working tree containing dir.
func ToplevelDir(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", ErrNotARepo
	}
	return strings.TrimSpace(out), nil
}

// CurrentBranch returns the symbolic branch name at HEAD, or ErrDetachedHEAD.
func CurrentBranch(dir string) (string, error) {
	out, err := run(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", ErrDetachedHEAD
	}
	return strings.TrimSpace(out), nil
}

// RemoteURL returns the URL for the given remote (typically "origin").
// Returns empty string and nil error if the remote is not configured.
func RemoteURL(dir, remote string) (string, error) {
	out, err := run(dir, "config", "--get", fmt.Sprintf("remote.%s.url", remote))
	if err != nil {
		// `git config --get` exits 1 when key not set; treat as no-remote.
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

// AllCommitSHAs returns SHAs reachable from any local branch (git rev-list --all).
func AllCommitSHAs(dir string) ([]string, error) {
	out, err := run(dir, "rev-list", "--all")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// BranchTips returns a map of local branch name to its tip SHA.
func BranchTips(dir string) (map[string]string, error) {
	out, err := run(dir, "for-each-ref", "--format=%(refname:short) %(objectname)", "refs/heads")
	if err != nil {
		return nil, err
	}
	tips := map[string]string{}
	if out == "" {
		return tips, nil
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		tips[parts[0]] = parts[1]
	}
	return tips, nil
}

// CommitInfoFor returns metadata for a single commit.
// Format: <SHA>\n<parent SHAs space-separated>\n<author name <email>>\n<authored ISO-8601>\n<subject>
func CommitInfoFor(dir, sha string) (CommitInfo, error) {
	const sep = "\x00"
	format := strings.Join([]string{"%H", "%P", "%an <%ae>", "%aI", "%s"}, sep)
	out, err := run(dir, "show", "--no-patch", "--format="+format, sha)
	if err != nil {
		return CommitInfo{}, err
	}
	parts := strings.SplitN(out, sep, 5)
	if len(parts) < 5 {
		return CommitInfo{}, fmt.Errorf("commit info: unexpected format for %s: %q", sha, out)
	}
	info := CommitInfo{
		SHA:        strings.TrimSpace(parts[0]),
		Author:     strings.TrimSpace(parts[2]),
		AuthoredAt: strings.TrimSpace(parts[3]),
		Subject:    strings.TrimSpace(parts[4]),
	}
	if p := strings.TrimSpace(parts[1]); p != "" {
		info.Parents = strings.Split(p, " ")
	}
	return info, nil
}

// BranchesContaining returns local branches that contain the given commit.
func BranchesContaining(dir, sha string) ([]string, error) {
	out, err := run(dir, "branch", "--list", "--contains", sha, "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	branches := []string{}
	for _, line := range strings.Split(out, "\n") {
		s := strings.TrimSpace(line)
		if s != "" {
			branches = append(branches, s)
		}
	}
	return branches, nil
}

// BranchCommitsSince returns SHAs reachable from branch but not from baseRef.
// e.g. BranchCommitsSince(dir, "feat/x", "main") -> commits unique to feat/x.
func BranchCommitsSince(dir, branch, baseRef string) ([]string, error) {
	out, err := run(dir, "rev-list", branch, "^"+baseRef)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}
