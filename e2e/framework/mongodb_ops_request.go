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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmmeta "kmodules.xyz/client-go/meta"
)

func (i *Invocation) MongoDBOpsRequestUpgrade(name, namespace, version string) *dbaapi.MongoDBOpsRequest {
	return &dbaapi.MongoDBOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mor"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},

		Spec: dbaapi.MongoDBOpsRequestSpec{
			Type: dbaapi.OpsRequestTypeUpgrade,
			Upgrade: &dbaapi.MongoDBUpgradeSpec{
				TargetVersion: version,
			},
			DatabaseRef: corev1.LocalObjectReference{
				Name: name,
			},
		},
	}
}

func (i *Invocation) MongoDBOpsRequestHorizontalScale(name, namespace string, shard *dbaapi.MongoDBShardNode, configServer *dbaapi.ConfigNode, mongos *dbaapi.MongosNode, replicas *int32) *dbaapi.MongoDBOpsRequest {
	return &dbaapi.MongoDBOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mor"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},

		Spec: dbaapi.MongoDBOpsRequestSpec{
			Type: dbaapi.OpsRequestTypeHorizontalScaling,
			DatabaseRef: corev1.LocalObjectReference{
				Name: name,
			},
			HorizontalScaling: &dbaapi.MongoDBHorizontalScalingSpec{
				Shard:        shard,
				ConfigServer: configServer,
				Mongos:       mongos,
				Replicas:     replicas,
			},
		},
	}
}

func (i *Invocation) MongoDBOpsRequestVerticalScale(name, namespace string, containers, replicaset, mongos, configServer, shard, exporter *corev1.ResourceRequirements) *dbaapi.MongoDBOpsRequest {
	return &dbaapi.MongoDBOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mor"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},

		Spec: dbaapi.MongoDBOpsRequestSpec{
			Type: dbaapi.OpsRequestTypeVerticalScaling,
			DatabaseRef: corev1.LocalObjectReference{
				Name: name,
			},
			VerticalScaling: &dbaapi.MongoDBVerticalScalingSpec{
				Standalone:   containers,
				ReplicaSet:   replicaset,
				Mongos:       mongos,
				ConfigServer: configServer,
				Shard:        shard,
				Exporter:     exporter,
			},
		},
	}
}

func (i *Invocation) MongoDBOpsRequestVolumeExpansion(name, namespace string, standalone, replicaset, shard, configServer *resource.Quantity) *dbaapi.MongoDBOpsRequest {
	return &dbaapi.MongoDBOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mor"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},

		Spec: dbaapi.MongoDBOpsRequestSpec{
			Type: dbaapi.OpsRequestTypeVolumeExpansion,
			DatabaseRef: corev1.LocalObjectReference{
				Name: name,
			},
			VolumeExpansion: &dbaapi.MongoDBVolumeExpansionSpec{
				Shard:        shard,
				ConfigServer: configServer,
				Standalone:   standalone,
				ReplicaSet:   replicaset,
			},
		},
	}
}

func (i *Invocation) MongoDBOpsRequestReconfigure(name, namespace string, standalone, replicaset, shard, configServer, mongos *dbaapi.MongoDBCustomConfiguration) *dbaapi.MongoDBOpsRequest {
	return &dbaapi.MongoDBOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mor"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},

		Spec: dbaapi.MongoDBOpsRequestSpec{
			Type: dbaapi.OpsRequestTypeReconfigure,
			DatabaseRef: corev1.LocalObjectReference{
				Name: name,
			},
			Configuration: &dbaapi.MongoDBCustomConfigurationSpec{
				Shard:        shard,
				ConfigServer: configServer,
				Standalone:   standalone,
				ReplicaSet:   replicaset,
				Mongos:       mongos,
			},
		},
	}
}

func (i *Invocation) CreateMongoDBOpsRequest(obj *dbaapi.MongoDBOpsRequest) error {
	_, err := i.dbClient.OpsV1alpha1().MongoDBOpsRequests(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) DeleteMongoDBOpsRequest(meta metav1.ObjectMeta) error {
	return f.dbClient.OpsV1alpha1().MongoDBOpsRequests(meta.Namespace).Delete(context.TODO(), meta.Name, kmmeta.DeleteInForeground())
}

func (f *Framework) EventuallyMongoDBOpsRequestPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() dbaapi.OpsRequestPhase {
			db, err := f.dbClient.OpsV1alpha1().MongoDBOpsRequests(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		time.Minute*30,
		time.Second*5,
	)
}
