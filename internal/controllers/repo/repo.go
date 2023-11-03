package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/cbroglie/mustache"
	"github.com/go-git/go-billy/v5"
	"github.com/pkg/errors"

	commonv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	"github.com/krateoplatformops/provider-runtime/pkg/helpers"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"github.com/krateoplatformops/provider-runtime/pkg/meta"

	"github.com/krateoplatformops/provider-runtime/pkg/reconciler"
	"github.com/krateoplatformops/provider-runtime/pkg/resource"

	repov1alpha1 "github.com/krateoplatformops/git-provider/apis/repo/v1alpha1"
	"github.com/krateoplatformops/git-provider/internal/clients/git"

	gi "github.com/sabhiram/go-gitignore"

	corev1 "k8s.io/api/core/v1"
)

const (
	errNotRepo                         = "managed resource is not a repo custom resource"
	errUnableToLoadConfigMapWithValues = "unable to load configmap with template values"
	errConfigMapValuesNotReadyYet      = "configmap values not ready yet"
)

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube client.Client
	log  logging.Logger
	cfg  *externalClientOpts
	rec  record.EventRecorder
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (reconciler.ExternalObservation, error) {
	cr, ok := mg.(*repov1alpha1.Repo)
	if !ok {
		return reconciler.ExternalObservation{}, errors.New(errNotRepo)
	}

	if cr.Status.TargetCommitId != nil {
		meta.SetExternalName(cr, helpers.String(cr.Status.TargetCommitId))
	}

	if meta.GetExternalName(cr) == "" {
		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, nil
	}
	latestCommit, err := git.GetLatestCommitRemote(git.ListOptions{
		URL:      cr.Spec.FromRepo.Url,
		Auth:     e.cfg.FromRepoCreds,
		Insecure: e.cfg.Insecure,
		Branch:   *cr.Spec.FromRepo.Branch,
	})
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	if helpers.String(latestCommit) != helpers.String(cr.Status.OriginCommitId) {
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
	}

	cr.Status.SetConditions(commonv1.Available())

	return reconciler.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil // e.kube.Status().Update(ctx, cr)
}

func (e *external) Create(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*repov1alpha1.Repo)
	if !ok {
		return errors.New(errNotRepo)
	}
	if !meta.IsActionAllowed(cr, meta.ActionCreate) {
		e.log.Debug("External resource should not be created by provider, skip creating.")
		return nil
	}
	cr.Status.SetConditions(commonv1.Creating())
	return e.SyncRepos(ctx, cr, "first commit")
}

func (e *external) Update(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*repov1alpha1.Repo)
	if !ok {
		return errors.New(errNotRepo)
	}
	if !meta.IsActionAllowed(cr, meta.ActionUpdate) {
		e.log.Debug("External resource should not be updated by provider, skip updating.")
		return nil
	}
	return e.SyncRepos(ctx, cr, "updated target repo with origin repo")
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*repov1alpha1.Repo)
	if !ok {
		return errors.New(errNotRepo)
	}

	cr.Status.SetConditions(commonv1.Deleting())

	return nil // noop
}

func (e *external) loadValuesFromConfigMap(ctx context.Context, ref *commonv1.ConfigMapKeySelector) (map[string]interface{}, error) {
	var res map[string]interface{}

	js, err := resource.GetConfigMapValue(ctx, e.kube, ref)
	if err != nil {
		e.log.Debug(err.Error(), "name", ref.Name, "key", ref.Key, "namespace", ref.Namespace)
		return nil, err
	}

	js = strings.TrimPrefix(js, "'")
	js = strings.TrimSuffix(js, "'")

	err = json.Unmarshal([]byte(js), &res)
	if err != nil {
		e.log.Debug(err.Error(), "json", js)
		return nil, err
	}

	return res, nil
}

