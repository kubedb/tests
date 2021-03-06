/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Free Trial License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Free-Trial-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"context"
	"time"

	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmmeta "kmodules.xyz/client-go/meta"
)

func (fi *Invocation) GetElasticsearchOpsRequestUpgrade(esMeta metav1.ObjectMeta, targetVersion string) *dbaapi.ElasticsearchOpsRequest {
	return &dbaapi.ElasticsearchOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("es-upgrade"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
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

func (fi *Invocation) GetElasticsearchOpsRequestHorizontalScale(esMeta metav1.ObjectMeta, scaleSpec *dbaapi.ElasticsearchHorizontalScalingSpec) *dbaapi.ElasticsearchOpsRequest {
	return &dbaapi.ElasticsearchOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("es-scale-up"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},

		Spec: dbaapi.ElasticsearchOpsRequestSpec{
			Type: dbaapi.OpsRequestTypeHorizontalScaling,
			DatabaseRef: corev1.LocalObjectReference{
				Name: esMeta.Name,
			},
			HorizontalScaling: scaleSpec,
		},
	}
}

func (fi *Invocation) GetElasticsearchOpsRequestVerticalScale(esMeta metav1.ObjectMeta, scaleSpec *dbaapi.ElasticsearchVerticalScalingSpec) *dbaapi.ElasticsearchOpsRequest {
	return &dbaapi.ElasticsearchOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("es-vertical-scale"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},

		Spec: dbaapi.ElasticsearchOpsRequestSpec{
			Type: dbaapi.OpsRequestTypeVerticalScaling,
			DatabaseRef: corev1.LocalObjectReference{
				Name: esMeta.Name,
			},
			VerticalScaling: scaleSpec,
		},
	}
}

func (fi *Invocation) GetElasticsearchOpsRequestVolumeExpansion(esMeta metav1.ObjectMeta, veSpec *dbaapi.ElasticsearchVolumeExpansionSpec) *dbaapi.ElasticsearchOpsRequest {
	return &dbaapi.ElasticsearchOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("es-volume-expansion"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},

		Spec: dbaapi.ElasticsearchOpsRequestSpec{
			Type: dbaapi.OpsRequestTypeVolumeExpansion,
			DatabaseRef: corev1.LocalObjectReference{
				Name: esMeta.Name,
			},
			VolumeExpansion: veSpec,
		},
	}
}

func (fi *Invocation) EventuallyElasticsearchOpsRequestSuccessful(meta metav1.ObjectMeta, timeOut time.Duration) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			esOpsReq, err := fi.dbClient.OpsV1alpha1().ElasticsearchOpsRequests(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(esOpsReq.Status.Phase).ShouldNot(Equal(dbaapi.OpsRequestPhaseFailed))
			return esOpsReq.Status.Phase == dbaapi.OpsRequestPhaseSuccessful
		},
		timeOut,
		PullInterval,
	)
}

func (i *Invocation) GetElasticsearchOpsRequestReconfigureTLS(esMeta metav1.ObjectMeta, tlsSpec *dbaapi.TLSSpec) *dbaapi.ElasticsearchOpsRequest {
	return &dbaapi.ElasticsearchOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("es-reconfigure-tls"),
			Namespace: i.namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},

		Spec: dbaapi.ElasticsearchOpsRequestSpec{
			Type: dbaapi.OpsRequestTypeReconfigureTLSs,
			DatabaseRef: corev1.LocalObjectReference{
				Name: esMeta.Name,
			},
			TLS: tlsSpec,
		},
	}
}

func (f *Framework) DeleteElasticsearchOpsRequest(meta metav1.ObjectMeta) error {
	return f.dbClient.OpsV1alpha1().ElasticsearchOpsRequests(meta.Namespace).Delete(context.TODO(), meta.Name, kmmeta.DeleteInForeground())
}
