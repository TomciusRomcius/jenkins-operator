package jenkins

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("FetchJenkins", func() {
	ctx := context.Background()

	It("returns the jenkins resource", func() {
		c := newFakeClient(nil, newJenkins())

		got, err := FetchJenkins(ctx, c, key(testName))

		Expect(err).NotTo(HaveOccurred())
		Expect(got.Name).To(Equal(testName))
		Expect(got.Spec.Image).To(Equal(testImage))
	})

	It("returns a not found error when missing", func() {
		c := newFakeClient(nil)

		got, err := FetchJenkins(ctx, c, key("missing"))

		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		Expect(got).To(BeNil())
	})
})
