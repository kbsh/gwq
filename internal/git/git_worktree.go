package git

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/d-kuro/gwq/pkg/models"
)

// ListWorktrees returns a list of all worktrees in the repository.
func (g *Git) ListWorktrees() ([]models.Worktree, error) {
	output, err := g.run("worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []models.Worktree
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for i := 0; i < len(lines); i++ {
		if after, ok := strings.CutPrefix(lines[i], "worktree "); ok {
			path := after

			var branch, commitHash string

			for j := i + 1; j < len(lines) && !strings.HasPrefix(lines[j], "worktree "); j++ {
				if after, ok := strings.CutPrefix(lines[j], "branch "); ok {
					branch = after
					// Remove refs/heads/ prefix if present
					branch = strings.TrimPrefix(branch, "refs/heads/")
				} else if after, ok := strings.CutPrefix(lines[j], "HEAD "); ok {
					commitHash = after
				}
				i = j
			}

			if branch == "" {
				branch = g.getCurrentBranch(path)
			}

			info, err := os.Stat(path)
			var createdAt time.Time
			if err == nil {
				createdAt = info.ModTime()
			}

			worktrees = append(worktrees, models.Worktree{
				Path:       path,
				Branch:     branch,
				CommitHash: commitHash,
				CreatedAt:  createdAt,
			})
		}
	}

	// git lists the main worktree, or the bare repository, first.
	if len(worktrees) > 0 {
		worktrees[0].IsMain = true
	}

	return worktrees, nil
}

// AddWorktree creates a new worktree.
//
// If createBranch is true, a new branch is created starting at base
// (HEAD when base is empty). Otherwise the existing branch is checked out
// and base must be empty.
func (g *Git) AddWorktree(path, branch, base string, createBranch bool) error {
	// Refuse values git would parse as options (e.g. a base of "--force").
	for _, v := range []string{branch, base} {
		if strings.HasPrefix(v, "-") {
			return fmt.Errorf("invalid ref %q: must not start with '-'", v)
		}
	}

	args := []string{"worktree", "add"}

	switch {
	case createBranch:
		// Only track base when it is the same-named remote branch
		// (origin/feature -> feature). Otherwise "feature/x from origin/main"
		// would get origin/main as its upstream, and `git pull`/`git push`
		// in the worktree would target main.
		if base != "" && !strings.HasSuffix(base, "/"+branch) {
			args = append(args, "--no-track")
		}
		args = append(args, "-b", branch, path)
		if base != "" {
			args = append(args, base)
		}
	case base != "":
		return fmt.Errorf("base %q requires creating a new branch", base)
	default:
		args = append(args, path, branch)
	}

	if _, err := g.run(args...); err != nil {
		return fmt.Errorf("failed to add worktree: %w", err)
	}

	return nil
}

// RemoveWorktree removes a worktree.
func (g *Git) RemoveWorktree(path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)

	if _, err := g.run(args...); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	return nil
}

// PruneWorktrees removes worktree information for deleted directories.
func (g *Git) PruneWorktrees() error {
	if _, err := g.run("worktree", "prune"); err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}
	return nil
}
