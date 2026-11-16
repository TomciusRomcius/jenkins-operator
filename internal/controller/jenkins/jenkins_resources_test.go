package jenkins

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	civ1alpha1 "github.com/tomciusromcius/jenkins-operator/api/v1alpha1"
)

var errInjected = errors.New("injected API error")

var failCreate = &interceptor.Funcs{
	Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
		return errInjected
	},
}

// expectControlledByJenkins checks the object has exactly one controller owner reference pointing at jenkins.
func expectControlledByJenkins(obj metav1.Object, jenkins *civ1alpha1.Jenkins) {
	GinkgoHelper()
	Expect(obj.GetOwnerReferences()).To(HaveLen(1))
	ref := obj.GetOwnerReferences()[0]
	Expect(ref.Kind).To(Equal("Jenkins"))
	Expect(ref.APIVersion).To(Equal(civ1alpha1.GroupVersion.String()))
	Expect(ref.Name).To(Equal(jenkins.Name))
	Expect(ref.UID).To(Equal(jenkins.UID))
	Expect(ref.Controller).To(HaveValue(BeTrue()))
}

var _ = Describe("JenkinsResources", func() {
	var (
		ctx     context.Context
		c       client.Client
		r       *JenkinsResources
		jenkins *civ1alpha1.Jenkins
	)

	BeforeEach(func() {
		ctx = context.Background()
		c = newFakeClient(nil)
		r = NewJenkinsControllerResourceCreator(c)
		jenkins = newJenkins()
	})

	Describe("FormatResource", func() {
		It("joins the jenkins name and resource type", func() {
			Expect(r.FormatResource(jenkins, "pvc")).To(Equal("my-jenkins-pvc"))
		})
	})

	Describe("EnsureStatefulSet", func() {
		getStatefulSet := func() *appsv1.StatefulSet {
			GinkgoHelper()
			sts := &appsv1.StatefulSet{}
			Expect(c.Get(ctx, key("my-jenkins-statefulset"), sts)).To(Succeed())
			return sts
		}

		It("creates the statefulset", func() {
			Expect(r.EnsureStatefulSet(ctx, jenkins)).To(Succeed())

			sts := getStatefulSet()
			Expect(sts.Spec.Replicas).To(HaveValue(Equal(int32(1))))
			Expect(sts.Spec.Selector).NotTo(BeNil())
			Expect(sts.Spec.Selector.MatchLabels).To(Equal(sts.Spec.Template.Labels),
				"selector must match pod template labels or the API server rejects the StatefulSet")

			containers := sts.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			Expect(containers[0].Name).To(Equal("jenkins"))
			Expect(containers[0].Image).To(Equal(testImage))

			expectControlledByJenkins(sts, jenkins)
		})

		It("mounts the PVC created by EnsurePvc", func() {
			Expect(r.EnsureStatefulSet(ctx, jenkins)).To(Succeed())
			Expect(r.EnsurePvc(ctx, jenkins)).To(Succeed())

			pvcs := &apiv1.PersistentVolumeClaimList{}
			Expect(c.List(ctx, pvcs, client.InNamespace(testNamespace))).To(Succeed())
			Expect(pvcs.Items).To(HaveLen(1))

			volumes := getStatefulSet().Spec.Template.Spec.Volumes
			Expect(volumes).To(HaveLen(1))
			Expect(volumes[0].PersistentVolumeClaim).NotTo(BeNil())
			Expect(volumes[0].PersistentVolumeClaim.ClaimName).To(Equal(pvcs.Items[0].Name))
		})

		It("updates the image when the spec changes", func() {
			Expect(r.EnsureStatefulSet(ctx, jenkins)).To(Succeed())

			jenkins.Spec.Image = "jenkins/jenkins:2.500"
			Expect(r.EnsureStatefulSet(ctx, jenkins)).To(Succeed())

			Expect(getStatefulSet().Spec.Template.Spec.Containers[0].Image).To(Equal("jenkins/jenkins:2.500"))
		})

		It("is idempotent", func() {
			Expect(r.EnsureStatefulSet(ctx, jenkins)).To(Succeed())
			before := getStatefulSet().ResourceVersion

			Expect(r.EnsureStatefulSet(ctx, jenkins)).To(Succeed())

			Expect(getStatefulSet().ResourceVersion).To(Equal(before), "second call should not update")
		})

		It("returns API errors", func() {
			r = NewJenkinsControllerResourceCreator(newFakeClient(failCreate))

			Expect(r.EnsureStatefulSet(ctx, jenkins)).To(MatchError(errInjected))
		})
	})

	Describe("EnsurePvc", func() {
		getPvc := func() *apiv1.PersistentVolumeClaim {
			GinkgoHelper()
			pvc := &apiv1.PersistentVolumeClaim{}
			Expect(c.Get(ctx, key("my-jenkins-pvc"), pvc)).To(Succeed())
			return pvc
		}

		It("creates the pvc", func() {
			Expect(r.EnsurePvc(ctx, jenkins)).To(Succeed())

			pvc := getPvc()
			Expect(pvc.Spec.AccessModes).To(ConsistOf(apiv1.ReadWriteOnce))
			Expect(pvc.Spec.Resources.Requests).To(HaveKeyWithValue(apiv1.ResourceStorage, resource.MustParse("10Gi")))
			Expect(pvc.Spec.VolumeName).To(BeEmpty(), "pvc must not pre-bind to a PV so the provisioner can create one")
			Expect(pvc.Spec.StorageClassName).To(BeNil(), "nil storage class means the cluster default is used")

			expectControlledByJenkins(pvc, jenkins)
		})

		It("is idempotent", func() {
			Expect(r.EnsurePvc(ctx, jenkins)).To(Succeed())
			before := getPvc().ResourceVersion

			Expect(r.EnsurePvc(ctx, jenkins)).To(Succeed())

			Expect(getPvc().ResourceVersion).To(Equal(before), "second call should not update")
		})

		It("returns API errors", func() {
			r = NewJenkinsControllerResourceCreator(newFakeClient(failCreate))

			Expect(r.EnsurePvc(ctx, jenkins)).To(MatchError(errInjected))
		})
	})

	Describe("EnsureNodeport", func() {
		getService := func() *apiv1.Service {
			GinkgoHelper()
			svc := &apiv1.Service{}
			Expect(c.Get(ctx, key("my-jenkins-service"), svc)).To(Succeed())
			return svc
		}

		It("creates the nodeport service", func() {
			Expect(r.EnsureNodeport(ctx, jenkins)).To(Succeed())

			svc := getService()
			Expect(svc.Spec.Type).To(Equal(apiv1.ServiceTypeNodePort))
			Expect(svc.Spec.Ports).To(HaveLen(1))
			Expect(svc.Spec.Ports[0].Name).To(Equal("jenkins"))
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(8080)))
			Expect(svc.Spec.Ports[0].TargetPort).To(Equal(intstr.FromInt(8080)))

			expectControlledByJenkins(svc, jenkins)
		})

		It("selects the jenkins pods", func() {
			Expect(r.EnsureStatefulSet(ctx, jenkins)).To(Succeed())
			Expect(r.EnsureNodeport(ctx, jenkins)).To(Succeed())

			sts := &appsv1.StatefulSet{}
			Expect(c.Get(ctx, key("my-jenkins-statefulset"), sts)).To(Succeed())

			selector := getService().Spec.Selector
			Expect(selector).NotTo(BeEmpty(), "a service without a selector routes to nothing")
			for k, v := range selector {
				Expect(sts.Spec.Template.Labels).To(HaveKeyWithValue(k, v))
			}
		})

		It("is idempotent", func() {
			Expect(r.EnsureNodeport(ctx, jenkins)).To(Succeed())
			before := getService().ResourceVersion

			Expect(r.EnsureNodeport(ctx, jenkins)).To(Succeed())

			Expect(getService().ResourceVersion).To(Equal(before), "second call should not update")
		})

		It("returns API errors", func() {
			r = NewJenkinsControllerResourceCreator(newFakeClient(failCreate))

			Expect(r.EnsureNodeport(ctx, jenkins)).To(MatchError(errInjected))
		})
	})
})
