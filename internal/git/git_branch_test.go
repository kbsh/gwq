package git

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBranchLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantOK     bool
		wantName   string
		wantRemote bool
		wantMsg    string
	}{
		{
			name:     "LocalBranch",
			line:     "refs/heads/feature/x|*||2026-01-02 03:04:05 +0900|abc123|Alice|add feature",
			wantOK:   true,
			wantName: "feature/x",
			wantMsg:  "add feature",
		},
		{
			name:       "RemoteBranch",
			line:       "refs/remotes/upstream/feature/x| ||2026-01-02 03:04:05 +0900|abc123|Alice|add feature",
			wantOK:     true,
			wantName:   "upstream/feature/x",
			wantRemote: true,
			wantMsg:    "add feature",
		},
		{
			name:     "SubjectContainsSeparator",
			line:     "refs/heads/main| ||2026-01-02 03:04:05 +0900|abc123|Alice|fix: a | b",
			wantOK:   true,
			wantName: "main",
			wantMsg:  "fix: a | b",
		},
		{
			name:   "RemoteHEADSymref",
			line:   "refs/remotes/origin/HEAD| |refs/remotes/origin/main|2026-01-02 03:04:05 +0900|abc123|Alice|msg",
			wantOK: false,
		},
		{
			name:   "DetachedHEAD",
			line:   "(HEAD detached at abc123)|*||2026-01-02 03:04:05 +0900|abc123|Alice|msg",
			wantOK: false,
		},
		{
			name:   "TooFewFields",
			line:   "refs/heads/main|*",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, ok := parseBranchLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseBranchLine() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if b.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", b.Name, tt.wantName)
			}
			if b.IsRemote != tt.wantRemote {
				t.Errorf("IsRemote = %v, want %v", b.IsRemote, tt.wantRemote)
			}
			if b.LastCommit.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", b.LastCommit.Message, tt.wantMsg)
			}
			if b.LastCommit.Author != "Alice" {
				t.Errorf("Author = %q, want %q", b.LastCommit.Author, "Alice")
			}
		})
	}
}

func TestSplitRemoteBranch(t *testing.T) {
	remotes := []string{"origin", "me", "team", "team/upstream"}

	tests := []struct {
		in         string
		wantRemote string
		wantBranch string
		wantOK     bool
	}{
		{"origin/main", "origin", "main", true},
		{"origin/feature/x", "origin", "feature/x", true},
		{"me/x", "me", "x", true},
		{"team/upstream/feature/y", "team/upstream", "feature/y", true},
		{"team/other", "team", "other", true},
		{"main", "", "", false},
		{"origin/", "", "", false},
		{"unknown/x", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			remote, branch, ok := SplitRemoteBranch(tt.in, remotes)
			if remote != tt.wantRemote || branch != tt.wantBranch || ok != tt.wantOK {
				t.Errorf("SplitRemoteBranch(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.in, remote, branch, ok, tt.wantRemote, tt.wantBranch, tt.wantOK)
			}
		})
	}
}

// newCloneWithRemoteBranch creates an upstream repository with a "feature"
// branch that is one commit ahead of main, clones it, and returns the clone
// together with the commit hash of origin/feature.
func newCloneWithRemoteBranch(t *testing.T) (*TestRepository, string) {
	t.Helper()

	upstream := NewTestRepository(t)
	upstream.CreateBranch(t, "feature")
	if err := upstream.run("commit", "--allow-empty", "-m", "feature commit"); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	if err := upstream.run("checkout", "main"); err != nil {
		t.Fatalf("Failed to checkout main: %v", err)
	}

	clonePath := filepath.Join(t.TempDir(), "clone")
	if err := upstream.run("clone", upstream.Path, clonePath); err != nil {
		t.Fatalf("Failed to clone: %v", err)
	}
	clone := &TestRepository{Path: clonePath}

	return clone, revParse(t, clonePath, "origin/feature")
}

