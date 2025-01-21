package repo

import (
	"os"
	"testing"

	"github.com/krateoplatformops/git-provider/internal/clients/git"
	"github.com/stretchr/testify/require"
)

func TestCopier(t *testing.T) {
	baseRepo := git.BaseSuite{}
	baseRepo.BuildBasicRepository()
	origin, err := git.Clone(git.CloneOptions{
		URL: baseRepo.GetBasicLocalRepositoryURL(),
	})
	require.NoError(t, err)

	f, _ := origin.FS().OpenFile(".krateoignore", os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)
	f.Write([]byte("*"))
	f.Close()
	_, err = origin.FS().OpenFile("file1.txt", os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)
	_, err = origin.FS().OpenFile("file2.txt", os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)

	targetRepo := git.BaseSuite{}
	targetRepo.BuildBasicRepository()
	target, err := git.Clone(git.CloneOptions{
		URL: targetRepo.GetBasicLocalRepositoryURL(),
	})
	require.NoError(t, err)

	co := newCopier(origin, target, "/", "/")
	co.toRepo.FS().MkdirAll("/", 0755)
	err = loadIgnoreTargetFiles("/", co)
	require.NoError(t, err)
	err = loadIgnoreFileEventually(co)
	require.NoError(t, err)
	err = co.copyDir("/", "/")
	require.NoError(t, err)
}
