package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZenLockSpec defines the desired state of ZenLock
type ZenLockSpec struct {
	// EncryptedData is a map of key -> Base64-encoded ciphertext
	// +kubebuilder:validation:Required
	EncryptedData map[string]string `json:"encryptedData"`

	// Algorithm specifies the encryption method (default: "age")
	// +kubebuilder:default="age"
	// +kubebuilder:validation:Enum=age
	Algorithm string `json:"algorithm,omitempty"`

	// AllowedSubjects is an optional list of ServiceAccounts allowed to use this secret
	// +optional
	AllowedSubjects []SubjectReference `json:"allowedSubjects,omitempty"`
}

// SubjectReference references a Kubernetes subject (ServiceAccount, User, Group)
type SubjectReference struct {
	// Kind is the kind of subject (ServiceAccount, User, Group)
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name is the name of the subject
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the subject (required for ServiceAccount)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ZenLockStatus defines the observed state of ZenLock
type ZenLockStatus struct {
	// Phase represents the current phase of the ZenLock
	// +kubebuilder:validation:Enum=Ready;Error
	Phase string `json:"phase,omitempty"`

	// LastRotation is the timestamp of the last key rotation
	// +optional
	LastRotation *metav1.Time `json:"lastRotation,omitempty"`

	// Conditions represent the latest available observations of the ZenLock's state
	// +optional
	Conditions []ZenLockCondition `json:"conditions,omitempty"`
}

// ZenLockCondition describes the state of a ZenLock at a certain point
type ZenLockCondition struct {
	// Type of condition
	Type string `json:"type"`

	// Status of the condition (True, False, Unknown)
	Status string `json:"status"`

	// Reason for the condition's last transition
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable explanation of the condition
	// +optional
	Message string `json:"message,omitempty"`

	// LastTransitionTime is the last time the condition transitioned
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=zenlocks,shortName=zl;zenlock
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +genclient

// ZenLock is the Schema for the zenlocks API
type ZenLock struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZenLockSpec   `json:"spec,omitempty"`
	Status ZenLockStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZenLockList contains a list of ZenLock
type ZenLockList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZenLock `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZenLock{}, &ZenLockList{})
}

