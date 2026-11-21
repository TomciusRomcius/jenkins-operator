/*
Copyright 2026.

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
)

// JenkinsSpec defines the desired state of Jenkins.
type JenkinsSpec struct {
	// Image is the Jenkins controller container image, e.g. "jenkins/jenkins:lts".
	// +kubebuilder:validation:MinLength=1
	Image      string `json:"image"`
	UserSecret string `json:"usersecret"`
	// +kubebuilder:validation:Minimum=0
	ControllerExecutors int32 `json:"controllerexecutors"`
	// +optional
	Agents *JenkinsAgentOpts `json:"agents,omitempty"`
}

type JenkinsAgentOpts struct {
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`
	// +kubebuilder:validation:Minimum=1
	Executors int32 `json:"executors,omitempty"`
}

// JenkinsStatus defines the observed state of Jenkins.
type JenkinsStatus struct {
	// Conditions represent the latest available observations of the Jenkins instance's state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Jenkins is the Schema for the jenkins API.
type Jenkins struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JenkinsSpec   `json:"spec,omitempty"`
	Status JenkinsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// JenkinsList contains a list of Jenkins.
type JenkinsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Jenkins `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Jenkins{}, &JenkinsList{})
}
