package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Desired",type="string",JSONPath=`.spec.oss`
// +kubebuilder:printcolumn:name="Current",type="string",JSONPath=`.status.oss`

// PodSet is the Schema for the podsets API
type PodSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PodSetSpec   `json:"spec,omitempty"`
	Status            PodSetStatus `json:"status,omitempty"`
}

// PodSetSpec defines the desired state of PodSet
type PodSetSpec struct {
	Oss    int32 `json:"oss"`
	Low    int32 `json:"low"`
	High   int32 `json:"high"`
	Period int32 `json:"period"`
}

// +k8s:openapi-gen=true

// PodSetStatus defines the observed state of PodSet
type PodSetStatus struct {
	Oss      int32    `json:"oss"`
	Low      int32    `json:"low"`
	High     int32    `json:"high"`
	Period   int32    `json:"period"`
	PodNames []string `json:"podNames"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodSetList contains a list of PodSet
type PodSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodSet{}, &PodSetList{})
}
