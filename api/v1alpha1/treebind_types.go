/*
Copyright The Treebind Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SyncDirection says which way changes flow between the repository and the
// volume.
// +kubebuilder:validation:Enum=pull;bidirectional
type SyncDirection string

const (
	// SyncDirectionPull mirrors the repository into the volume. Local edits are
	// never committed or pushed.
	SyncDirectionPull SyncDirection = "pull"
	// SyncDirectionBidirectional also commits the application's edits and
	// pushes them to the repository.
	SyncDirectionBidirectional SyncDirection = "bidirectional"
)

// ConflictPolicy says which side wins when the repository and the volume have
// both changed the same file.
// +kubebuilder:validation:Enum=followRemote;followLocal
type ConflictPolicy string

const (
	// ConflictPolicyFollowRemote keeps the repository's version.
	ConflictPolicyFollowRemote ConflictPolicy = "followRemote"
	// ConflictPolicyFollowLocal keeps the version on the volume.
	ConflictPolicyFollowLocal ConflictPolicy = "followLocal"
)

// KeySelector names a key in a Secret or ConfigMap. The object is always in
// the same namespace as the Treebind, so a Treebind cannot read another
// namespace's data.
type KeySelector struct {
	// name is the name of the Secret or ConfigMap.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// key is the key inside the Secret or ConfigMap that holds the value.
	// +required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// AuthSpec holds the credentials the agent uses to reach the repository. The
// Secret and ConfigMap are mounted into the agent pod, so the operator never
// reads their contents.
type AuthSpec struct {
	// sshKeySecretRef points to the Secret key that holds the private SSH key.
	// +optional
	SSHKeySecretRef *KeySelector `json:"sshKeySecretRef,omitempty"`

	// knownHostsConfigMapRef points to the ConfigMap key that holds the
	// known_hosts entries used to verify the Git host.
	// +optional
	KnownHostsConfigMapRef *KeySelector `json:"knownHostsConfigMapRef,omitempty"`
}

// RepositorySpec describes the Git repository to sync with.
type RepositorySpec struct {
	// url is the address of the repository, for example
	// ssh://git@github.com/acme/dashboards.git.
	// +required
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// branch is the branch to follow. The repository's default branch is used
	// when it is empty.
	// +optional
	Branch string `json:"branch,omitempty"`

	// path is a directory inside the repository. Only that directory is synced
	// with the volume. The whole repository is used when it is empty.
	// +optional
	Path string `json:"path,omitempty"`

	// auth names the credentials for the repository.
	// +optional
	Auth *AuthSpec `json:"auth,omitempty"`
}

// TargetSpec describes the volume the files live on.
type TargetSpec struct {
	// claimName is the name of an existing PersistentVolumeClaim in the same
	// namespace as the Treebind.
	// +required
	// +kubebuilder:validation:MinLength=1
	ClaimName string `json:"claimName"`

	// subPath is a directory inside the claim. Only that directory is synced
	// with the repository. The whole claim is used when it is empty.
	// +optional
	SubPath string `json:"subPath,omitempty"`

	// mountPath is where the claim is mounted inside the sync container. It
	// does not affect where the application mounts the claim.
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

// SyncSpec controls when and in which direction files are synced.
type SyncSpec struct {
	// direction is pull for a read-only mirror, or bidirectional to also commit
	// and push the application's edits.
	// +required
	Direction SyncDirection `json:"direction"`

	// interval is how often the agent polls the repository for new commits.
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// settleTime is the quiet period after the last local edit before the agent
	// commits, so a file that is still being written is not committed half done.
	// +optional
	SettleTime *metav1.Duration `json:"settleTime,omitempty"`

	// conflictPolicy says which side wins when both changed the same file.
	// +optional
	ConflictPolicy ConflictPolicy `json:"conflictPolicy,omitempty"`
}

// CommitSpec controls the commits the agent creates from local edits.
type CommitSpec struct {
	// authorName is the name recorded as the author of every commit.
	// +optional
	AuthorName string `json:"authorName,omitempty"`

	// authorEmail is the email recorded as the author of every commit.
	// +optional
	AuthorEmail string `json:"authorEmail,omitempty"`

	// messageTemplate is the commit message, as a Go text/template.
	// +optional
	MessageTemplate string `json:"messageTemplate,omitempty"`
}

// PathsSpec limits which files are synced, using glob patterns relative to the
// synced directory.
type PathsSpec struct {
	// include lists the patterns of files to sync. Everything is included when
	// the list is empty.
	// +optional
	// +listType=atomic
	Include []string `json:"include,omitempty"`

	// exclude lists the patterns of files to leave out, for example temporary
	// and swap files.
	// +optional
	// +listType=atomic
	Exclude []string `json:"exclude,omitempty"`
}

// TreebindSpec defines the desired state of Treebind.
type TreebindSpec struct {
	// repository is the Git repository to sync with.
	// +required
	Repository RepositorySpec `json:"repository"`

	// target is the volume the files live on.
	// +required
	Target TargetSpec `json:"target"`

	// sync controls the direction and timing of the sync.
	// +required
	Sync SyncSpec `json:"sync"`

	// commit controls the commits created from local edits.
	// +optional
	Commit *CommitSpec `json:"commit,omitempty"`

	// paths limits which files are synced.
	// +optional
	Paths *PathsSpec `json:"paths,omitempty"`
}

// TreebindStatus defines the observed state of Treebind.
type TreebindStatus struct {
	// observedGeneration is the generation of the spec that the status
	// describes. A status older than metadata.generation is not up to date.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// lastSyncedCommit is the commit the volume and the repository last agreed
	// on.
	// +optional
	LastSyncedCommit string `json:"lastSyncedCommit,omitempty"`

	// lastSyncTime is when the last sync finished.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// conditions represent the current state of the Treebind resource. Each
	// condition has a unique type and is set by the reconciler. The status of
	// each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Treebind is the Schema for the treebinds API
type Treebind struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Treebind
	// +required
	Spec TreebindSpec `json:"spec"`

	// status defines the observed state of Treebind
	// +optional
	Status TreebindStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// TreebindList contains a list of Treebind
type TreebindList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Treebind `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Treebind{}, &TreebindList{})
		return nil
	})
}
