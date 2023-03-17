//go:build integration
// +build integration

package git

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/lucasepe/dotenv"
)

func TestClone(t *testing.T) {
	setEnv()

	repo, err := Clone(CloneOptions{
		URL: os.Getenv("URL"),
		Auth: &http.BasicAuth{
			Username: "krateoctl",
			Password: os.Getenv("TOKEN"),
		},
		Insecure:                true,
		UnsupportedCapabilities: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	all, err := repo.FS().ReadDir("/")
	if err != nil {
		t.Fatal(err)
	}

	for _, el := range all {
		t.Log(el.Name())
	}
}

func TestWriteBytes(t *testing.T) {
	setEnv()

	repo, err := Clone(CloneOptions{
		URL: os.Getenv("URL"),
		Auth: &http.BasicAuth{
			Username: "krateoctl",
			Password: os.Getenv("TOKEN"),
		},
		Insecure:                true,
		UnsupportedCapabilities: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := repo.FS().Create("JUST_TO_TEST_A_ISSUE.md")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	in := strings.NewReader("Please be patient...")
	if _, err = io.Copy(out, in); err != nil {
		t.Fatal(err)
	}

	all, err := repo.FS().ReadDir("/")
	if err != nil {
		t.Fatal(err)
	}

	for _, el := range all {
		t.Log(el.Name())
	}

	t.Logf("current branch: %s", repo.CurrentBranch())

	commitId, err := repo.Commit(".", "first commit")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("commitId: %s", commitId)

	err = repo.Push("origin", repo.CurrentBranch(), true)
	if err != nil {
		t.Fatal(err)
	}
}

func setEnv() {
	env, _ := dotenv.FromFile("../../../.env")
	dotenv.PutInEnv(env, false)
}