func revParse(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"rev-parse"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func TestListBranchesWithRemote(t *testing.T) {
	clone, _ := newCloneWithRemoteBranch(t)
	g := New(clone.Path)

	branches, err := g.ListBranches(true)
	if err != nil {
		t.Fatalf("ListBranches(true) error = %v", err)
	}

	got := map[string]bool{}
	for _, b := range branches {
		got[b.Name] = b.IsRemote
	}

	if isRemote, ok := got["origin/feature"]; !ok || !isRemote {
		t.Errorf("origin/feature: found=%v isRemote=%v, want found remote branch; branches=%v", ok, isRemote, got)
	}
	if isRemote, ok := got["main"]; !ok || isRemote {
		t.Errorf("main: found=%v isRemote=%v, want found local branch; branches=%v", ok, isRemote, got)
	}
	for _, name := range []string{"origin", "origin/HEAD"} {
		if _, ok := got[name]; ok {
			t.Errorf("symbolic ref %q should not be listed; branches=%v", name, got)
		}
	}
}

// upstreamOf returns the configured upstream of branch, or "" if none.
func upstreamOf(t *testing.T, dir, branch string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func TestListRemotes(t *testing.T) {
	clone, _ := newCloneWithRemoteBranch(t)
	g := New(clone.Path)

	remotes, err := g.ListRemotes()
	if err != nil {
		t.Fatalf("ListRemotes() error = %v", err)
	}
	if len(remotes) != 1 || remotes[0] != "origin" {
		t.Errorf("ListRemotes() = %v, want [origin]", remotes)
	}
}

func TestIsCommitish(t *testing.T) {
	clone, _ := newCloneWithRemoteBranch(t)
	g := New(clone.Path)

	for rev, want := range map[string]bool{
		"main":           true,
		"origin/feature": true,
		"main~0":         true,
		"no-such-branch": false,
		"work/wt":        false,
		"--all":          false,
		"":               false,
	} {
		if got := g.IsCommitish(rev); got != want {
			t.Errorf("IsCommitish(%q) = %v, want %v", rev, got, want)
		}
	}
}

func TestAddWorktreeWithBase(t *testing.T) {
	t.Run("NewBranchFromRemoteBase", func(t *testing.T) {
		clone, featureHash := newCloneWithRemoteBranch(t)
		g := New(clone.Path)

		worktreePath := filepath.Join(t.TempDir(), "wt")
		if err := g.AddWorktree(worktreePath, "my-feature", "origin/feature", true); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		if got := revParse(t, worktreePath, "HEAD"); got != featureHash {
			t.Errorf("worktree HEAD = %s, want origin/feature %s", got, featureHash)
		}
		if got := revParse(t, worktreePath, "--abbrev-ref", "HEAD"); got != "my-feature" {
			t.Errorf("worktree branch = %s, want my-feature", got)
		}
	})

	t.Run("SameNamedRemoteBaseIsTracked", func(t *testing.T) {
		clone, _ := newCloneWithRemoteBranch(t)
		g := New(clone.Path)

		worktreePath := filepath.Join(t.TempDir(), "wt")
		if err := g.AddWorktree(worktreePath, "feature", "origin/feature", true); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}
		if got := upstreamOf(t, clone.Path, "feature"); got != "origin/feature" {
			t.Errorf("upstream of feature = %q, want origin/feature", got)
		}
	})

	t.Run("DifferentNamedRemoteBaseIsNotTracked", func(t *testing.T) {
		clone, _ := newCloneWithRemoteBranch(t)
		g := New(clone.Path)

		worktreePath := filepath.Join(t.TempDir(), "wt")
		if err := g.AddWorktree(worktreePath, "my-feature", "origin/main", true); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}
		if got := upstreamOf(t, clone.Path, "my-feature"); got != "" {
			t.Errorf("upstream of my-feature = %q, want none", got)
		}
	})

	t.Run("RejectsOptionLikeValues", func(t *testing.T) {
		repo := NewTestRepository(t)
		g := New(repo.Path)

		for _, tc := range []struct{ branch, base string }{
			{"feat", "--force"},
			{"-feat", ""},
		} {
			worktreePath := filepath.Join(t.TempDir(), "wt")
			if err := g.AddWorktree(worktreePath, tc.branch, tc.base, true); err == nil {
				t.Errorf("AddWorktree(branch=%q, base=%q) should fail", tc.branch, tc.base)
			}
		}
	})

	t.Run("BaseWithoutCreateBranch", func(t *testing.T) {
		repo := NewTestRepository(t)
		g := New(repo.Path)

		err := g.AddWorktree(filepath.Join(t.TempDir(), "wt"), "main", "HEAD", false)
		if err == nil {
			t.Fatal("AddWorktree() with base and createBranch=false should fail")
		}
	})
}
