package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/d-kuro/gwq/internal/worktree"
	"github.com/d-kuro/gwq/pkg/models"
)

func TestResolveAddArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		createBranch bool
		want         worktree.AddOptions
		errContains  string
	}{
		{
			name: "ExistingBranch",
			args: []string{"feature/x"},
			want: worktree.AddOptions{Branch: "feature/x"},
		},
		{
			name:         "NewBranchFromHEAD",
			args:         []string{"feature/x"},
			createBranch: true,
			want:         worktree.AddOptions{Branch: "feature/x", CreateBranch: true},
		},
		{
			name: "BaseImpliesCreateBranch",
			args: []string{"feature/x", "origin/main"},
			want: worktree.AddOptions{Branch: "feature/x", Base: "origin/main", CreateBranch: true},
		},
		{
			name:         "BaseWithExplicitCreateBranch",
			args:         []string{"feature/x", "origin/main"},
			createBranch: true,
			want:         worktree.AddOptions{Branch: "feature/x", Base: "origin/main", CreateBranch: true},
		},
		{
			name: "BaseRevisionWithTilde",
			args: []string{"feature/x", "main~1"},
			want: worktree.AddOptions{Branch: "feature/x", Base: "main~1", CreateBranch: true},
		},
		{
			name:        "NoArgs",
			args:        nil,
			errContains: "branch name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAddArgs(tt.args, tt.createBranch)
			if tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("resolveAddArgs() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveAddArgs() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveAddArgs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateBase(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	if err := os.MkdirAll(filepath.Join(cwd, "work", "wt"), 0755); err != nil {
		t.Fatal(err)
	}

	valid := map[string]bool{"origin/main": true, "main~1": true}
	isCommitish := func(rev string) bool { return valid[rev] }

	tests := []struct {
		name        string
		base        string
		errContains string
	}{
		{name: "RemoteBranch", base: "origin/main"},
		{name: "RevisionWithTilde", base: "main~1"},
		{name: "AbsolutePath", base: "/home/u/projects/x", errContains: "use --path"},
		{name: "TildePath", base: "~/projects/x", errContains: "use --path"},
		{name: "DotRelativePath", base: "../x", errContains: "use --path"},
		{name: "PlainRelativeExistingPath", base: "work/wt", errContains: "looks like a path"},
		{name: "UnknownRef", base: "no-such-branch", errContains: "not a valid branch or commit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBase(tt.base, isCommitish)
			if tt.errContains == "" {
				if err != nil {
					t.Errorf("validateBase(%q) error = %v", tt.base, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("validateBase(%q) error = %v, want error containing %q", tt.base, err, tt.errContains)
			}
		})
	}
}

func TestResolveSelectedBranch(t *testing.T) {
	all := []models.Branch{
		{Name: "main"},
		{Name: "existing"},
		{Name: "origin/main", IsRemote: true},
		{Name: "origin/existing", IsRemote: true},
		{Name: "upstream/feature/x", IsRemote: true},
		{Name: "me/x", IsRemote: true},
		{Name: "team/upstream/feature/y", IsRemote: true},
		{Name: "unknown/z", IsRemote: true},
	}
	remotes := []string{"origin", "upstream", "me", "team/upstream"}

	tests := []struct {
		name         string
		selected     models.Branch
		createBranch bool
		want         worktree.AddOptions
		wantErr      bool
	}{
		{
			name:     "LocalBranch",
			selected: models.Branch{Name: "main"},
			want:     worktree.AddOptions{Branch: "main"},
		},
		{
			name:         "LocalBranchWithCreateFlag",
			selected:     models.Branch{Name: "main"},
			createBranch: true,
			want:         worktree.AddOptions{Branch: "main", CreateBranch: true},
		},
		{
			name:     "RemoteBranchWithoutLocal",
			selected: models.Branch{Name: "upstream/feature/x", IsRemote: true},
			want:     worktree.AddOptions{Branch: "feature/x", Base: "upstream/feature/x", CreateBranch: true},
		},
		{
			name:     "RemoteWithShortName",
			selected: models.Branch{Name: "me/x", IsRemote: true},
			want:     worktree.AddOptions{Branch: "x", Base: "me/x", CreateBranch: true},
		},
		{
			name:     "RemoteNameContainsSlash",
			selected: models.Branch{Name: "team/upstream/feature/y", IsRemote: true},
			want:     worktree.AddOptions{Branch: "feature/y", Base: "team/upstream/feature/y", CreateBranch: true},
		},
		{
			name:     "RemoteBranchWithExistingLocal",
			selected: models.Branch{Name: "origin/existing", IsRemote: true},
			want:     worktree.AddOptions{Branch: "existing"},
		},
		{
			name:         "RemoteBranchWithExistingLocalAndCreateFlag",
			selected:     models.Branch{Name: "origin/existing", IsRemote: true},
			createBranch: true,
			want:         worktree.AddOptions{Branch: "existing", Base: "origin/existing", CreateBranch: true},
		},
		{
			name:     "UnknownRemote",
			selected: models.Branch{Name: "unknown/z", IsRemote: true},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveSelectedBranch(tt.selected, all, remotes, tt.createBranch)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveSelectedBranch() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSelectedBranch() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveSelectedBranch() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAddBranchCompletions(t *testing.T) {
	branches := []models.Branch{
		{Name: "main"},
		{Name: "feature/a"},
		{Name: "origin/main", IsRemote: true},
		{Name: "origin/feature/b", IsRemote: true},
		{Name: "upstream/feature/b", IsRemote: true},
	}
	remotes := []string{"origin", "upstream"}

	got := addBranchCompletions(branches, remotes, "")
	want := []string{
		"main\tLocal branch",
		"feature/a\tLocal branch",
		"feature/b\tRemote branch origin/feature/b",
	}
	if !slices.Equal(got, want) {
		t.Errorf("addBranchCompletions() = %q, want %q", got, want)
	}

	got = addBranchCompletions(branches, remotes, "feature/b")
	if !slices.Equal(got, want[2:]) {
		t.Errorf("addBranchCompletions(prefix) = %q, want %q", got, want[2:])
	}
}

func TestBaseCompletions(t *testing.T) {
	branches := []models.Branch{
		{Name: "main"},
		{Name: "origin/main", IsRemote: true},
	}

	got := baseCompletions(branches, "origin/")
	want := []string{"origin/main\tRemote branch"}
	if !slices.Equal(got, want) {
		t.Errorf("baseCompletions() = %q, want %q", got, want)
	}
}
