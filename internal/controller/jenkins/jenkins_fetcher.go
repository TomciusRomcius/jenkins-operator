package jenkins

import (
	"context"

	civ1alpha1 "github.com/tomciusromcius/jenkins-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func FetchJenkins(ctx context.Context, client client.Client, objectKey types.NamespacedName) (*civ1alpha1.Jenkins, error) {
	jenkins := &civ1alpha1.Jenkins{}
	err := client.Get(ctx, objectKey, jenkins)
	if err != nil {
		return nil, err
	}
	return jenkins, nil
}
