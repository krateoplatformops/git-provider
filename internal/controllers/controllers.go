package controllers

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/krateoplatformops/git-provider/internal/controllers/common/option"
	"github.com/krateoplatformops/git-provider/internal/controllers/localresource"
	"github.com/krateoplatformops/git-provider/internal/controllers/repo"
)

// Setup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o option.SetupOptions) error {
	for _, setup := range []func(ctrl.Manager, option.SetupOptions) error{
		localresource.Setup,
		repo.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
