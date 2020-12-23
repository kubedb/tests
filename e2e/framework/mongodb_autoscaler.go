package framework

import (
	"context"

	"kubedb.dev/apimachinery/apis/autoscaling/v1alpha1"

	"github.com/appscode/go/crypto/rand"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmmeta "kmodules.xyz/client-go/meta"
)

func (i *Invocation) MongoDBAutoscalerCompute(name, namespace string, standalone, replicaset, shard, configServer, mongos *v1alpha1.ComputeAutoscalerSpec) *v1alpha1.MongoDBAutoscaler {
	return &v1alpha1.MongoDBAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mg-autoscaler"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},
		Spec: v1alpha1.MongoDBAutoscalerSpec{
			DatabaseRef: &corev1.LocalObjectReference{
				Name: name,
			},
			Compute: &v1alpha1.MongoDBComputeAutoscalerSpec{
				Standalone:   standalone,
				ReplicaSet:   replicaset,
				ConfigServer: configServer,
				Shard:        shard,
				Mongos:       mongos,
			},
		},
	}
}

func (i *Invocation) MongoDBAutoscalerStorage(name, namespace string, standalone, replicaset, shard, configServer *v1alpha1.StorageAutoscalerSpec) *v1alpha1.MongoDBAutoscaler {
	return &v1alpha1.MongoDBAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mg-autoscaler"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},

		Spec: v1alpha1.MongoDBAutoscalerSpec{
			DatabaseRef: &corev1.LocalObjectReference{
				Name: name,
			},
			Storage: &v1alpha1.MongoDBStorageAutoscalerSpec{
				Standalone:   standalone,
				ReplicaSet:   replicaset,
				ConfigServer: configServer,
				Shard:        shard,
			},
		},
	}
}

func (i *Invocation) CreateMongoDBAutoscaler(obj *v1alpha1.MongoDBAutoscaler) error {
	_, err := i.dbClient.AutoscalingV1alpha1().MongoDBAutoscalers(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) DeleteMongoDBAutoscaler(meta metav1.ObjectMeta) error {
	return f.dbClient.AutoscalingV1alpha1().MongoDBAutoscalers(meta.Namespace).Delete(context.TODO(), meta.Name, kmmeta.DeleteInForeground())
}
