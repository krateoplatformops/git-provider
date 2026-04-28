package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/krateoplatformops/plumbing/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsInGitCommitHistory(t *testing.T) {
	baseRepo := BaseSuite{}
	baseRepo.BuildBasicRepository()

	opts := ListOptions{
		URL: baseRepo.GetBasicLocalRepositoryURL(),
	}

	hash := "0123456789abcdef0123456789abcdef01234567"

	exists, err := IsInGitCommitHistory(opts, hash)
	if err != nil {
		t.Errorf("Error checking commit history: %v", err)
	}

	if exists {
		t.Logf("Commit %s exists in the Git repository", hash)
	} else {
		t.Logf("Commit %s does not exist in the Git repository", hash)
	}
}

func TestGetLatestCommitRemote(t *testing.T) {
	baseRepo := BaseSuite{}
	baseRepo.BuildBasicRepository()

	repo, err := Clone(CloneOptions{
		URL: baseRepo.GetBasicLocalRepositoryURL(),
	})
	require.NoError(t, err)

	_, err = repo.FS().OpenFile("README.md", os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)

	commit, err := GetLatestCommitRemote(ListOptions{
		URL:    baseRepo.GetBasicLocalRepositoryURL(),
		Branch: "master",
	})
	require.NoError(t, err)
	expected := "6ecf0ef2c2dffb796033e5a02219af86ec6584e5"
	assert.Equal(t, expected, *commit)

	commit, err = GetLatestCommitRemote(ListOptions{
		URL:    baseRepo.GetBasicLocalRepositoryURL(),
		Branch: "branch",
	})
	require.NoError(t, err)
	expected = "e8d3ffab552895c19b9fcf7aa264d277cde33881"
	assert.Equal(t, expected, *commit)
}

func TestPull(t *testing.T) {
	baseRepo := BaseSuite{}
	baseRepo.BuildBasicRepository()

	repo, err := Clone(CloneOptions{
		URL:    baseRepo.GetBasicLocalRepositoryURL(),
		Branch: "master",
	})
	require.NoError(t, err)

	err = Pull(repo, false)
	require.NoError(t, err)
}

func TestBranch(t *testing.T) {
	baseRepo := BaseSuite{}
	baseRepo.BuildBasicRepository()

	repo, err := Clone(CloneOptions{
		URL:    baseRepo.GetBasicLocalRepositoryURL(),
		Branch: "master",
	})
	require.NoError(t, err)

	err = repo.Branch("test", &CreateOpt{
		Create: true,
		Orphan: false,
	})
	require.NoError(t, err)

	err = repo.Branch("test-orphan", &CreateOpt{
		Create: true,
		Orphan: true,
	})
	require.NoError(t, err)

	err = repo.Branch("test", nil)
	require.NoError(t, err)
}

func TestCurrentBranch(t *testing.T) {
	baseRepo := BaseSuite{}
	baseRepo.BuildBasicRepository()

	repo, err := Clone(CloneOptions{
		URL:    baseRepo.GetBasicLocalRepositoryURL(),
		Branch: "master",
	})
	require.NoError(t, err)

	branch := repo.CurrentBranch()
	assert.Equal(t, "master", branch)
}

func TestGetLatestCommit(t *testing.T) {
	baseRepo := BaseSuite{}
	baseRepo.BuildBasicRepository()

	repo, err := Clone(CloneOptions{
		URL: baseRepo.GetBasicLocalRepositoryURL(),
	})
	require.NoError(t, err)

	commit, err := repo.GetLatestCommit("master")
	require.NoError(t, err)
	expected := "6ecf0ef2c2dffb796033e5a02219af86ec6584e5"
	assert.Equal(t, expected, commit)
}
func TestPush(t *testing.T) {
	baseRepo := BaseSuite{}
	baseRepo.BuildBasicRepository()

	t.Run("push without branch", func(t *testing.T) {
		repo, err := Clone(CloneOptions{
			URL:    baseRepo.GetBasicLocalRepositoryURL(),
			Branch: "master",
		})
		require.NoError(t, err)
		defer repo.Cleanup()

		// Create a new file with content
		file, err := repo.FS().OpenFile("test.txt", os.O_RDWR|os.O_CREATE, 0644)
		require.NoError(t, err)
		_, err = file.Write([]byte("test content"))
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		_, err = repo.Commit("test.txt", "Add test file", &IndexOptions{
			OriginRepo: repo,
			FromPath:   "/",
			ToPath:     "/",
		})
		require.NoError(t, err)

		// Push to a different branch to avoid the "currently checked out" error
		err = repo.Push("origin", "test-branch", false)
		require.NoError(t, err)
	})

	t.Run("push with specific branch", func(t *testing.T) {
		repo, err := Clone(CloneOptions{
			URL:    baseRepo.GetBasicLocalRepositoryURL(),
			Branch: "master",
		})
		require.NoError(t, err)
		defer repo.Cleanup()

		// Create a new branch
		err = repo.Branch("feature", &CreateOpt{
			Create: true,
			Orphan: false,
		})
		require.NoError(t, err)

		// Create a new file with content
		file, err := repo.FS().OpenFile("feature.txt", os.O_RDWR|os.O_CREATE, 0644)
		require.NoError(t, err)
		_, err = file.Write([]byte("feature content"))
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		_, err = repo.Commit("feature.txt", "Add feature file", &IndexOptions{
			OriginRepo: repo,
			FromPath:   "/",
			ToPath:     "/",
		})
		require.NoError(t, err)

		err = repo.Push("origin", "feature", false)
		require.NoError(t, err)
	})

	t.Run("push with insecure option", func(t *testing.T) {
		repo, err := Clone(CloneOptions{
			URL:    baseRepo.GetBasicLocalRepositoryURL(),
			Branch: "master",
		})
		require.NoError(t, err)
		defer repo.Cleanup()

		// Create a new file with content
		file, err := repo.FS().OpenFile("insecure_test.txt", os.O_RDWR|os.O_CREATE, 0644)
		require.NoError(t, err)
		_, err = file.Write([]byte("insecure test content"))
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		_, err = repo.Commit("insecure_test.txt", "Add insecure test file", &IndexOptions{
			OriginRepo: repo,
			FromPath:   "/",
			ToPath:     "/",
		})
		require.NoError(t, err)

		// Push to a different branch to avoid the "currently checked out" error
		err = repo.Push("origin", "insecure-test-branch", true)
		require.NoError(t, err)
	})

	t.Run("push non-existing local branch", func(t *testing.T) {
		repo, err := Clone(CloneOptions{
			URL:    baseRepo.GetBasicLocalRepositoryURL(),
			Branch: "master",
		})
		require.NoError(t, err)
		defer repo.Cleanup()

		// Create a new file with content on master
		file, err := repo.FS().OpenFile("master_file.txt", os.O_RDWR|os.O_CREATE, 0644)
		require.NoError(t, err)
		_, err = file.Write([]byte("master file content"))
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		_, err = repo.Commit("master_file.txt", "Add master file", &IndexOptions{
			OriginRepo: repo,
			FromPath:   "/",
			ToPath:     "/",
		})
		require.NoError(t, err)

		// Try to push to a branch that doesn't exist locally yet
		err = repo.Push("origin", "new-branch", false)
		require.NoError(t, err)
	})
}

