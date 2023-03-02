package repo

import (
	"context"
	"fmt"
	"strings"

	repov1alpha1 "github.com/krateoplatformops/git-provider/apis/repo/v1alpha1"
	"github.com/krateoplatformops/provider-runtime/pkg/helpers"
	"github.com/krateoplatformops/provider-runtime/pkg/reconciler"

	"github.com/krateoplatformops/provider-runtime/pkg/controller"
	"github.com/krateoplatformops/provider-runtime/pkg/event"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"github.com/krateoplatformops/provider-runtime/pkg/ratelimiter"
	"github.com/krateoplatformops/provider-runtime/pkg/resource"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/pkg/errors"
)

// Setup adds a controller that reconciles Token managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := reconciler.ControllerName(repov1alpha1.RepoGroupKind)

	log := o.Logger.WithValues("controller", name)

	recorder := mgr.GetEventRecorderFor(name)

	r := reconciler.NewReconciler(mgr,
		resource.ManagedKind(repov1alpha1.RepoGroupVersionKind),
		reconciler.WithExternalConnecter(&connector{
			kube:     mgr.GetClient(),
			log:      log,
			recorder: recorder,
		}),
		reconciler.WithLogger(log),
		reconciler.WithRecorder(event.NewAPIRecorder(recorder)))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&repov1alpha1.Repo{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube     client.Client
	log      logging.Logger
	recorder record.EventRecorder
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (reconciler.ExternalClient, error) {
	cr, ok := mg.(*repov1alpha1.Repo)
	if !ok {
		return nil, errors.New(errNotRepo)
	}

	cfg, err := loadExternalClientOpts(ctx, c.kube, cr)
	if err != nil {
		return nil, err
	}

	return &external{
		kube: c.kube,
		log:  c.log,
		cfg:  cfg,
		rec:  c.recorder,
	}, nil
}

type externalClientOpts struct {
	Insecure                bool
	UnsupportedCapabilities bool
	DeploymentServiceUrl    string
	FromRepoCreds           transport.AuthMethod
	ToRepoCreds             transport.AuthMethod
}

func loadExternalClientOpts(ctx context.Context, kc client.Client, cr *repov1alpha1.Repo) (*externalClientOpts, error) {
	if len(cr.Spec.DeploymentServiceUrl) == 0 {
		return nil, fmt.Errorf("deployment service url must be specified")
	}

	fromRepoCreds, err := getRepoCredentials(ctx, kc, cr.Spec.FromRepo)
	if err != nil {
		return nil, errors.Wrapf(err, "retrieving from repo credentials")
	}

	toRepoCreds, err := getRepoCredentials(ctx, kc, cr.Spec.ToRepo)
	if err != nil {
		return nil, errors.Wrapf(err, "retrieving to repo credentials")
	}

	return &externalClientOpts{
		Insecure:                helpers.Bool(cr.Spec.Insecure),
		UnsupportedCapabilities: helpers.Bool(cr.Spec.UnsupportedCapabilities),
		DeploymentServiceUrl:    cr.Spec.DeploymentServiceUrl,
		FromRepoCreds:           fromRepoCreds,
		ToRepoCreds:             toRepoCreds,
	}, nil
}

// getRepoCredentials returns the from repo credentials stored in a secret.
func getRepoCredentials(ctx context.Context, k client.Client, opts repov1alpha1.RepoOpts) (transport.AuthMethod, error) {
	if opts.SecretRef == nil {
		return nil, nil
	}

	token, err := resource.GetSecret(ctx, k, opts.SecretRef)
	if err != nil {
		return nil, err
	}

	authMethod := helpers.String(opts.AuthMethod)
	if strings.EqualFold(authMethod, "bearer") {
		return &http.TokenAuth{
			Token: token,
		}, nil
	}

	return &http.BasicAuth{
		Username: "krateoctl",
		Password: token,
	}, nil
}
