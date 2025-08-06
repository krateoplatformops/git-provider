package git

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/cache"
	gitclient "github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/krateoplatformops/plumbing/ptr"
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
	NoErrAlreadyUpToDate      = git.NoErrAlreadyUpToDate
)

var clientMutex sync.Mutex

type Repo struct {
	rawURL      string
	auth        transport.AuthMethod
	storer      storage.Storer
	fs          billy.Filesystem
	repo        *git.Repository
	isNewBranch *bool
	cookie      []byte
	tmpDir      string
}

type CloneOptions struct {
	URL                     string
	Auth                    transport.AuthMethod
	Insecure                bool
	UnsupportedCapabilities bool
	Branch                  string
	AlternativeBranch       *string
	GitCookies              []byte
	HomeDir                 string // The home directory to use for temporary files
}

type ListOptions struct {
	URL        string
	Auth       transport.AuthMethod
	Insecure   bool
	Branch     string
	GitCookies []byte
	HomeDir    string // The home directory to use for temporary files
}

type IndexOptions struct {
	OriginRepo *Repo
	FromPath   string
	ToPath     string
}

func (repo *Repo) setDefaultHTTPSClient() {
	clientMutex.Lock()
	defer clientMutex.Unlock()
	gitclient.InstallProtocol("https", githttp.NewClient(nil))
}

func (repo *Repo) setCustomHTTPSClientWithCookieJar() error {
	// Initialize a CookieJar to hold our cookies
	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("error creating cookie jar: %w", err)
	}

	repo.cookie = bytes.Trim(repo.cookie, "\n")
	split := bytes.Split(repo.cookie, []byte("\t"))

	if len(split) < 7 {
		return nil
	}

	cookie := &http.Cookie{
		Name:       string(split[5]),
		Value:      string(split[6]),
		RawExpires: string(split[4]),
		Path:       string(split[2]),
		Domain:     string(split[0]),
		Secure:     string(split[3]) == "TRUE",
		HttpOnly:   string(split[1]) == "TRUE",
	}

	jar.SetCookies(
		&url.URL{
			Scheme: "https",
			Host:   cookie.Domain,
		},
		[]*http.Cookie{
			cookie,
		},
	)

	customClient := &http.Client{
		Jar: jar,
	}

	clientMutex.Lock()
	defer clientMutex.Unlock()
	gitclient.InstallProtocol("https", githttp.NewClient(customClient))

	return err
}

