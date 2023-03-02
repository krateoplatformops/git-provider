//go:build integration
// +build integration

package git

import (
	"os"
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

func setEnv() {
	env, _ := dotenv.FromFile("../../../.env")
	dotenv.PutInEnv(env, false)
}
