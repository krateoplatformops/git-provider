package v1alpha1

import (
	commonv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RepoOpts struct {
	// Url: the repository URL.
	// +immutable
	Url string `json:"url"`

	// Path: name of the folder in the git repository
	// to copy from (or to).
	// +optional
	Path *string `json:"path,omitempty"`

	// Branch: in the git repository to copy from (or to).
	// +optional
	Branch *string `json:"branch,omitempty"`

	// SecretRef: holds credentials required to git server authentication.
	SecretRef *commonv1.SecretKeySelector `json:"secretRef"`

	// AuthMethod defines the authentication mode. One of 'basic' or 'bearer'
	// +optional
	AuthMethod *string `json:"authMethod,omitempty"`
}

// A RepoSpec defines the desired state of a Repo.
type RepoSpec struct {
	commonv1.ManagedSpec `json:",inline"`

	// FromRepo: repo origin to copy from
	// +immutable
	FromRepo RepoOpts `json:"fromRepo"`

	// ToRepo: repo destination to copy to
	// +immutable
	ToRepo RepoOpts `json:"toRepo"`

	// ConfigMapKeyRef: holds template values
	// +optional
	ConfigMapKeyRef *commonv1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`

	// Insecure is useful with hand made SSL certs (default: false)
	// +optional
	Insecure *bool `json:"insecure,omitempty"`

	// UnsupportedCapabilities enable Go-Git transport.UnsupportedCapabilities (default: false)
	// Azure DevOps requires capabilities multi_ack / multi_ack_detailed,
	// which are not fully implemented in go-git library and by default
	// are included in transport.UnsupportedCapabilities.
	// +optional
	UnsupportedCapabilities *bool `json:"unsupportedCapabilities,omitempty"`
}

// A RepoStatus represents the observed state of a Repo.
type RepoStatus struct {
	commonv1.ManagedStatus `json:",inline"`

	// OriginCommitId: last commit identifier of the origin repo
	OriginCommitId *string `json:"originCommitId,omitempty"`

	// TargetCommitId: last commit identifier of the target repo
	TargetCommitId *string `json:"targetCommitId,omitempty"`

	// Branch: branch where commit was done
	Branch *string `json:"branch,omitempty"`
}

// +kubebuilder:object:root=true

// A Repo is a managed resource that represents a Krateo Git Repository
// +kubebuilder:printcolumn:name="TARGET_COMMIT_ID",type="string",JSONPath=".status.targetCommitId"
// +kubebuilder:printcolumn:name="TARGET_BRANCH",type="string",JSONPath=".status.branch"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status",priority=10
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={git,krateo}
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
