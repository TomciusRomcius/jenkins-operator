package jenkins

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	civ1alpha1 "github.com/tomciusromcius/jenkins-operator/api/v1alpha1"
)

const (
	testNamespace = "ci"
	testName      = "my-jenkins"
	testImage     = "jenkins/jenkins:lts"
)

func TestJenkins(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jenkins Resources Suite")
}

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(s)).To(Succeed())
	Expect(civ1alpha1.AddToScheme(s)).To(Succeed())
	return s
}

func newJenkins() *civ1alpha1.Jenkins {
	return &civ1alpha1.Jenkins{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			UID:       types.UID("jenkins-uid"),
		},
		Spec: civ1alpha1.JenkinsSpec{Image: testImage},
	}
}

// newFakeClient builds a fake client, optionally with interceptors to inject API errors.
func newFakeClient(funcs *interceptor.Funcs, objs ...client.Object) client.Client {
	b := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(objs...)
	if funcs != nil {
		b = b.WithInterceptorFuncs(*funcs)
	}
	return b.Build()
}

func key(name string) types.NamespacedName {
	return types.NamespacedName{Namespace: testNamespace, Name: name}
}
