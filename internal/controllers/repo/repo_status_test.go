package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/krateoplatformops/git-provider/apis"
	repov1alpha1 "github.com/krateoplatformops/git-provider/apis/repo/v1alpha1"
	commonv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFailSyncMarksResourceUnavailableAndSyncedFalse(t *testing.T) {
	require.NoError(t, apis.AddToScheme(clientsetscheme.Scheme))

	cr := &repov1alpha1.Repo{
		ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: "test-system"},
	}
	cr.Status.SetConditions(commonv1.Available())

	kubeClient := fake.NewClientBuilder().
		WithScheme(clientsetscheme.Scheme).
		WithStatusSubresource(cr).
		WithObjects(cr).
		Build()

	e := &external{kube: kubeClient}

	inputErr := errors.New("push failed")
	returnedErr := e.failSync(context.Background(), cr, inputErr)

	require.ErrorIs(t, returnedErr, inputErr)

	updated := &repov1alpha1.Repo{}
	require.NoError(t, kubeClient.Get(context.Background(), ctrlclient.ObjectKey{Name: "sample", Namespace: "test-system"}, updated))

	ready := updated.GetCondition(commonv1.TypeReady)
	synced := updated.GetCondition(commonv1.TypeSynced)

	require.Equal(t, metav1.ConditionFalse, ready.Status)
	require.Equal(t, commonv1.ReasonUnavailable, ready.Reason)
	require.Equal(t, metav1.ConditionFalse, synced.Status)
	require.Equal(t, commonv1.ReasonReconcileError, synced.Reason)
	require.Equal(t, "push failed", synced.Message)
}

// errorStatusWriter simula un fallimento durante la scrittura dello status
type errorStatusWriter struct {
	ctrlclient.StatusWriter
}

func (w *errorStatusWriter) Update(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.SubResourceUpdateOption) error {
	return errors.New("simulated update conflict error")
}

// errorClient fa da wrapper a un client mockato e restituisce lo StatusWriter fallato
type errorClient struct {
	ctrlclient.Client
}

func (c *errorClient) Status() ctrlclient.StatusWriter {
	return &errorStatusWriter{c.Client.Status()}
}

func TestFailSyncWithUpdateConflictCausesNestedError(t *testing.T) {
	require.NoError(t, apis.AddToScheme(clientsetscheme.Scheme))

	cr := &repov1alpha1.Repo{
		ObjectMeta: metav1.ObjectMeta{Name: "sample-conflict", Namespace: "test-system"},
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(clientsetscheme.Scheme).
		WithStatusSubresource(cr).
		WithObjects(cr).
		Build()

	// Inseriamo il client malevolo
	e := &external{kube: &errorClient{Client: kubeClient}}

	inputErr := errors.New("primary reconcile failure")

	// Eseguiamo failSync.
	returnedErr := e.failSync(context.Background(), cr, inputErr)

	// Verifichiamo che il fallimento originale venga restituito intatto,
	// senza errori di update annidati.
	require.Equal(t, inputErr, returnedErr)
	require.NotContains(t, returnedErr.Error(), "simulated update conflict error")
}
