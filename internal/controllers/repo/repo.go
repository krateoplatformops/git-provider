package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	commonv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"github.com/krateoplatformops/provider-runtime/pkg/meta"

	"github.com/krateoplatformops/provider-runtime/pkg/reconciler"
	"github.com/krateoplatformops/provider-runtime/pkg/resource"

	repov1alpha1 "github.com/krateoplatformops/git-provider/apis/repo/v1alpha1"
	"github.com/krateoplatformops/git-provider/internal/clients/git"
	"github.com/krateoplatformops/plumbing/ptr"

	corev1 "k8s.io/api/core/v1"
)

const (
	errNotRepo = "managed resource is not a repo custom resource"
)

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube client.Client
	log  logging.Logger
	cfg  *externalClientOpts
	rec  record.EventRecorder
}

var homeDir string

func (e *external) Observe(ctx context.Context, mg resource.Managed) (reconciler.ExternalObservation, error) {
	cr, ok := mg.(*repov1alpha1.Repo)
	if !ok {
		return reconciler.ExternalObservation{}, errors.New(errNotRepo)
	}
	e.log.Info("Observing resource")

	if cr.GetCondition(commonv1.TypeReady).Reason == commonv1.ReasonDeleting {
		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, nil
	}

	if !cr.DeletionTimestamp.IsZero() && cr.GetCondition(commonv1.TypeSynced).Reason == commonv1.ReasonReconcileError {
		if !meta.IsActionAllowed(cr, meta.ActionDelete) {
			e.log.Debug("External resource should not be deleted by provider, skip deleting.")
		} else {
			return reconciler.ExternalObservation{
				ResourceExists:   false,
				ResourceUpToDate: true,
			}, nil
		}
	}

	if !cr.Spec.EnableUpdate && cr.Status.TargetCommitId != "" && cr.Status.OriginCommitId != "" && cr.Status.TargetBranch != "" && cr.Status.OriginBranch != "" {
		e.log.Debug("External resource should not be observed by provider, skip observing. EnableUpdate is false.", "name", cr.Name)
		cr.Status.SetConditions(commonv1.Available())
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, e.kube.Status().Update(ctx, cr)
	}

	if cr.Status.TargetCommitId != "" {
		meta.SetExternalName(cr, cr.Status.TargetCommitId)
	}

	if meta.GetExternalName(cr) == "" {
		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, nil
	}
	latestCommit, err := git.GetLatestCommitRemote(git.ListOptions{
		URL:        cr.Spec.FromRepo.Url,
		Auth:       e.cfg.FromRepoCreds,
		Insecure:   e.cfg.Insecure,
		Branch:     cr.Spec.FromRepo.Branch,
		GitCookies: e.cfg.FromRepoCookieFile,
		HomeDir:    homeDir, // Use the configured home directory for temporary files
	})

	if err != nil {
		e.log.Debug("Unable to get latest commit from origin remote repository", "msg", err.Error())
		return reconciler.ExternalObservation{}, err
	}

	isTargetRepoSynced, err := git.IsInGitCommitHistory(git.ListOptions{
		URL:        cr.Spec.ToRepo.Url,
		Auth:       e.cfg.ToRepoCreds,
		Insecure:   e.cfg.Insecure,
		Branch:     cr.Spec.ToRepo.Branch,
		GitCookies: e.cfg.ToRepoCookieFile,
		HomeDir:    homeDir, // Use the configured home directory for temporary files
	}, cr.Status.TargetCommitId)
	if err != nil {
		e.log.Debug("Unable to check if target repo is synced", "msg", err.Error())
		return reconciler.ExternalObservation{}, err
	}

	if ptr.Deref(latestCommit, "") != cr.Status.OriginCommitId {
		e.log.Debug("Origin commit not found in origin remote repository", "commitId", cr.Status.OriginCommitId, "branch", cr.Status.OriginBranch)
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
	}

	if !isTargetRepoSynced {
		e.log.Debug("Target commit not found in target remote repository", "commitId", cr.Status.TargetCommitId, "branch", cr.Status.TargetBranch)
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
	}

	cr.Status.SetConditions(commonv1.Available())

	return reconciler.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
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
	e.log.Info("Creating resource")
	cr.Status.SetConditions(commonv1.Creating())
	return e.SyncRepos(ctx, cr, "first commit")
}

