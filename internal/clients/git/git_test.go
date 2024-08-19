package git

import (
	"testing"
)

func TestIsInGitCommitHistory(t *testing.T) {

	baseRepo := BaseSuite{}
	baseRepo.BuildBasicRepository()
	// origin, err := git.Clone(git.CloneOptions{
	// 	URL: baseRepo.GetBasicLocalRepositoryURL(),
	// })
	// require.NoError(t, err)

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
