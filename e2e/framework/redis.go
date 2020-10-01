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
	"fmt"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha2/util"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

func (fi *Invocation) RedisStandalone(version string) *api.Redis {
	return &api.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("redis"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: api.RedisSpec{
			Version:           version,
			TerminationPolicy: api.TerminationPolicyWipeOut,
			StorageType:       api.StorageTypeDurable,
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				StorageClassName: types.StringP(fi.StorageClass),
			},
		},
	}
}

func (fi *Invocation) RedisCluster(version string) *api.Redis {
	redis := fi.RedisStandalone(version)
	redis.Spec.Mode = api.RedisModeCluster
	redis.Spec.Cluster = &api.RedisClusterSpec{
		Master:   types.Int32P(3),
		Replicas: types.Int32P(1),
	}

	return redis
}

func (fi *Invocation) RedisClusterWithSpec(version string, master *int32, replicas *int32) *api.Redis {
	redis := fi.RedisStandalone(version)
	redis.Spec.Mode = api.RedisModeCluster
	redis.Spec.Cluster = &api.RedisClusterSpec{
		Master:   master,
		Replicas: replicas,
	}

	return redis
}

func (f *Framework) CreateRedis(obj *api.Redis) error {
	_, err := f.dbClient.KubedbV1alpha1().Redises(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) GetRedis(meta metav1.ObjectMeta) (*api.Redis, error) {
	return f.dbClient.KubedbV1alpha1().Redises(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
}

func (f *Framework) PatchRedis(meta metav1.ObjectMeta, transform func(*api.Redis) *api.Redis) (*api.Redis, error) {
	redis, err := f.dbClient.KubedbV1alpha1().Redises(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	redis, _, err = util.PatchRedis(context.TODO(), f.dbClient.KubedbV1alpha1(), redis, transform, metav1.PatchOptions{})
	return redis, err
}

func (f *Framework) DeleteRedis(meta metav1.ObjectMeta) error {
	return f.dbClient.KubedbV1alpha1().Redises(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInBackground())
}

func (f *Framework) EventuallyRedis(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.dbClient.KubedbV1alpha1().Redises(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				}
				Expect(err).NotTo(HaveOccurred())
			}
			return true
		},
		time.Minute*12,
		time.Second*5,
	)
}

func (f *Framework) EventuallyRedisPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DatabasePhase {
			db, err := f.dbClient.KubedbV1alpha1().Redises(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyRedisRunning(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			redis, err := f.dbClient.KubedbV1alpha1().Redises(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return redis.Status.Phase == api.DatabasePhaseRunning
		},
		time.Minute*13,
		time.Second*5,
	)
}

func (f *Framework) CleanRedis() {
	redisList, err := f.dbClient.KubedbV1alpha1().Redises(f.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range redisList.Items {
		if _, _, err := util.PatchRedis(context.TODO(), f.dbClient.KubedbV1alpha1(), &e, func(in *api.Redis) *api.Redis {
			in.ObjectMeta.Finalizers = nil
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		}, metav1.PatchOptions{}); err != nil {
			fmt.Printf("error Patching Redis. error: %v", err)
		}
	}
	if err := f.dbClient.KubedbV1alpha1().Redises(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInBackground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of Redis. Error: %v", err)
	}
}
