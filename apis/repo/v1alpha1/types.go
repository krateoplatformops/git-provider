package v1alpha1

import (
	commonv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	prv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	"github.com/krateoplatformops/provider-runtime/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RepoOpts struct {
	// Url: url of the remote repository
	// +immutable
	Url string `json:"url"`

	// Path: if in spec.fromRepo, Represents the folder to clone from. If not set the entire repository is cloned. If in spec.toRepo, represents the folder to use as destination.
	// +kubebuilder:default:="/"
	// +optional
	Path *string `json:"path,omitempty"`

	// Branch: if in spec.fromRepo, the branch to copy from. If in spec.toRepo, represents the branch to populate; If the branch does not exist on remote is created by the provider.
	// +required
	Branch *string `json:"branch"`

	// SecretRef: reference to a secret that contains token required to git server authentication or cookie file in case of 'cookiefile' authMethod.
	SecretRef *commonv1.SecretKeySelector `json:"secretRef"`

	// UsernameRef: holds username required to git server authentication. - If 'authMethod' is 'bearer' or 'cookiefile' the field is ignored. If the field is not set, username is setted as 'krateoctl'
	// +optional
	UsernameRef *commonv1.SecretKeySelector `json:"usernameRef"`

	// AuthMethod: Possible values are: `generic`, `bearer`, `gitcookies`. `generic` requires  `secretRef` and `usernameRef`; `generic` requires only `secretRef`; `cookiefile` requires only `secretRef`
	// In case of 'cookiefile' the secretRef must contain a file with the cookie.
	// +kubebuilder:validation:Enum=generic;bearer;cookiefile
	// +kubebuilder:default:=generic
	// +optional
	AuthMethod *string `json:"authMethod,omitempty"`

	/*
		CloneFromBranch: used the parent of the new branch.
		- If the branch exists, the parameter is ignored.
		- If the parameter is not set, the branch is created empty and has no parents (no history) - `git switch --orphan branch-name`
	*/
	// +optional
	CloneFromBranch *string `json:"cloneFromBranch,omitempty"`
}

// A RepoSpec defines the desired state of a Repo.
type RepoSpec struct {
	// FromRepo: repo origin to copy from
	// +immutable
	FromRepo RepoOpts `json:"fromRepo"`

	// ToRepo: repo destination to copy to
	// +immutable
	ToRepo RepoOpts `json:"toRepo"`

	// ConfigMapKeyRef: holds template values
	// +optional
	ConfigMapKeyRef *commonv1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`

	// Insecure: Insecure is useful with hand made SSL certs (default: false)
	// +optional
	Insecure *bool `json:"insecure,omitempty"`

	// UnsupportedCapabilities: If `true` [capabilities not supported by any client implementation](https://github.com/go-git/go-git/blob/4fd9979d5c2940e72bdd6946fec21e02d959f0f6/plumbing/transport/common.go#L310) will not be used by the provider
	// +optional
	// +kubebuilder:default:=false
	UnsupportedCapabilities *bool `json:"unsupportedCapabilities,omitempty"`

	// EnableUpdate: If `true`, the provider performs updates on the repository specified in `toRepo` when newer commits are retrieved from `fromRepo`
	// +kubebuilder:default:=false
	// +optional
	EnableUpdate *bool `json:"enableUpdate,omitempty"`
}

// A RepoStatus represents the observed state of a Repo.
type RepoStatus struct {
	commonv1.ConditionedStatus `json:",inline"`

	// OriginCommitId: last commit identifier of the origin repo
	OriginCommitId *string `json:"originCommitId,omitempty"`

	// TargetCommitId: last commit identifier of the target repo
	TargetCommitId *string `json:"targetCommitId,omitempty"`

	// TargetBranch: branch where commit was done
	TargetBranch *string `json:"targetBranch,omitempty"`

	// OriginBranch: branch where commit was done
	OriginBranch *string `json:"originBranch,omitempty"`
}

// +kubebuilder:object:root=true

// A Repo is a managed resource that represents a Krateo Git Repository
// +kubebuilder:printcolumn:name="ORIGIN_COMMIT_ID",type="string",JSONPath=".status.originCommitId"
// +kubebuilder:printcolumn:name="ORIGIN_BRANCH",type="string",JSONPath=".status.originBranch"
// +kubebuilder:printcolumn:name="TARGET_COMMIT_ID",type="string",JSONPath=".status.targetCommitId"
// +kubebuilder:printcolumn:name="TARGET_BRANCH",type="string",JSONPath=".status.targetBranch"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={git,krateo}
type Repo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepoSpec   `json:"spec"`
	Status RepoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RepoList contains a list of Repo.
type RepoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Repo `json:"items"`
}

// GetCondition of this Repo.
func (mg *Repo) GetCondition(ct prv1.ConditionType) prv1.Condition {
	return mg.Status.GetCondition(ct)
}

// SetConditions of this Repo.
func (mg *Repo) SetConditions(c ...prv1.Condition) {
	mg.Status.SetConditions(c...)
}

// GetItems of this RepoList.
func (l *RepoList) GetItems() []resource.Managed {
	items := make([]resource.Managed, len(l.Items))
	for i := range l.Items {
		items[i] = &l.Items[i]
	}
	return items
}
