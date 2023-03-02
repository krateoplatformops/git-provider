//go:build integration
// +build integration

package git

import (
	"os"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
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
	os.Setenv("URL", "https://Kiratech-BancaSella@dev.azure.com/Kiratech-BancaSella/Test%20project%20created%20by%20Krateo/_git/AZ%20DevOps%20Krateo%20Provider%20Repo")
	os.Setenv("TOKEN", "feqra3spfjkfma6bbnb5nuslfae7wvoagh3lhjc56xhuch2nyiaa")
}