// createLocalBareRepo creates a bare git repo with one commit on "main",
// suitable for use as a file:// remote in tests.
func createLocalBareRepo(t *testing.T) string {
	t.Helper()

	workDir := t.TempDir()
	bareDir := t.TempDir()

	repo, err := gogit.PlainInit(workDir, false)
	if err != nil {
		t.Fatalf("init working repo: %v", err)
	}

	// Write a file so we have something to commit
	err = os.WriteFile(filepath.Join(workDir, "README.md"), []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("write README: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("get worktree: %v", err)
	}

	if _, err = wt.Add("README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}

	if _, err = wt.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	}); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	// Clone as a bare repo — this becomes our "remote"
	if _, err = gogit.PlainClone(bareDir, true, &gogit.CloneOptions{
		URL: workDir,
	}); err != nil {
		t.Fatalf("clone bare: %v", err)
	}

	return bareDir
}

// TestCloneEmptyAlternativeBranch_ProducesInvalidRef reproduces the bug where
// passing ptr.To("") as AlternativeBranch (which is what SyncRepos does when
// spec.toRepo.cloneFromBranch is unset) causes Clone to construct the malformed
// refname "refs/heads/heads/" and fail with a confusing error.
//
// Before the fix: error contains "refs/heads/heads/"
// After the fix:  Clone succeeds and creates an orphan branch, or fails with a
//
//	clear "branch does not exist" message — never "refs/heads/heads/"
func TestCloneEmptyAlternativeBranch_ProducesInvalidRef(t *testing.T) {
	bareDir := createLocalBareRepo(t)
	repoURL := "file://" + bareDir

	// "newbranch" does not exist on the remote, so GetLatestCommitRemote will
	// fail and Clone will fall into the AlternativeBranch code path.
	_, err := Clone(CloneOptions{
		URL:    repoURL,
		Branch: "newbranch",
		// This is exactly what SyncRepos passes when CloneFromBranch == "":
		//   AlternativeBranch: ptr.To(cr.Spec.ToRepo.CloneFromBranch)
		// ptr.To("") is non-nil, so the nil guard inside Clone does not fire,
		// and plumbing.NewBranchReferenceName("") = "refs/heads/" is used,
		// producing the malformed ref "refs/heads/heads/" at clone time.
		AlternativeBranch: ptr.To(""),
		HomeDir:           t.TempDir(),
	})

	// Pre-fix, we expect an error about the malformed ref. Post-fix, we expect no error, or at worst an error about the branch not existing (but never the malformed ref).
	if err != nil {
		t.Logf("Clone failed as expected, got error: %v", err)
		if !assert.Contains(t, err.Error(), "refs/heads/heads/") {
			t.Errorf("expected error to contain 'refs/heads/heads/', got: %v", err)
		} else {
			t.Errorf("expected error to NOT contain 'refs/heads/heads/' after the fix, got: %v", err)
			t.Fatal("expected Clone to succede after bug-fix for malformed-ref error, but it failed")
		}
	}
}

// TestCloneNilAlternativeBranch_CreatesOrphanBranch is the companion to the
// test above. It shows what the CORRECT behaviour looks like: when
// AlternativeBranch is nil (no CloneFromBranch set), the branch is created as
// an orphan and Clone succeeds.
//
// This test should pass both before and after the fix. If it starts failing
// the fix has broken the orphan-branch creation path.
func TestCloneNilAlternativeBranch_CreatesOrphanBranch(t *testing.T) {
	bareDir := createLocalBareRepo(t)
	repoURL := "file://" + bareDir

	repo, err := Clone(CloneOptions{
		URL:               repoURL,
		Branch:            "newbranch",
		AlternativeBranch: nil, // nil → orphan path, no malformed ref
		HomeDir:           t.TempDir(),
	})

	if err != nil {
		t.Fatalf("expected Clone to succeed with an orphan branch, got: %v", err)
	}
	defer repo.Cleanup()

	if repo.CurrentBranch() != "newbranch" {
		t.Errorf("expected current branch to be %q, got %q", "newbranch", repo.CurrentBranch())
	}
}
