package aggregation

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DataPlaneService struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec   DataPlaneServiceSpec
	Status DataPlaneServiceStatus
}

type DataPlaneServiceSpec struct {
	Group   string
	Version string

	// Services is a list of services that the proxied service provides.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Services []Service
}

// Service defines the type of service the proxied service provides.
//
// Currently this only includes the service type, but it can be extended in the future to include
// additional configuration options if necessary.
type Service struct {
	Type ServiceType
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DataPlaneServiceList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta

	Items []DataPlaneService
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
	Type DataPlaneServiceConditionType
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status ConditionStatus
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time
	// Unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason string
	// Human-readable message indicating details about last transition.
	// +optional
	Message string
}

// DataPlaneServiceStatus contains derived information about a remote DataPlaneService.
type DataPlaneServiceStatus struct {
	// Current service state of DataPlaneService.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []DataPlaneServiceCondition
}
