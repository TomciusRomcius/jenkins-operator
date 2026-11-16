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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	civ1alpha1 "github.com/tomciusromcius/jenkins-operator/api/v1alpha1"
	"github.com/tomciusromcius/jenkins-operator/internal/controller/jenkins"
)

// JenkinsReconciler reconciles a Jenkins object
type JenkinsReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	JenkinsResources jenkins.JenkinsResourcesInterface
}

// +kubebuilder:rbac:groups=ci.tomciusromcius.github.io,resources=jenkins,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ci.tomciusromcius.github.io,resources=jenkins/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ci.tomciusromcius.github.io,resources=jenkins/finalizers,verbs=update

func (r *JenkinsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)
	objectKey := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}
	jenkins, err := jenkins.FetchJenkins(ctx, r.Client, objectKey)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.JenkinsResources.EnsureStatefulSet(ctx, jenkins); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.JenkinsResources.EnsurePvc(ctx, jenkins); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.JenkinsResources.EnsureNodeport(ctx, jenkins); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JenkinsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&civ1alpha1.Jenkins{}).
		Named("jenkins").
		Complete(r)
}
