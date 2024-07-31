package v0alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DataPlaneService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataPlaneServiceSpec   `json:"spec,omitempty"`
	Status DataPlaneServiceStatus `json:"status,omitempty"`
}

type DataPlaneServiceSpec struct {
	Group   string `json:"group"`
	Version string `json:"version"`

	// Services is a list of services that the proxied service provides.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Services []Service `json:"services" patchStrategy:"merge" patchMergeKey:"type"`
}

// Service defines the type of service the proxied service provides.
//
// Currently this only includes the service type, but it can be extended in the future to include
// additional configuration options if necessary.
type Service struct {
	Type ServiceType `json:"type"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DataPlaneServiceList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []DataPlaneService `json:"items,omitempty"`
}

// ConnectionType defines the type of connection to the proxied service.
type ConnectionType string

const (
	GRPC ConnectionType = "grpc"
)

// ServiceType defines the type of services the proxied service provides.
// +enum
type ServiceType string

// ServiceType values
const (
	AdmissionControlServiceType ServiceType = "admission"
	DiagnosticsServiceType      ServiceType = "diagnostics"
	DataServiceType             ServiceType = "data"
	ResourceServiceType         ServiceType = "resource"
	StreamServiceType           ServiceType = "stream"
)

// ConditionStatus indicates the status of a condition (true, false, or unknown).
type ConditionStatus string

// These are valid condition statuses. "ConditionTrue" means a resource is in the condition;
// "ConditionFalse" means a resource is not in the condition; "ConditionUnknown" means kubernetes
// can't decide if a resource is in the condition or not. In the future, we could add other
// intermediate conditions, e.g. ConditionDegraded.
const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

// DataPlaneServiceConditionType is a valid value for DataPlaneServiceCondition.Type
type DataPlaneServiceConditionType string

const (
	// Available indicates that the service exists and is reachable
	Available DataPlaneServiceConditionType = "Available"
)

// DataPlaneServiceCondition describes the state of an DataPlaneService at a particular point
type DataPlaneServiceCondition struct {
	// Type is the type of the condition.
	Type DataPlaneServiceConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// DataPlaneServiceStatus contains derived information about a remote DataPlaneService.
type DataPlaneServiceStatus struct {
	// Current service state of DataPlaneService.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []DataPlaneServiceCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}