func (e *external) SyncRepos(ctx context.Context, cr *repov1alpha1.Repo, commitMessage string) error {

	spec := cr.Spec.DeepCopy()

	toRepo, err := git.Clone(git.CloneOptions{
		URL:                     spec.ToRepo.Url,
		Auth:                    e.cfg.ToRepoCreds,
		Insecure:                e.cfg.Insecure,
		UnsupportedCapabilities: e.cfg.UnsupportedCapabilities,
	})

	if err != nil {
		return errors.Wrapf(err, "cloning toRepo: %s", spec.ToRepo.Url)
	}
	e.log.Debug("Target repo cloned", "url", spec.ToRepo.Url)
	e.rec.Eventf(cr, corev1.EventTypeNormal, "TargetRepoCloned",
		"Successfully cloned target repo: %s", spec.ToRepo.Url)

	branch := "main"
	if spec.ToRepo.Branch != nil {
		branch = helpers.String(spec.FromRepo.Branch)
	}
	isRemote := false
	if branch != "main" {
		isRemote = true
	}
	fromRepo, err := git.Clone(git.CloneOptions{
		URL:                     spec.FromRepo.Url,
		Auth:                    e.cfg.FromRepoCreds,
		Insecure:                e.cfg.Insecure,
		UnsupportedCapabilities: e.cfg.UnsupportedCapabilities,
	})
	if err != nil {
		return err
	}
	e.log.Debug("Origin repo cloned", "url", spec.FromRepo.Url)
	e.rec.Eventf(cr, corev1.EventTypeNormal, "OriginRepoCloned",
		"Successfully cloned origin repo: %s", spec.FromRepo.Url)
	err = fromRepo.Branch(branch, true)
	if err != nil {
		return errors.Wrapf(err, fmt.Sprint("Switching on branch ", "repoUrl ", spec.FromRepo.Url, " branch ", branch))
	}
	e.log.Debug(fmt.Sprintf("Origin repo on branch %s", branch))

	fromRepoCommitId, err := fromRepo.GetLatestCommit(branch, isRemote)
	if err != nil {
		return err
	}

	branch = "main"
	if spec.ToRepo.Branch != nil {
		branch = helpers.String(spec.ToRepo.Branch)
	}
	isRemote = false
	if branch != "main" {
		isRemote = true
	}
	err = toRepo.Branch(branch, isRemote)
	if err != nil {
		return errors.Wrapf(err, fmt.Sprint("Switching on branch ", "repoUrl ", spec.FromRepo.Url, " branch ", branch))
	}
	e.log.Debug(fmt.Sprintf("Target repo on branch %s", branch))

	co := &copier{
		fromRepo: fromRepo,
		toRepo:   toRepo,
	}

	// If fromPath is not specified DON'T COPY!
	fromPath := helpers.String(spec.FromRepo.Path)
	if len(fromPath) > 0 {
		values, err := e.loadValuesFromConfigMap(ctx, spec.ConfigMapKeyRef)
		if err != nil {
			e.log.Debug("Unable to load configmap with template data", "msg", err.Error())
			e.rec.Eventf(cr, corev1.EventTypeWarning, "CannotLoadConfigMap",
				"Unable to load configmap with template data: %s", err.Error())
		}

		e.log.Debug("Loaded values from config map",
			"name", spec.ConfigMapKeyRef.Name,
			"key", spec.ConfigMapKeyRef.Key,
			"namespace", spec.ConfigMapKeyRef.Namespace,
			"values", values,
		)

		err = loadIgnoreTargetFiles(co)
		if err != nil {
			return err
		}

		if err := loadIgnoreFileEventually(co); err != nil {
			e.log.Info("Unable to load '.krateoignore'", "msg", err.Error())
			e.rec.Eventf(cr, corev1.EventTypeWarning, "CannotLoadIgnoreFile",
				"Unable to load '.krateoignore' file: %s", err.Error())
		}

		createRenderFunc(co, values)

		toPath := helpers.String(spec.ToRepo.Path)
		if len(toPath) == 0 {
			toPath = "/"
		}

		if err := co.copyDir(fromPath, toPath); err != nil {
			return err
		}
	}

	e.log.Debug("Origin and target repo synchronized",
		"fromUrl", spec.FromRepo.Url,
		"toUrl", spec.ToRepo.Url,
		"fromPath", helpers.String(spec.FromRepo.Path),
		"toPath", helpers.String(spec.ToRepo.Path))
	e.rec.Eventf(cr, corev1.EventTypeNormal, "RepoSyncSuccess",
		"Origin and target repo synchronized")

	toPath := helpers.String(spec.ToRepo.Path)
	commitId, err := toRepo.Commit(".", commitMessage, &git.IndexOptions{
		OriginRepo: fromRepo,
		FromPath:   fromPath,
		ToPath:     toPath,
	})
	if err == git.NoErrAlreadyUpToDate {
		commitId, err := toRepo.GetLatestCommit(branch, isRemote)
		if err != nil {
			return err
		}
		e.log.Debug("Target repo not commited", "branch", branch, "status", "repository already up-to-date")
		e.rec.Eventf(cr, corev1.EventTypeNormal, "RepoAlreadyUpToDate",
			fmt.Sprintf("Target repo already up-to-date on branch %s", branch))
		meta.SetExternalName(cr, commitId)

		cr.Status.OriginCommitId = helpers.StringPtr(fromRepoCommitId)
		cr.Status.TargetCommitId = helpers.StringPtr(commitId)
		cr.Status.Branch = helpers.StringPtr(branch)
		return e.kube.Status().Update(ctx, cr)
	} else if err != nil {
		return err
	}
	e.log.Debug("Target repo committed", "branch", branch, "commitId", commitId)
	e.rec.Eventf(cr, corev1.EventTypeNormal, "RepoCommitSuccess",
		fmt.Sprintf("Target repo committed on branch %s", branch))

	err = toRepo.Push("origin", branch, e.cfg.Insecure)
	if err != nil {
		return err
	}
	e.log.Debug("Target repo pushed", "branch", branch, "commitId", commitId)
	e.rec.Eventf(cr, corev1.EventTypeNormal, "RepoPushSuccess",
		fmt.Sprintf("Target repo pushed branch %s", branch))

	meta.SetExternalName(cr, commitId)

	cr.Status.OriginCommitId = helpers.StringPtr(fromRepoCommitId)
	cr.Status.TargetCommitId = helpers.StringPtr(commitId)
	cr.Status.Branch = helpers.StringPtr(branch)
	//cr.Status.SetConditions(commonv1.Available())

	return e.kube.Status().Update(ctx, cr)
}