func (e *external) Update(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*repov1alpha1.Repo)
	if !ok {
		return errors.New(errNotRepo)
	}

	if !cr.Spec.EnableUpdate {
		e.log.Debug("External resource should not be updated by provider, skip updating.")
		return nil
	}
	if !meta.IsActionAllowed(cr, meta.ActionUpdate) {
		e.log.Debug("External resource should not be updated by provider, skip updating.")
		return nil
	}

	e.log.Info("Updating resource")
	cr.Status.SetConditions(commonv1.Creating())
	return e.SyncRepos(ctx, cr, "updated target repo with origin repo")
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*repov1alpha1.Repo)
	if !ok {
		return errors.New(errNotRepo)
	}
	if !meta.IsActionAllowed(cr, meta.ActionDelete) {
		e.log.Debug("External resource should not be deleted by provider, skip deleting.")
		return nil
	}

	e.log.Info("Deleting resource")

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
		Branch:                  spec.ToRepo.Branch,
		AlternativeBranch:       ptr.To(cr.Spec.ToRepo.CloneFromBranch),
		GitCookies:              e.cfg.ToRepoCookieFile,
		HomeDir:                 homeDir, // Use the configured home directory for temporary files
	})
	if err != nil {
		return fmt.Errorf("cloning toRepo: %w", err)
	}
	defer toRepo.Cleanup()

	e.log.Debug("Target repo cloned", "url", spec.ToRepo.Url)
	e.rec.Eventf(cr, corev1.EventTypeNormal, "TargetRepoCloned",
		"Successfully cloned target repo: %s", spec.ToRepo.Url)
	e.log.Debug(fmt.Sprintf("Target repo on branch %s", toRepo.CurrentBranch()))

	fromRepo, err := git.Clone(git.CloneOptions{
		URL:                     spec.FromRepo.Url,
		Auth:                    e.cfg.FromRepoCreds,
		Insecure:                e.cfg.Insecure,
		UnsupportedCapabilities: e.cfg.UnsupportedCapabilities,
		Branch:                  spec.FromRepo.Branch,
		GitCookies:              e.cfg.FromRepoCookieFile,
		HomeDir:                 homeDir, // Use the configured home directory for temporary files
	})
	if err != nil {
		return fmt.Errorf("cloning fromRepo: %w", err)
	}
	defer fromRepo.Cleanup()
	e.log.Debug("Origin repo cloned", "url", spec.FromRepo.Url)
	e.rec.Eventf(cr, corev1.EventTypeNormal, "OriginRepoCloned",
		"Successfully cloned origin repo: %s", spec.FromRepo.Url)
	e.log.Debug(fmt.Sprintf("Origin repo on branch %s", fromRepo.CurrentBranch()))

	fromRepoCommitId, err := fromRepo.GetLatestCommit(fromRepo.CurrentBranch())
	if err != nil {
		return err
	}

	co := newCopier(fromRepo, toRepo, spec.FromRepo.Path, spec.ToRepo.Path)

	// If fromPath is not specified DON'T COPY!
	fromPath := spec.FromRepo.Path
	toPath := spec.ToRepo.Path
	if len(toPath) == 0 {
		toPath = "/"
	}
	if len(fromPath) == 0 {
		fromPath = "/"
	}
	if len(fromPath) > 0 {
		var values map[string]interface{}
		if spec.ConfigMapKeyRef != nil {
			values, err = e.loadValuesFromConfigMap(ctx, spec.ConfigMapKeyRef)
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
		}

		if !cr.Spec.Override {
			e.log.Debug("Override is false, ignoring files that already exist in target repo")
			if _, err := toRepo.FS().Stat(toPath); err == nil {
				err = loadIgnoreTargetFiles(toPath, co)
				if err != nil {
					return fmt.Errorf("unable to load ignore target files: %w", err)
				}
			} else if os.IsNotExist(err) {
				e.log.Debug("Target path does not exist, no files to ignore", "path", toPath)
			} else {
				return fmt.Errorf("unable to check target path: %w", err)
			}
		} else {
			co.targetIgnore = nil
			e.log.Debug("Override is true, overriding all files in target repo")
			if co.originCopyPath == "/" && co.targetCopyPath == "/" {
				e.rec.Eventf(cr, corev1.EventTypeWarning, "OverrideWarning",
					"Override is set to true, but originPath and targetPath are both set to '/', this will override also service folders like .git, .github, .gitignore, etc. Consider using a different path for originPath or targetPath. This can broke the target repository causing the impossibility to push changes.")
				e.log.Info("Override is set to true, but originPath and targetPath are both set to '/', this will override also service folders like .git, .github, .gitignore, etc. Consider using a different path for originPath or targetPath. This can broke the target repository causing the impossibility to push changes.")
			}
		}

		ignorePath := cr.Spec.FromRepo.KrateoIgnorePath
		if err := loadIgnoreFileEventually(co, ignorePath); err != nil {
			e.log.Info("Unable to load '.krateoignore'", "msg", err.Error())
			e.rec.Eventf(cr, corev1.EventTypeWarning, "CannotLoadIgnoreFile",
				"Unable to load '.krateoignore' file: %s", err.Error())
		}

		if values != nil {
			createRenderFuncs(co, values)
		}

		if err := co.copyDir(fromPath, toPath); err != nil {
			return fmt.Errorf("unable to copy files: %w", err)
		}
	}

	e.log.Info("Origin and target repo synchronized",
		"fromUrl", spec.FromRepo.Url,
		"toUrl", spec.ToRepo.Url,
		"fromPath", fromPath,
		"toPath", toPath)
	e.rec.Eventf(cr, corev1.EventTypeNormal, "RepoSyncSuccess",
		"Origin and target repo synchronized")

	toRepoCommitId, err := toRepo.Commit(".", commitMessage, &git.IndexOptions{
		OriginRepo: fromRepo,
		FromPath:   fromPath,
		ToPath:     toPath,
	})
	if err == git.NoErrAlreadyUpToDate {
		toRepoCommitId, err := toRepo.GetLatestCommit(toRepo.CurrentBranch())
		if err != nil {
			return fmt.Errorf("unable to get latest commit from target repo: %w", err)
		}
		e.log.Info("Target repo not commited", "branch", toRepo.CurrentBranch(), "status", "repository already up-to-date")
		e.rec.Eventf(cr, corev1.EventTypeNormal, "RepoAlreadyUpToDate",
			fmt.Sprintf("Target repo already up-to-date on branch %s", toRepo.CurrentBranch()))

		meta.SetExternalName(cr, toRepoCommitId)
		cr.Status.OriginCommitId = fromRepoCommitId
		cr.Status.TargetCommitId = toRepoCommitId
		cr.Status.TargetBranch = toRepo.CurrentBranch()
		cr.Status.OriginBranch = fromRepo.CurrentBranch()

		err = e.kube.Status().Update(ctx, cr)
		if err != nil {
			return fmt.Errorf("unable to update status: %w", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("unable to commit target repo: %w", err)
	}
	e.log.Info("Target repo committed", "branch", toRepo.CurrentBranch(), "commitId", toRepoCommitId)
	e.rec.Eventf(cr, corev1.EventTypeNormal, "RepoCommitSuccess",
		fmt.Sprintf("Target repo committed on branch %s", toRepo.CurrentBranch()))

	err = toRepo.Push("origin", toRepo.CurrentBranch(), e.cfg.Insecure)
	if err != nil {
		return fmt.Errorf("unable to push target repo: %w", err)
	}
	e.log.Info("Target repo pushed", "branch", toRepo.CurrentBranch(), "commitId", toRepoCommitId)
	e.rec.Eventf(cr, corev1.EventTypeNormal, "RepoPushSuccess",
		fmt.Sprintf("Target repo pushed branch %s", toRepo.CurrentBranch()))

	meta.SetExternalName(cr, toRepoCommitId)
	cr.Status.OriginCommitId = fromRepoCommitId
	cr.Status.TargetCommitId = toRepoCommitId
	cr.Status.TargetBranch = toRepo.CurrentBranch()
	cr.Status.OriginBranch = fromRepo.CurrentBranch()
	err = e.kube.Status().Update(ctx, cr)
	if err != nil {
		return fmt.Errorf("unable to update status: %w", err)
	}
	return nil
}
