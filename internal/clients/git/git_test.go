package git

import (
	"os"
	"testing"

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