func GetLatestCommitRemote(opts ListOptions) (*string, error) {
	tmpDir, err := os.MkdirTemp(opts.HomeDir, "git-provider-list-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	diskFS := osfs.New(tmpDir)
	dotGitFS, err := diskFS.Chroot(".git")
	if err != nil {
		return nil, fmt.Errorf("failed to create .git directory: %w", err)
	}

	storer := filesystem.NewStorage(dotGitFS, cache.NewObjectLRUDefault())
	res := &Repo{
		rawURL: opts.URL,
		auth:   opts.Auth,
		storer: storer,
		fs:     diskFS,
		cookie: opts.GitCookies,
		tmpDir: tmpDir,
	}

	if len(res.cookie) > 0 {
		if err := res.setCustomHTTPSClientWithCookieJar(); err != nil {
			return nil, err
		}
	}
	defer res.setDefaultHTTPSClient()

	res.repo, err = git.Init(res.storer, res.fs)
	if err != nil {
		return nil, err
	}
	remote, err := res.repo.CreateRemote(&config.RemoteConfig{
		URLs: []string{opts.URL},
		Name: "origin",
	})
	if err != nil {
		return nil, err
	}

	refs, err := remote.List(&git.ListOptions{
		Auth:            opts.Auth,
		InsecureSkipTLS: opts.Insecure,
	})
	if err != nil {
		return nil, err
	}
	repoRef := plumbing.NewBranchReferenceName(opts.Branch)
	for _, ref := range refs {
		if ref.Name() == repoRef {
			return ptr.To(ref.Hash().String()), nil
		}
	}

	return nil, fmt.Errorf("Branch %s reference %s not found on remote %s", opts.Branch, repoRef, opts.URL)
}

func restoreUnsupportedCapabilities(oldUnsupportedCaps []capability.Capability) {
	transport.UnsupportedCapabilities = oldUnsupportedCaps
}

func IsInGitCommitHistory(opts ListOptions, hash string) (bool, error) {
	tmpDir, err := os.MkdirTemp(opts.HomeDir, "git-provider-history-*")
	if err != nil {
		return false, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	diskFS := osfs.New(tmpDir)
	dotGitFS, err := diskFS.Chroot(".git")
	if err != nil {
		return false, fmt.Errorf("failed to create .git directory: %w", err)
	}

	storer := filesystem.NewStorage(dotGitFS, cache.NewObjectLRUDefault())

	res := &Repo{
		rawURL: opts.URL,
		auth:   opts.Auth,
		storer: storer,
		fs:     diskFS,
		cookie: opts.GitCookies,
		tmpDir: tmpDir,
	}

	if len(res.cookie) > 0 {
		if err := res.setCustomHTTPSClientWithCookieJar(); err != nil {
			return false, err
		}
	}
	defer res.setDefaultHTTPSClient()

	cloneOpts := git.CloneOptions{
		RemoteName:      "origin",
		URL:             opts.URL,
		Auth:            opts.Auth,
		ReferenceName:   plumbing.NewBranchReferenceName(opts.Branch),
		SingleBranch:    true,
		InsecureSkipTLS: opts.Insecure,
	}

	oldUnsupportedCaps := transport.UnsupportedCapabilities
	defer restoreUnsupportedCapabilities(oldUnsupportedCaps)

	// Azure DevOps requires multi_ack and multi_ack_detailed capabilities, which go-git doesn't
	// implement. But: it's possible to do a full clone by saying it's _not_ _un_supported, in which
	// case the library happily functions so long as it doesn't _actually_ get a multi_ack packet. See
	// https://github.com/go-git/go-git/blob/v5.5.1/_examples/azure_devops/main.go.
	if strings.Contains(opts.URL, "dev.azure.com") {
		transport.UnsupportedCapabilities = []capability.Capability{
			capability.ThinPack,
		}
	}

	res.repo, err = git.Clone(res.storer, res.fs, &cloneOpts)
	if err != nil {
		if strings.Contains(err.Error(), "couldn't find remote ref") {
			return false, nil
		}
		return false, fmt.Errorf("failed to clone repository: %v", err)
	}
	head, err := res.repo.Head()
	if err != nil {
		return false, fmt.Errorf("failed to get HEAD: %v", err)
	}
	iter, err := res.repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return false, fmt.Errorf("failed to get commit history: %v", err)
	}

	// Iterate through the commits
	found := false
	err = iter.ForEach(func(c *object.Commit) error {
		if c.Hash.String() == hash {
			found = true
			return nil
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to iterate through commits: %v", err)
	}
	return found, err
}

/*
The function simulate the application of filemode of each from the origin repo (contained in "IndexOption.FromPath") to the destination repo (to files contained in IndexOption.ToPath)
---- git update-index --chmod
*/
func (s *Repo) UpdateIndex(idx *IndexOptions) error {
	if err := s.setCustomHTTPSClientWithCookieJar(); err != nil {
		return err
	}
	defer s.setDefaultHTTPSClient()

	getIndexRelative := func(basepath, targpath string) string {
		if len(basepath) > 0 && basepath[0] != '/' {
			basepath = fmt.Sprintf("%c%s", '/', basepath)
		}
		if len(targpath) > 0 && targpath[0] != '/' {
			targpath = fmt.Sprintf("%c%s", '/', targpath)
		}
		path, err := filepath.Rel(basepath, targpath)
		if err != nil {
			return targpath
		}
		if path == "." {
			return ""
		}
		return path
	}

	fromIdx, err := idx.OriginRepo.storer.Index()
	if err != nil {
		return err
	}
	toIdx, err := s.storer.Index()
	if err != nil {
		return err
	}
	pattern := path.Join(getIndexRelative("/", idx.ToPath), "*")
	subInd, err := toIdx.Glob(pattern)
	if err != nil {
		return err
	}
	for _, e := range subInd {
		relativeName := getIndexRelative(idx.ToPath, e.Name)
		relativeSrc := getIndexRelative("/", idx.FromPath)

		/* .Entry() return ErrEntryNotFound if there is no match.
		The error is ignored because the destination folder can contain element that are not included in the source repo */
		fromEntry, _ := fromIdx.Entry(path.Join(relativeSrc, relativeName))

		//if Entry doesn't return an element skip to the next without updating
		if fromEntry != nil {
			e.Mode = fromEntry.Mode
		}
	}
	return nil
}
func Clone(opts CloneOptions) (*Repo, error) {
	tmpDir, err := os.MkdirTemp(opts.HomeDir, "git-provider-clone-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	diskFS := osfs.New(tmpDir)

	dotGitFS, err := diskFS.Chroot(".git")
	if err != nil {
		return nil, fmt.Errorf("failed to create .git directory: %w", err)
	}

	storer := filesystem.NewStorage(dotGitFS, cache.NewObjectLRUDefault())
	res := &Repo{
		rawURL: opts.URL,
		auth:   opts.Auth,
		storer: storer,
		fs:     diskFS,
		cookie: opts.GitCookies,
		tmpDir: tmpDir,
	}

	if len(res.cookie) > 0 {
		if err := res.setCustomHTTPSClientWithCookieJar(); err != nil {
			return nil, err
		}
	}

	if opts.UnsupportedCapabilities {
		transport.UnsupportedCapabilities = []capability.Capability{
			capability.ThinPack,
		}
	}

	// Clone the given repository to the given directory
	cloneOpts := git.CloneOptions{
		RemoteName:      "origin",
		URL:             opts.URL,
		Auth:            opts.Auth,
		ReferenceName:   plumbing.NewBranchReferenceName(opts.Branch),
		SingleBranch:    true,
		InsecureSkipTLS: opts.Insecure,
	}
	isOrphan := true
	_, err = GetLatestCommitRemote(ListOptions{
		URL:        opts.URL,
		Auth:       opts.Auth,
		Insecure:   opts.Insecure,
		Branch:     opts.Branch,
		GitCookies: opts.GitCookies,
	})
	if err != nil {
		cloneOpts = git.CloneOptions{
			RemoteName:      "origin",
			URL:             opts.URL,
			Auth:            opts.Auth,
			InsecureSkipTLS: opts.Insecure,
		}
		if opts.AlternativeBranch != nil {
			isOrphan = false
			cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(ptr.Deref(opts.AlternativeBranch, ""))
			cloneOpts.SingleBranch = true
		}
		res.isNewBranch = ptr.To(true)
	}
	if len(res.cookie) > 0 {
		if err := res.setCustomHTTPSClientWithCookieJar(); err != nil {
			return nil, err
		}
	}
	res.repo, err = git.Clone(res.storer, res.fs, &cloneOpts)
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
	}

	err = res.Branch(opts.Branch, &CreateOpt{
		Create: ptr.Deref(res.isNewBranch, false),
		Orphan: isOrphan,
	})

	res.setDefaultHTTPSClient()
	return res, err
}

func (s *Repo) Exists(path string) (bool, error) {
	if err := s.setCustomHTTPSClientWithCookieJar(); err != nil {
		return false, err
	}
	defer s.setDefaultHTTPSClient()
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

func (s *Repo) Cleanup() error {
	if s.tmpDir != "" {
		return os.RemoveAll(s.tmpDir)
	}
	return nil
}

func (s *Repo) CurrentBranch() string {
	if err := s.setCustomHTTPSClientWithCookieJar(); err != nil {
		return ""
	}
	defer s.setDefaultHTTPSClient()
	//head, _ := s.repo.Head()
	head, _ := s.repo.Reference(plumbing.HEAD, false)

	return head.Target().Short()
}

type CreateOpt struct {
	Create bool
	Orphan bool
}

/*
Switch braches or create according to parameters passed in createOpt.
  - if createOpt is `nil` no branch are created and a `git checkout` is performed on branch specified by name
  - if creteOpt is different from nil and createOpt.Create is true a new branch is created checking out from the branch specified during clone - `git checkout -b branch-name`
  - if creteOpt is different from nil and both createOpt.Create and createOpt.Orphan are true a new branch is created from blank with no history or parents - `git switch --orphan branch-name`
*/
func (s *Repo) Branch(name string, createOpt *CreateOpt) error {
	if err := s.setCustomHTTPSClientWithCookieJar(); err != nil {
		return err
	}
	defer s.setDefaultHTTPSClient()

	ref := plumbing.NewBranchReferenceName(name)
	if createOpt != nil && createOpt.Create {
		ref = plumbing.NewBranchReferenceName(name)
		wt, err := s.repo.Worktree()
		if err != nil {
			return err
		}
		if createOpt.Orphan {
			if err := s.repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, ref)); err != nil {
				return err
			}
			// Remove all files in the worktree
			if err := wt.RemoveGlob("*"); err != nil {
				return err
			}
			return err
		}

		return wt.Checkout(&git.CheckoutOptions{
			Create: true,
			Branch: ref,
		})
	}

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

func (s *Repo) Commit(path, msg string, opt *IndexOptions) (string, error) {
	if err := s.setCustomHTTPSClientWithCookieJar(); err != nil {
		return "", fmt.Errorf("failed to set custom HTTPS client: %w", err)
	}
	defer s.setDefaultHTTPSClient()

	wt, err := s.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}
	// git add $path
	if _, err := wt.Add(path); err != nil {
		return "", fmt.Errorf("failed to add file to index: %w", err)
	}

	err = s.UpdateIndex(opt)
	if err != nil {
		return "", fmt.Errorf("failed to update index: %w", err)
	}

	fStatus, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get status of worktree: %w", err)
	}

	if fStatus.IsClean() && !ptr.Deref(s.isNewBranch, false) {
		return "", NoErrAlreadyUpToDate
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
		return "", NoErrAlreadyUpToDate
	}

	return hash.String(), nil
}

func (s *Repo) Push(downstream, branch string, insecure bool) error {
	if err := s.setCustomHTTPSClientWithCookieJar(); err != nil {
		return err
	}
	defer s.setDefaultHTTPSClient()

	//Push the code to the remote
	if len(branch) == 0 {
		return s.repo.Push(&git.PushOptions{
			RemoteName:      downstream,
			Auth:            s.auth,
			InsecureSkipTLS: insecure,
		})
	}
	refName := plumbing.NewBranchReferenceName(branch)

	refs, err := s.repo.References()
	if err != nil {
		return err
	}

	var foundLocal bool
	refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name() == refName {
			foundLocal = true
		}
		return nil
	})

	if !foundLocal {
		headRef, err := s.repo.Head()
		if err != nil {
			return err
		}

		ref := plumbing.NewHashReference(refName, headRef.Hash())
		err = s.repo.Storer.SetReference(ref)
		if err != nil {
			return err
		}
	}

	return s.repo.Push(&git.PushOptions{
		RemoteName:      downstream,
		Force:           false,
		Auth:            s.auth,
		InsecureSkipTLS: insecure,
		RefSpecs: []config.RefSpec{
			config.RefSpec(refName + ":" + refName),
		},
	})
}

func Pull(s *Repo, insecure bool) error {
	if err := s.setCustomHTTPSClientWithCookieJar(); err != nil {
		return err
	}
	defer s.setDefaultHTTPSClient()

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

func (s *Repo) GetLatestCommit(branch string) (string, error) {
	if err := s.setCustomHTTPSClientWithCookieJar(); err != nil {
		return "", err
	}
	defer s.setDefaultHTTPSClient()
	refName := plumbing.NewBranchReferenceName(branch)
	ref, err := s.repo.Reference(refName, true)
	if err != nil {
		return "", err
	}
	return ref.Hash().String(), nil
}
