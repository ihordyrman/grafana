/*
Copyright 2016 The Kubernetes Authors.

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

package helper

import (
	"strings"

	v0alpha1 "github.com/grafana/grafana/pkg/aggregator/apis/aggregation/v0alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DataPlaneServiceNameToGroupVersion returns the GroupVersion for a given dataplaneServiceNam.  The name
// must be valid, but any object you get back from an informer will be valid.
func DataPlaneServiceNameToGroupVersion(apiServiceName string) schema.GroupVersion {
	tokens := strings.SplitN(apiServiceName, ".", 2)
	return schema.GroupVersion{Group: tokens[1], Version: tokens[0]}
}

// SetDataPlaneServiceCondition sets the status condition.  It either overwrites the existing one or
// creates a new one
func SetDataPlaneServiceCondition(apiService *v0alpha1.DataPlaneService, newCondition v0alpha1.DataPlaneServiceCondition) {
	existingCondition := GetDataPlaneServiceConditionByType(apiService, newCondition.Type)
	if existingCondition == nil {
		apiService.Status.Conditions = append(apiService.Status.Conditions, newCondition)
		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		existingCondition.LastTransitionTime = newCondition.LastTransitionTime
	}

	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
}

// IsDataPlaneServiceConditionTrue indicates if the condition is present and strictly true
func IsDataPlaneServiceConditionTrue(apiService *v0alpha1.DataPlaneService, conditionType v0alpha1.DataPlaneServiceConditionType) bool {
	condition := GetDataPlaneServiceConditionByType(apiService, conditionType)
	return condition != nil && condition.Status == v0alpha1.ConditionTrue
}

// GetDataPlaneServiceConditionByType gets an *DataPlaneServiceCondition by DataPlaneServiceConditionType if present
func GetDataPlaneServiceConditionByType(apiService *v0alpha1.DataPlaneService, conditionType v0alpha1.DataPlaneServiceConditionType) *v0alpha1.DataPlaneServiceCondition {
	for i := range apiService.Status.Conditions {
		if apiService.Status.Conditions[i].Type == conditionType {
			return &apiService.Status.Conditions[i]
		}
	}
	return nil
}
