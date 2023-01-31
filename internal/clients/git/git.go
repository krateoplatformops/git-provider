package git

import (
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

const (
	commitAuthorEmail = "krateoctl@krateoplatformops.io"
	commitAuthorName  = "krateoctl"
)

var (
	ErrRepositoryNotFound     = errors.New("repository not found")
	ErrEmptyRemoteRepository  = errors.New("remote repository is empty")
	ErrAuthenticationRequired = errors.New("authentication required")
	ErrAuthorizationFailed    = errors.New("authorization failed")
)

// Repo is an in-memory git repository
type Repo struct {
	rawURL string
	auth   transport.AuthMethod
	storer *memory.Storage
	fs     billy.Filesystem
	repo   *git.Repository
}

func Tags(repoUrl string, auth transport.AuthMethod, insecure bool) ([]string, error) {
	// Create the remote with repository URL
	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoUrl},
	})

	// We can then use every Remote functions to retrieve wanted information
	refs, err := rem.List(&git.ListOptions{
		Auth:            auth,
		InsecureSkipTLS: insecure,
	})
	if err != nil {
		return nil, err
	}

	// Filters the references list and only keeps tags
	var tags []string
	for _, ref := range refs {
		if ref.Name().IsTag() {
			tags = append(tags, ref.Name().Short())
		}
	}

	return tags, nil
}

func Clone(repoUrl string, auth transport.AuthMethod, insecure bool) (*Repo, error) {
	res := &Repo{
		rawURL: repoUrl,
		auth:   auth,
		storer: memory.NewStorage(),
		fs:     memfs.New(),
	}

	// Clone the given repository to the given directory
	var err error
	res.repo, err = git.Clone(res.storer, res.fs, &git.CloneOptions{
		RemoteName:      "origin",
		URL:             repoUrl,
		Auth:            auth,
		InsecureSkipTLS: insecure,
	})
	if err != nil {
		if errors.Is(err, transport.ErrRepositoryNotFound) {
			return nil, ErrRepositoryNotFound
		}

		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return nil, ErrEmptyRemoteRepository
		}

		if errors.Is(err, transport.ErrAuthenticationRequired) {
			return nil, ErrAuthenticationRequired
		}

		if errors.Is(err, transport.ErrAuthorizationFailed) {
			return nil, ErrAuthorizationFailed
		}

		return nil, err
		/*
			h := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.ReferenceName("refs/heads/main"))
			err := res.repo.Storer.SetReference(h)
			if err != nil {
				return nil, err
			}
		*/
	}

	return res, nil
}

func (s *Repo) Exists(path string) (bool, error) {
	_, err := s.fs.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (s *Repo) FS() billy.Filesystem {
	return s.fs
}

func (s *Repo) Branch(name string) error {
	branch := fmt.Sprintf("refs/heads/%s", name)
	ref := plumbing.ReferenceName(branch)

	h := plumbing.NewSymbolicReference(plumbing.HEAD, ref)
	err := s.repo.Storer.SetReference(h)
	if err != nil {
		return err
	}

	wt, err := s.repo.Worktree()
	if err != nil {
		return err
	}

	return wt.Checkout(&git.CheckoutOptions{
		Create: false,
		Branch: ref,
	})
}

func (s *Repo) Commit(path, msg string) (string, error) {
	wt, err := s.repo.Worktree()
	if err != nil {
		return "", err
	}
	// git add $path
	if _, err := wt.Add(path); err != nil {
		return "", err
	}

	// git commit -m $message
	hash, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  commitAuthorName,
			Email: commitAuthorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", err
	}

	return hash.String(), nil
}

func (s *Repo) Push(downstream, branch string, insecure bool) error {
	//Push the code to the remote
	if len(branch) == 0 {
		return s.repo.Push(&git.PushOptions{
			RemoteName:      downstream,
			Auth:            s.auth,
			InsecureSkipTLS: insecure,
		})
	}

	headRef, err := s.repo.Head()
	if err != nil {
		return err
	}

	refName := plumbing.NewBranchReferenceName(branch)

	refs, err := s.repo.References()
	if err != nil {
		return err
	}

	var foundLocal bool
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name() == refName {
			//fmt.Printf("reference exists locally:\n%s\n", ref)
			foundLocal = true
		}
		return nil
	})
	if !foundLocal {
		ref := plumbing.NewHashReference(refName, headRef.Hash())
		err = s.repo.Storer.SetReference(ref)
		if err != nil {
			return err
		}
	}

	return s.repo.Push(&git.PushOptions{
		RemoteName:      downstream,
		Force:           true,
		Auth:            s.auth,
		InsecureSkipTLS: insecure,
		RefSpecs: []config.RefSpec{
			config.RefSpec(refName + ":" + refName),
		},
	})
}

func Pull(s *Repo, insecure bool) error {
	// Get the working directory for the repository
	wt, err := s.repo.Worktree()
	if err != nil {
		return err
	}

	err = wt.Pull(&git.PullOptions{
		RemoteName:      "origin",
		Auth:            s.auth,
		InsecureSkipTLS: insecure,
	})

	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			err = nil
		}
	}

	return err
}

func getHeadCommit(s *Repo) (*object.Commit, error) {
	// retrieve the branch being pointed by HEAD
	ref, err := s.repo.Head()
	if err != nil {
		return nil, err
	}

	// retrieve the commit object
	return s.repo.CommitObject(ref.Hash())
}

func TagExists(tag string, r *git.Repository) (bool, error) {
	//Info("git show-ref --tag")
	tags, err := r.TagObjects()
	if err != nil {
		return false, err
	}

	exists := false
	tagFoundErr := "tag was found"
	err = tags.ForEach(func(t *object.Tag) error {
		if t.Name == tag {
			exists = true
			return fmt.Errorf(tagFoundErr)
		}
		return nil
	})
	if err != nil && err.Error() != tagFoundErr {
		return false, err
	}
	return exists, nil
}

func (s *Repo) CreateTag(tag string) (bool, error) {
	r := s.repo

	exists, err := TagExists(tag, r)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	h, err := r.Head()
	if err != nil {
		return false, err
	}

	//Info("git tag -a %s %s -m \"%s\"", tag, h.Hash(), tag)
	_, err = r.CreateTag(tag, h.Hash(), &git.CreateTagOptions{
		Message: tag,
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *Repo) PushTags(token string, insecure bool) error {
	r := s.repo

	opts := &git.PushOptions{
		RemoteName: "origin",
		//Progress:   os.Stdout,
		RefSpecs: []config.RefSpec{config.RefSpec("refs/tags/*:refs/tags/*")},
		Auth: &http.TokenAuth{
			Token: token,
		},
		InsecureSkipTLS: insecure,
	}
	//Info("git push --tags")
	err := r.Push(opts)
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			//log.Print("origin remote was up to date, no push done")
			return nil
		}
		//log.Printf("push to remote origin error: %s", err)
		return err
	}

	return nil
}
