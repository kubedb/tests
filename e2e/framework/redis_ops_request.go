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

	api "kubedb.dev/apimachinery/apis/ops/v1alpha1"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

func (fi *Invocation) RedisOpsRequestUpgrade(name, version string, typ api.OpsRequestType) *api.RedisOpsRequest {
	return &api.RedisOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mmr"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},

		Spec: api.RedisOpsRequestSpec{
			Type: typ,
			Upgrade: &api.RedisUpgradeSpec{
				TargetVersion: version,
			},
			DatabaseRef: v1.LocalObjectReference{
				Name: name,
			},
		},
	}
}

func (fi *Invocation) RedisOpsRequestHorizontalScale(name, namespace string, scale *api.RedisHorizontalScalingSpec) *api.RedisOpsRequest {
	return &api.RedisOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mmr"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},

		Spec: api.RedisOpsRequestSpec{
			Type: api.OpsRequestTypeHorizontalScaling,
			DatabaseRef: v1.LocalObjectReference{
				Name: name,
			},
			HorizontalScaling: scale,
		},
	}
}

func (fi *Invocation) RedisOpsRequestVerticalScale(name, namespace string, containers, exporter *v1.ResourceRequirements) *api.RedisOpsRequest {
	return &api.RedisOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mmr"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},

		Spec: api.RedisOpsRequestSpec{
			Type: api.OpsRequestTypeVerticalScaling,
			DatabaseRef: v1.LocalObjectReference{
				Name: name,
			},
			VerticalScaling: &api.RedisVerticalScalingSpec{
				Redis:    containers,
				Exporter: exporter,
			},
		},
	}
}

func (fi *Invocation) RedisOpsRequestVolumeExpansion(name, namespace string, redis *resource.Quantity) *api.RedisOpsRequest {
	return &api.RedisOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mmr"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},

		Spec: api.RedisOpsRequestSpec{
			Type: api.OpsRequestTypeVolumeExpansion,
			DatabaseRef: v1.LocalObjectReference{
				Name: name,
			},
			VolumeExpansion: &api.RedisVolumeExpansionSpec{
				Redis: redis,
			},
		},
	}
}

func (fi *Invocation) RedisOpsRequestReconfiguration(name, namespace string, configSpec *api.RedisCustomConfigurationSpec) *api.RedisOpsRequest {
	return &api.RedisOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mmr"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},

		Spec: api.RedisOpsRequestSpec{
			Type: api.OpsRequestTypeReconfigure,
			DatabaseRef: v1.LocalObjectReference{
				Name: name,
			},
			Configuration: configSpec,
		},
	}
}

func (fi *Invocation) RedisOpsRequestTLSReconfiguration(name, namespace string, tlsSpec *api.TLSSpec) *api.RedisOpsRequest {
	return &api.RedisOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mmr"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},

		Spec: api.RedisOpsRequestSpec{
			Type: api.OpsRequestTypeReconfigureTLSs,
			DatabaseRef: v1.LocalObjectReference{
				Name: name,
			},
			TLS: tlsSpec,
		},
	}
}

func (fi *Invocation) CreateRedisOpsRequest(obj *api.RedisOpsRequest) (*api.RedisOpsRequest, error) {
	return fi.dbClient.OpsV1alpha1().RedisOpsRequests(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func (fi *Invocation) DeleteRedisOpsRequest(meta metav1.ObjectMeta) error {
	return fi.dbClient.OpsV1alpha1().RedisOpsRequests(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInBackground())
}

func (f *Framework) EventuallyRedisOpsRequestPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.OpsRequestPhase {
			db, err := f.dbClient.OpsV1alpha1().RedisOpsRequests(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		time.Minute*20,
		time.Second*5,
	)
}
