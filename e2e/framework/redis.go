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
	"strconv"
	"time"

	"kubedb.dev/apimachinery/apis/kubedb"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha2/util"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1beta1"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kmapi "kmodules.xyz/client-go/api/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

func (fi *Invocation) RedisStandalone(version string) *api.Redis {
	redis := &api.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("redis"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: api.RedisSpec{
			Version:           version,
			Mode:              api.RedisModeStandalone,
			TerminationPolicy: api.TerminationPolicyHalt,
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

	if fi.testConfig.UseTLS {
		redis = fi.RedisWithTLS(redis)
	}

	return redis
}

func (fi *Invocation) RedisCluster(version string, master, replicas *int32) *api.Redis {
	redis := fi.RedisStandalone(version)
	redis.Spec.Mode = api.RedisModeCluster
	if master == nil {
		master = types.Int32P(3)
	}
	if replicas == nil {
		replicas = types.Int32P(1)
	}
	redis.Spec.Cluster = &api.RedisClusterSpec{
		Master:   master,
		Replicas: replicas,
	}

	return redis
}

func (fi *Invocation) RedisWithTLS(redis *api.Redis) *api.Redis {
	issuer, err := fi.EnsureIssuer(redis.ObjectMeta, api.Redis{}.ResourceFQN())
	Expect(err).NotTo(HaveOccurred())
	if redis.Spec.TLS == nil {
		redis.Spec.TLS = &kmapi.TLSConfig{
			IssuerRef: &core.TypedLocalObjectReference{
				Name:     issuer.Name,
				Kind:     "Issuer",
				APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
			},
			Certificates: []kmapi.CertificateSpec{
				{
					Subject: &kmapi.X509Subject{
						Organizations: []string{
							"kubedb:server",
						},
					},
					DNSNames: []string{
						"localhost",
					},
					IPAddresses: []string{
						"127.0.0.1",
					},
				},
			},
		}
	}
	return redis
}

func (f *Framework) CreateRedis(obj *api.Redis) error {
	_, err := f.dbClient.KubedbV1alpha2().Redises(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) GetRedis(meta metav1.ObjectMeta) (*api.Redis, error) {
	return f.dbClient.KubedbV1alpha2().Redises(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
}

func (f *Framework) PatchRedis(meta metav1.ObjectMeta, transform func(*api.Redis) *api.Redis) (*api.Redis, error) {
	redis, err := f.dbClient.KubedbV1alpha2().Redises(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	redis, _, err = util.PatchRedis(context.TODO(), f.dbClient.KubedbV1alpha2(), redis, transform, metav1.PatchOptions{})
	return redis, err
}

func (f *Framework) DeleteRedis(meta metav1.ObjectMeta) error {
	return f.dbClient.KubedbV1alpha2().Redises(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInBackground())
}

func (f *Framework) EventuallyRedis(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.dbClient.KubedbV1alpha2().Redises(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
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
			db, err := f.dbClient.KubedbV1alpha2().Redises(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
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
			redis, err := f.dbClient.KubedbV1alpha2().Redises(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return redis.Status.Phase == api.DatabasePhaseReady
		},
		time.Minute*13,
		time.Second*5,
	)
}

func (f *Framework) CleanRedis() {
	redisList, err := f.dbClient.KubedbV1alpha2().Redises(f.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range redisList.Items {
		if _, _, err := util.PatchRedis(context.TODO(), f.dbClient.KubedbV1alpha2(), &e, func(in *api.Redis) *api.Redis {
			in.ObjectMeta.Finalizers = nil
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		}, metav1.PatchOptions{}); err != nil {
			fmt.Printf("error Patching Redis. error: %v", err)
		}
	}
	if err := f.dbClient.KubedbV1alpha2().Redises(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInBackground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of Redis. Error: %v", err)
	}
}

func (f *Framework) EvictPodsFromStatefulSetRedis(meta metav1.ObjectMeta) error {
	var err error
	labelSelector := labels.Set{
		meta_util.ManagedByLabelKey: kubedb.GroupName,
		meta_util.NameLabelKey:      api.Redis{}.ResourceFQN(),
		meta_util.InstanceLabelKey:  meta.Name,
	}

	// get sts in the namespace
	stsList, err := f.kubeClient.AppsV1().StatefulSets(meta.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return err
	}

	if len(stsList.Items) < 1 {
		return fmt.Errorf("found no statefulset in namespace %s with specific labels", meta.Namespace)
	}

	for _, sts := range stsList.Items {
		// if PDB is not found, send error
		var pdb *policy.PodDisruptionBudget
		pdb, err = f.kubeClient.PolicyV1beta1().PodDisruptionBudgets(sts.Namespace).Get(context.TODO(), sts.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		eviction := &policy.Eviction{
			TypeMeta: metav1.TypeMeta{
				APIVersion: policy.SchemeGroupVersion.String(),
				Kind:       kindEviction,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      sts.Name,
				Namespace: sts.Namespace,
			},
			DeleteOptions: &metav1.DeleteOptions{},
		}

		if pdb.Spec.MaxUnavailable == nil {
			return fmt.Errorf("found pdb %s spec.maxUnavailable nil", pdb.Name)
		}

		// try to evict as many pod as allowed in pdb. No err should occur
		maxUnavailable := pdb.Spec.MaxUnavailable.IntValue()
		for i := 0; i < maxUnavailable; i++ {
			eviction.Name = sts.Name + "-" + strconv.Itoa(i)

			err := f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(context.TODO(), eviction)
			if err != nil {
				return err
			}
		}

		// try to evict one extra pod. TooManyRequests err should occur
		eviction.Name = sts.Name + "-" + strconv.Itoa(maxUnavailable-1)

		err = f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(context.TODO(), eviction)
		if kerr.IsTooManyRequests(err) {
			err = nil
		} else if err != nil {
			return err
		} else {
			return fmt.Errorf("expected pod %s/%s to be not evicted due to pdb %s", sts.Namespace, eviction.Name, pdb.Name)
		}
	}
	return err
}
