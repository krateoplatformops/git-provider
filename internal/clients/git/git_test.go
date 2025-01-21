package git

import (
	"os"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
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

func TestPush(t *testing.T) {
	baseRepo := BaseSuite{}
	baseRepo.BuildBasicRepository()

	repo, err := Clone(CloneOptions{
		URL: baseRepo.GetBasicLocalRepositoryURL(),
	})
	require.NoError(t, err)

	err = repo.Push("origin", "master", false)
	require.ErrorIs(t, err, git.NoErrAlreadyUpToDate)

	_, err = repo.FS().OpenFile("README.md", os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)

	repo.storer.IndexStorage = memory.IndexStorage{}
	_, err = repo.Commit(".", "Initial commit", &IndexOptions{
		OriginRepo: repo,
		FromPath:   "/",
		ToPath:     "/",
	})
	require.NoError(t, err)

	err = repo.Push("origin", "test", false)
	require.NoError(t, err)
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
