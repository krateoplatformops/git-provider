package repo

import (
	"context"
	"os"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	repov1alpha1 "github.com/krateoplatformops/git-provider/apis/repo/v1alpha1"
	"github.com/krateoplatformops/git-provider/internal/clients/git"
	"github.com/krateoplatformops/git-provider/internal/ptr"
	commonv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	gi "github.com/sabhiram/go-gitignore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLoadExternalClientOpts(t *testing.T) {
	ctx := context.TODO()
	kc := fake.NewFakeClient()

	// Create the Repo object

	cr := &repov1alpha1.Repo{
		Spec: repov1alpha1.RepoSpec{
			FromRepo: repov1alpha1.FromRepoOpts{
				RepoOpts: repov1alpha1.RepoOpts{
					AuthMethod: ptr.PtrTo("bearer"),
					SecretRef: &commonv1.SecretKeySelector{
						Key: "token",
						Reference: commonv1.Reference{
							Name:      "from-repo-secret",
							Namespace: "default",
						},
					},
				},
			},
			ToRepo: repov1alpha1.RepoOpts{
				AuthMethod: ptr.PtrTo("generic"),
				SecretRef: &commonv1.SecretKeySelector{
					Key: "token",
					Reference: commonv1.Reference{
						Name:      "to-repo-secret",
						Namespace: "default",
					},
				},
			},
			Insecure:                ptr.PtrTo(true),
			UnsupportedCapabilities: ptr.PtrTo(false),
		},
	}

	// Create the secret objects
	fromRepoSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "from-repo-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("from-repo-token"),
		},
	}
	toRepoSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "to-repo-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("to-repo-token"),
		},
	}

	// Create the secrets in the cluster
	require.NoError(t, kc.Create(ctx, fromRepoSecret))
	require.NoError(t, kc.Create(ctx, toRepoSecret))

	opts, err := loadExternalClientOpts(ctx, kc, cr)
	require.NoError(t, err)

	expectedOpts := &externalClientOpts{
		Insecure:                true,
		UnsupportedCapabilities: false,
		FromRepoCreds: &githttp.TokenAuth{
			Token: "from-repo-token",
		},
		ToRepoCreds: &githttp.BasicAuth{
			Username: "krateoctl",
			Password: "to-repo-token",
		},
		FromRepoCookieFile: nil,
		ToRepoCookieFile:   nil,
	}

	assert.Equal(t, expectedOpts, opts)
}

func TestGetRepoCookies(t *testing.T) {
	ctx := context.TODO()
	kc := fake.NewFakeClient()

	// Create the secret object
	secretData := map[string][]byte{
		"cookie": []byte("repo-cookie"),
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo-secret",
			Namespace: "default",
		},
		Data: secretData,
	}

	// Create the secret in the cluster
	require.NoError(t, kc.Create(ctx, secret))

	opts := repov1alpha1.RepoOpts{
		SecretRef: &commonv1.SecretKeySelector{
			Key: "cookie",
			Reference: commonv1.Reference{
				Name:      "repo-secret",
				Namespace: "default",
			},
		},
	}

	cookies, err := getRepoCookies(ctx, kc, opts)
	require.NoError(t, err)

	expectedCookies := []byte("repo-cookie")
	assert.Equal(t, expectedCookies, cookies)
}

func TestLoadIgnoreTargetFiles(t *testing.T) {

	baseRepo := git.BaseSuite{}
	baseRepo.BuildBasicRepository()
	origin, err := git.Clone(git.CloneOptions{
		URL: baseRepo.GetBasicLocalRepositoryURL(),
	})
	require.NoError(t, err)

	targetRepo := git.BaseSuite{}
	targetRepo.BuildBasicRepository()
	target, err := git.Clone(git.CloneOptions{
		URL: baseRepo.GetBasicLocalRepositoryURL(),
	})
	require.NoError(t, err)

	co := newCopier(origin, target, "/", "/")

	srcPath := "/path/to/dir"

	co.toRepo.FS().MkdirAll("/path/to/dir", 0755)
	err = loadIgnoreTargetFiles(srcPath, co)
	require.NoError(t, err)

	expectedIgnore := gi.CompileIgnoreLines()
	co.targetIgnore = expectedIgnore

	assert.Equal(t, expectedIgnore, co.targetIgnore)
}
func TestLoadFilesIntoArray(t *testing.T) {
	fs := memfs.New()

	// Create some test files
	err := fs.MkdirAll("/path/to/dir", 0755)
	require.NoError(t, err)

	f1, err := fs.OpenFile("/path/to/file1.txt", os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)
	_, err = f1.Write([]byte("file1"))
	require.NoError(t, err)
	err = f1.Close()
	require.NoError(t, err)

	f2, err := fs.OpenFile("/path/to/file2.txt", os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)
	_, err = f2.Write([]byte("file2"))
	require.NoError(t, err)
	err = f2.Close()
	require.NoError(t, err)

	f3, err := fs.OpenFile("/path/to/dir/file3.txt", os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)
	_, err = f3.Write([]byte("file3"))
	require.NoError(t, err)
	err = f3.Close()
	require.NoError(t, err)

	var flist []string
	err = loadFilesIntoArray(fs, "/path", &flist)
	require.NoError(t, err)

	expectedFiles := []string{
		"/path/to/file1.txt",
		"/path/to/file2.txt",
		"/path/to/dir/file3.txt",
	}

	assert.ElementsMatch(t, expectedFiles, flist)
}
