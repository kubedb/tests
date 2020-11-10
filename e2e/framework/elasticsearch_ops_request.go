package framework

import (
	"context"
	"time"

	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (i *Invocation) GetElasticsearchOpsRequestUpgrade(esMeta metav1.ObjectMeta, targetVersion string) *dbaapi.ElasticsearchOpsRequest {
	return &dbaapi.ElasticsearchOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("es-upgrade"),
			Namespace: i.namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},

		Spec: dbaapi.ElasticsearchOpsRequestSpec{
			Type: dbaapi.OpsRequestTypeUpgrade,
			Upgrade: &dbaapi.ElasticsearchUpgradeSpec{
				TargetVersion: targetVersion,
			},
			DatabaseRef: corev1.LocalObjectReference{
				Name: esMeta.Name,
			},
		},
	}
}

func (i *Invocation) EventuallyElasticsearchOpsRequestSuccessful(meta metav1.ObjectMeta, timeOut time.Duration) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			esOpsReq, err := i.dbClient.OpsV1alpha1().ElasticsearchOpsRequests(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return esOpsReq.Status.Phase == dbaapi.OpsRequestPhaseSuccessful
		},
		timeOut,
		PullInterval,
	)
}