func createRenderFunc(co *copier, values interface{}) {
	co.renderFunc = func(in io.Reader, out io.Writer) error {
		bin, err := io.ReadAll(in)
		if err != nil {
			return err
		}
		tmpl, err := mustache.ParseString(string(bin))
		if err != nil {
			return err
		}

		return tmpl.FRender(out, values)
	}
}

func loadIgnoreFileEventually(co *copier) error {
	fp, err := co.fromRepo.FS().Open(".krateoignore")
	if err != nil {
		return err
	}
	defer fp.Close()

	bs, err := io.ReadAll(fp)
	if err != nil {
		return err
	}

	lines := strings.Split(string(bs), "\n")

	co.krateoIgnore = gi.CompileIgnoreLines(lines...)

	return nil
}

func loadFilesIntoArray(fs billy.Filesystem, dir string, flist *[]string) error {
	files, err := fs.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			err := loadFilesIntoArray(fs, filepath.Join(dir, file.Name()), flist)
			if err != nil {
				return err
			}
		} else {
			absPath := filepath.Join(dir, file.Name())
			*flist = append(*flist, absPath)
		}
	}

	return nil
}

func loadIgnoreTargetFiles(co *copier) error {
	fs := co.toRepo.FS()
	var flist []string
	err := loadFilesIntoArray(fs, ".", &flist)
	if err != nil {
		return err
	}
	co.targetIgnore = gi.CompileIgnoreLines(flist...)
	return nil
}
