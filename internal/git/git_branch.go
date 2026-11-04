package git

import (
	"fmt"
	"strings"
	"time"

	"github.com/d-kuro/gwq/pkg/models"
)

// branchFormat is the `git branch --format` used by ListBranches.
// Fields are separated by "|"; subject is last so a "|" inside a commit
// message cannot shift the other fields.
const branchFormat = "%(refname)|%(HEAD)|%(symref)|%(committerdate:iso)|%(objectname)|%(authorname)|%(subject)"

// ListBranches returns a list of all branches.
func (g *Git) ListBranches(includeRemote bool) ([]models.Branch, error) {
	args := []string{"branch", "--format=" + branchFormat}
	if includeRemote {
		args = append(args, "-a")
	}

	output, err := g.run(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	var branches []models.Branch
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		if b, ok := parseBranchLine(line); ok {
			branches = append(branches, b)
		}
	}

	return branches, nil
}

// parseBranchLine parses one line of `git branch --format=branchFormat`.
// It returns false for lines that are not real branches: symbolic refs
// such as refs/remotes/origin/HEAD and a detached HEAD entry.
func parseBranchLine(line string) (models.Branch, bool) {
	parts := strings.SplitN(line, "|", 7)
	if len(parts) < 7 {
		return models.Branch{}, false
	}

	refname := parts[0]
	if parts[2] != "" {
		return models.Branch{}, false
	}

	var name string
	var isRemote bool
	switch {
	case strings.HasPrefix(refname, "refs/heads/"):
		name = strings.TrimPrefix(refname, "refs/heads/")
	case strings.HasPrefix(refname, "refs/remotes/"):
		name = strings.TrimPrefix(refname, "refs/remotes/")
		isRemote = true
	default:
		return models.Branch{}, false
	}

	date, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[3])

	return models.Branch{
		Name:      name,
		IsCurrent: parts[1] == "*",
		IsRemote:  isRemote,
		LastCommit: models.CommitInfo{
			Hash:    parts[4],
			Author:  parts[5],
			Message: parts[6],
			Date:    date,
		},
	}, true
}

// ListRemotes returns the names of the configured remotes.
func (g *Git) ListRemotes() ([]string, error) {
	output, err := g.run("remote")
	if err != nil {
		return nil, fmt.Errorf("failed to list remotes: %w", err)
	}
	return strings.Fields(output), nil
}

// IsCommitish reports whether rev resolves to a commit.
func (g *Git) IsCommitish(rev string) bool {
	if rev == "" || strings.HasPrefix(rev, "-") {
		return false
	}
	_, err := g.run("rev-parse", "--verify", "--quiet", rev+"^{commit}")
	return err == nil
}

// SplitRemoteBranch splits a remote-tracking branch name such as
// "origin/feature/x" into its remote ("origin") and branch ("feature/x").
// Remote names may themselves contain "/", so the longest matching name in
// remotes wins. It returns false if name does not start with any remote.
func SplitRemoteBranch(name string, remotes []string) (remote, branch string, ok bool) {
	for _, r := range remotes {
		b, found := strings.CutPrefix(name, r+"/")
		if found && b != "" && len(r) > len(remote) {
			remote, branch, ok = r, b, true
		}
	}
	return remote, branch, ok
}

// DeleteBranch deletes a branch.
func (g *Git) DeleteBranch(branch string, force bool) error {
	args := []string{"branch"}
	if force {
		args = append(args, "-D")
	} else {
		args = append(args, "-d")
	}
	args = append(args, branch)

	if _, err := g.run(args...); err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branch, err)
	}

	return nil
}

// getCurrentBranch returns the current branch name for a specific worktree.
func (g *Git) getCurrentBranch(worktreePath string) string {
	oldWorkDir := g.workDir
	g.workDir = worktreePath
	defer func() { g.workDir = oldWorkDir }()

	output, err := g.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}
