package jenkins

import (
	"context"
	"crypto/sha256"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	civ1alpha1 "github.com/tomciusromcius/jenkins-operator/api/v1alpha1"
)

const (
	cascVolumeName = "casc"
	cascMountPath  = "/var/jenkins_config"
	cascFileName   = "jenkins.yaml"
	// cascHashAnnotation holds a hash of the JCasC config on the pod template, so a config change restarts the pod.
	cascHashAnnotation = "ci.tomciusromcius.github.io/casc-hash"
)

type JenkinsResourcesInterface interface {
	EnsureCascConfigMap(ctx context.Context, jenkins *civ1alpha1.Jenkins) error
	EnsureStatefulSet(ctx context.Context, jenkins *civ1alpha1.Jenkins) error
	EnsurePvc(ctx context.Context, jenkins *civ1alpha1.Jenkins) error
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
		"app.kubernetes.io/name": "jenkins",
	}

	config, err := cascConfig(jenkins)
	if err != nil {
		return err
	}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      r.FormatResource(jenkins, "statefulset"),
			Namespace: jenkins.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.client, statefulSet, func() error {
		statefulSet.Labels = labels
		statefulSet.Spec.Replicas = &replicas
		statefulSet.Spec.Selector = &v1.LabelSelector{MatchLabels: labels}
		statefulSet.Spec.Template.Labels = labels // must match Selector
		// Jenkins doesn't pick up JCasC changes on its own, so changing the hash restarts the pod.
		// Only our key is set so annotations added by others (e.g. kubectl rollout restart) survive.
		if statefulSet.Spec.Template.Annotations == nil {
			statefulSet.Spec.Template.Annotations = map[string]string{}
		}
		statefulSet.Spec.Template.Annotations[cascHashAnnotation] = fmt.Sprintf("%x", sha256.Sum256([]byte(config)))
		statefulSet.Spec.Template.Spec.Containers = []apiv1.Container{{
			Name:  "jenkins",
			Image: jenkins.Spec.Image,
			Command: []string{"/usr/bin/tini", "--",
				"bash", "-c",
				`set -e
				jenkins-plugin-cli --plugins configuration-as-code
				/usr/local/bin/jenkins.sh`}, // Better to move into a prebuilt image but idk
			Env: []apiv1.EnvVar{
				{Name: "CASC_JENKINS_CONFIG", Value: cascMountPath + "/" + cascFileName},
			},
			VolumeMounts: []apiv1.VolumeMount{
				{Name: cascVolumeName, MountPath: cascMountPath, ReadOnly: true},
			},
		}}
		statefulSet.Spec.Template.Spec.Volumes = []apiv1.Volume{
			{
				Name: "jenkins-home",
				VolumeSource: apiv1.VolumeSource{
					PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
						ClaimName: r.FormatResource(jenkins, "pvc"),
					},
				},
			},
			{
				Name: cascVolumeName,
				VolumeSource: apiv1.VolumeSource{
					ConfigMap: &apiv1.ConfigMapVolumeSource{
						LocalObjectReference: apiv1.LocalObjectReference{Name: r.FormatResource(jenkins, "casc")},
					},
				},
			},
		}

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

// EnsureCascConfigMap writes the Jenkins configuration-as-code (JCasC) config into a ConfigMap
// that EnsureStatefulSet mounts into the controller pod.
func (r *JenkinsResources) EnsureCascConfigMap(ctx context.Context, jenkins *civ1alpha1.Jenkins) error {
	config, err := cascConfig(jenkins)
	if err != nil {
		return err
	}

	cm := &apiv1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: jenkins.Namespace,
			Name:      r.FormatResource(jenkins, "casc"),
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.client, cm, func() error {
		cm.Data = map[string]string{cascFileName: config}
		return controllerutil.SetControllerReference(jenkins, cm, r.client.Scheme())
	})
	return err
}

// cascConfig renders the JCasC YAML for the Jenkins controller from the spec.
func cascConfig(jenkins *civ1alpha1.Jenkins) (string, error) {
	config := map[string]any{
		"jenkins": map[string]any{
			"numExecutors": jenkins.Spec.ControllerExecutors,
		},
	}
	out, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (r *JenkinsResources) EnsurePvc(ctx context.Context, jenkins *civ1alpha1.Jenkins) error {
	pvc := apiv1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Namespace: jenkins.Namespace,
			Name:      r.FormatResource(jenkins, "pvc"),
		},
	}

	if err := controllerutil.SetControllerReference(jenkins, &pvc, r.client.Scheme()); err != nil {
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, &pvc, func() error {
		pvc.Spec.Resources.Requests = apiv1.ResourceList{apiv1.ResourceStorage: resource.MustParse("10Gi")}
		pvc.Spec.AccessModes = []apiv1.PersistentVolumeAccessMode{
			apiv1.ReadWriteOnce,
		}
		return nil
	})

	if err != nil {
		return err
	}
	return nil
}

func (r *JenkinsResources) EnsureNodeport(ctx context.Context, jenkins *civ1alpha1.Jenkins) error { // TODO:gateway or ingress in the future
	np := apiv1.Service{
		ObjectMeta: v1.ObjectMeta{
			Namespace: jenkins.Namespace,
			Name:      r.FormatResource(jenkins, "service"),
		},
	}
	if err := controllerutil.SetControllerReference(jenkins, &np, r.client.Scheme()); err != nil {
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, &np, func() error {
		np.Spec.Type = apiv1.ServiceTypeNodePort
		np.Spec.Ports = []apiv1.ServicePort{
			{Name: "jenkins", Port: 8080, TargetPort: intstr.FromInt(8080)},
		}
		np.Spec.Selector = map[string]string{
			"app.kubernetes.io/name": "jenkins",
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (r *JenkinsResources) FormatResource(jenkins *civ1alpha1.Jenkins, resourceType string) string {
	return fmt.Sprintf("%s-%s", jenkins.Name, resourceType)
}
