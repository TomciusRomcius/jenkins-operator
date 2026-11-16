package jenkins

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	civ1alpha1 "github.com/tomciusromcius/jenkins-operator/api/v1alpha1"
)

type JenkinsResourcesInterface interface {
	EnsureStatefulSet(ctx context.Context, jenkins *civ1alpha1.Jenkins) error
	EnsurePvc(ctx context.Context, jenkins *civ1alpha1.Jenkins) error
	EnsurePv(ctx context.Context, jenkins *civ1alpha1.Jenkins) error
	EnsureNodeport(ctx context.Context, jenkins *civ1alpha1.Jenkins) error // TODO:gateway or ingress in the future
}

// Compile-time check that JenkinsController implements JenkinsControllerInterface.
var _ JenkinsResourcesInterface = &JenkinsResources{}

type JenkinsResources struct {
	client client.Client
}

// NewJenkinsControllerResourceCreator returns a resource creator that uses the given client.
func NewJenkinsControllerResourceCreator(c client.Client) *JenkinsResources {
	return &JenkinsResources{client: c}
}

func (r *JenkinsResources) EnsureStatefulSet(ctx context.Context, jenkins *civ1alpha1.Jenkins) error {
	replicas := int32(1)
	labels := map[string]string{
		"app": fmt.Sprintf("%s-%s", jenkins.Name, "statefulset"),
	}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", jenkins.Name, "statefulset"),
			Namespace: jenkins.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.client, statefulSet, func() error {
		statefulSet.Labels = labels
		statefulSet.Spec.Replicas = &replicas
		statefulSet.Spec.Selector = &v1.LabelSelector{MatchLabels: labels}
		statefulSet.Spec.Template.Labels = labels // must match Selector
		statefulSet.Spec.Template.Spec.Containers = []apiv1.Container{{
			Name:  "jenkins",
			Image: jenkins.Spec.Image,
		}}

		return controllerutil.SetControllerReference(jenkins, statefulSet, r.client.Scheme())
	})

	if err != nil {
		return err
	}

	if result != controllerutil.OperationResultNone {
		logf.FromContext(ctx).Info("StatefulSet reconciled", "name", statefulSet.Name, "operation", result)
	}
	return nil
}

func (r *JenkinsResources) EnsurePvc(ctx context.Context, jenkins *civ1alpha1.Jenkins) error {
	return nil
}

func (r *JenkinsResources) EnsurePv(ctx context.Context, jenkins *civ1alpha1.Jenkins) error {
	return nil
}

func (r *JenkinsResources) EnsureNodeport(ctx context.Context, jenkins *civ1alpha1.Jenkins) error { // TODO:gateway or ingress in the future
	return nil
}
