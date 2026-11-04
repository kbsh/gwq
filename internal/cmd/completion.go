package cmd

import (
	"fmt"
	"strings"

	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/registry"
	"github.com/d-kuro/gwq/internal/worktree"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/cobra"
)

// getWorktreeCompletions returns worktree names for shell completion
func getWorktreeCompletions(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	g, err := git.NewFromCwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	wm := worktree.New(g, nil)
	worktrees, err := wm.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	for _, wt := range worktrees {
		if strings.HasPrefix(wt.Branch, toComplete) || strings.HasPrefix(wt.Path, toComplete) {
			desc := fmt.Sprintf("Branch: %s", wt.Branch)
			if wt.Path != "" {
				desc += fmt.Sprintf(" | Path: %s", wt.Path)
			}
			completions = append(completions, fmt.Sprintf("%s\t%s", wt.Branch, desc))
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// getBranchCompletions returns branch names for shell completion of
// `gwq add [branch] [base]`.
func getBranchCompletions(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	g, err := git.NewFromCwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	branches, err := g.ListBranches(true)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if len(args) == 1 {
		return baseCompletions(branches, toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	remotes, err := g.ListRemotes()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return addBranchCompletions(branches, remotes, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// addBranchCompletions completes the [branch] argument of `gwq add`.
// Remote-only branches are offered by their local name ("origin/x" -> "x"):
// `git worktree add <path> x` then creates a local x tracking origin/x,
// whereas "origin/x" itself would give a detached HEAD.
func addBranchCompletions(branches []models.Branch, remotes []string, toComplete string) []string {
	local := map[string]bool{}
	for _, b := range branches {
		if !b.IsRemote {
			local[b.Name] = true
		}
	}

	var completions []string
	seen := map[string]bool{}
	for _, b := range branches {
		name, desc := b.Name, "Local branch"
		if b.IsRemote {
			_, l, ok := git.SplitRemoteBranch(b.Name, remotes)
			if !ok || local[l] {
				continue
			}
			name, desc = l, "Remote branch "+b.Name
		}
		if seen[name] || !strings.HasPrefix(name, toComplete) {
			continue
		}
		seen[name] = true
		completions = append(completions, fmt.Sprintf("%s\t%s", name, desc))
	}
	return completions
}

// baseCompletions completes the [base] argument of `gwq add` with local and
// remote branch names as-is.
func baseCompletions(branches []models.Branch, toComplete string) []string {
	var completions []string
	for _, b := range branches {
		if !strings.HasPrefix(b.Name, toComplete) {
			continue
		}
		desc := "Local branch"
		if b.IsRemote {
			desc = "Remote branch"
		}
		completions = append(completions, fmt.Sprintf("%s\t%s", b.Name, desc))
	}
	return completions
}

// getGlobalWorktreeCompletions returns global worktree names (repo:branch format)
func getGlobalWorktreeCompletions(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	reg, err := registry.New()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	entries := reg.List()

	var completions []string
	for _, entry := range entries {
		fullName := fmt.Sprintf("%s:%s", entry.Repository, entry.Branch)
		if strings.HasPrefix(fullName, toComplete) || strings.HasPrefix(entry.Repository, toComplete) || strings.HasPrefix(entry.Branch, toComplete) {
			completions = append(completions, fmt.Sprintf("%s\tPath: %s", fullName, entry.Path))
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// getConfigKeyCompletions returns config key names for shell completion
func getConfigKeyCompletions(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	keys := []struct {
		name string
		desc string
	}{
		{"worktree.basedir", "Base directory for worktrees"},
		{"worktree.auto_mkdir", "Automatically create directories"},
		{"finder.preview", "Enable preview window"},
		{"finder.preview_size", "Preview window size"},
		{"finder.keybind_select", "Key binding for selection"},
		{"finder.keybind_cancel", "Key binding for cancellation"},
		{"naming.template", "Directory name template"},
		{"ui.color", "Enable colored output"},
		{"ui.icons", "Enable icon display"},
		{"ui.tilde_home", "Display home directory as ~"},
		{"cd.launch_shell", "Launch new shell on cd (default: true)"},
		{"cd.auto_cd_on_add", "Auto-cd after 'gwq add' under shell integration (default: false)"},
	}

	var completions []string
	for _, key := range keys {
		if strings.HasPrefix(key.name, toComplete) {
			completions = append(completions, fmt.Sprintf("%s\t%s", key.name, key.desc))
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// getRemoveCompletions returns only removable (non-main) worktree names for removal
func getRemoveCompletions(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	g, err := git.NewFromCwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	wm := worktree.New(g, nil)
	worktrees, err := wm.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Filter out main worktrees - same logic as fuzzy finder
	var completions []string
	for _, wt := range worktrees {
		if !wt.IsMain && (strings.HasPrefix(wt.Branch, toComplete) || strings.HasPrefix(wt.Path, toComplete)) {
			desc := fmt.Sprintf("Branch: %s", wt.Branch)
			if wt.Path != "" {
				desc += fmt.Sprintf(" | Path: %s", wt.Path)
			}
			completions = append(completions, fmt.Sprintf("%s\t%s", wt.Branch, desc))
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}
