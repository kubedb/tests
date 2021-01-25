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
	"strings"
	"time"

	"kubedb.dev/apimachinery/apis/autoscaling/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmmeta "kmodules.xyz/client-go/meta"
	"kmodules.xyz/client-go/tools/exec"
)

func (i *Invocation) ElasticsearchAutoscalerCompute(name, namespace string, computeSpec *v1alpha1.ElasticsearchComputeAutoscalerSpec) *v1alpha1.ElasticsearchAutoscaler {
	return &v1alpha1.ElasticsearchAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("es-autoscaler"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},
		Spec: v1alpha1.ElasticsearchAutoscalerSpec{
			DatabaseRef: &corev1.LocalObjectReference{
				Name: name,
			},
			Compute: computeSpec,
		},
	}
}

func (i *Invocation) ElasticsearchAutoscalerStorage(name, namespace string, storageSpec *v1alpha1.ElasticsearchStorageAutoscalerSpec) *v1alpha1.ElasticsearchAutoscaler {
	return &v1alpha1.ElasticsearchAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("es-autoscaler"),
			Namespace: namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},

		Spec: v1alpha1.ElasticsearchAutoscalerSpec{
			DatabaseRef: &corev1.LocalObjectReference{
				Name: name,
			},
			Storage: storageSpec,
		},
	}
}

func (i *Invocation) CreateElasticsearchAutoscaler(obj *v1alpha1.ElasticsearchAutoscaler) error {
	_, err := i.dbClient.AutoscalingV1alpha1().ElasticsearchAutoscalers(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) DeleteElasticsearchAutoscaler(meta metav1.ObjectMeta) error {
	return f.dbClient.AutoscalingV1alpha1().ElasticsearchAutoscalers(meta.Namespace).Delete(context.TODO(), meta.Name, kmmeta.DeleteInForeground())
}

func (f *Framework) FillDiskElasticsearch(db *api.Elasticsearch, storage *v1alpha1.ElasticsearchStorageAutoscalerSpec) error {
	var (
		podName string
		pod     *corev1.Pod
		err     error
	)

	if storage.Node != nil {
		podName = fmt.Sprintf("%v-0", db.OffshootName())
	} else if storage.Topology != nil {
		if storage.Topology.Ingest != nil {
			podName = fmt.Sprintf("%v-0", db.IngestStatefulSetName())
		} else if storage.Topology.Data != nil {
			podName = fmt.Sprintf("%v-0", db.DataStatefulSetName())
		} else if storage.Topology.Master != nil {
			podName = fmt.Sprintf("%v-0", db.MasterStatefulSetName())
		}
	}
	pod, err = f.kubeClient.CoreV1().Pods(f.namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	command := []string{"dd", "if=/dev/zero", "of=/usr/share/elasticsearch/data/file.txt", "count=1024", "bs=548576", "status=none"}

	_, err = exec.ExecIntoPod(f.restConfig, pod, exec.Command(command...))
	if err != nil {
		return err
	}

	return nil
}

func (f *Framework) getDiskSizeElasticsearch(db *api.Elasticsearch, storage *v1alpha1.ElasticsearchStorageAutoscalerSpec) (float64, error) {
	var (
		podName string
		pod     *core.Pod
		err     error
	)

	if storage.Node != nil {
		podName = fmt.Sprintf("%v-0", db.OffshootName())
	} else if storage.Topology != nil {
		if storage.Topology.Ingest != nil {
			podName = fmt.Sprintf("%v-0", db.IngestStatefulSetName())
		} else if storage.Topology.Data != nil {
			podName = fmt.Sprintf("%v-0", db.DataStatefulSetName())
		} else if storage.Topology.Master != nil {
			podName = fmt.Sprintf("%v-0", db.MasterStatefulSetName())
		}
	}

	pod, err = f.kubeClient.CoreV1().Pods(f.namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return -1, err
	}
	command := []string{"df", "/usr/share/elasticsearch/data", "--output=size"}
	res, err := exec.ExecIntoPod(f.restConfig, pod, exec.Command(command...))
	if err != nil {
		return -1, err
	}

	s := strings.Split(res, "\n")
	if len(s) >= 2 {
		return strconv.ParseFloat(strings.Trim(s[1], " "), 64)
	} else {
		return -1, errors.New("bad df output")
	}
}

func (f *Framework) EventuallyVolumeExpandedElasticsearch(db *api.Elasticsearch, storage *v1alpha1.ElasticsearchStorageAutoscalerSpec) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			size, err := f.getDiskSizeElasticsearch(db, storage)
			if err != nil {
				return false, err
			}
			if size > 1048576 { //1GB
				return true, nil
			}
			return false, nil
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) getCurrentCPUElasticsearch(prevDB *api.Elasticsearch, compute *v1alpha1.ElasticsearchComputeAutoscalerSpec) (bool, error) {
	db, err := f.dbClient.KubedbV1alpha2().Elasticsearches(prevDB.Namespace).Get(context.TODO(), prevDB.Name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if compute.Node != nil {
		return db.Spec.PodTemplate.Spec.Resources.Requests.Cpu().MilliValue() > prevDB.Spec.PodTemplate.Spec.Resources.Requests.Cpu().MilliValue(), nil
	} else if compute.Topology != nil {
		if compute.Topology.Master != nil {
			return db.Spec.Topology.Master.Resources.Requests.Cpu().MilliValue() > prevDB.Spec.Topology.Master.Resources.Requests.Cpu().MilliValue(), nil
		} else if compute.Topology.Data != nil {
			return db.Spec.Topology.Data.Resources.Requests.Cpu().MilliValue() > prevDB.Spec.Topology.Data.Resources.Requests.Cpu().MilliValue(), nil
		} else if compute.Topology.Ingest != nil {
			return db.Spec.Topology.Ingest.Resources.Requests.Cpu().MilliValue() > prevDB.Spec.Topology.Ingest.Resources.Requests.Cpu().MilliValue(), nil
		}
	}

	return false, errors.New("invalid config")
}

func (f *Framework) EventuallyVerticallyScaledElasticsearch(meta metav1.ObjectMeta, compute *v1alpha1.ElasticsearchComputeAutoscalerSpec) GomegaAsyncAssertion {
	db, err := f.dbClient.KubedbV1alpha2().Elasticsearches(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	return Eventually(
		func() (bool, error) {
			return f.getCurrentCPUElasticsearch(db, compute)
		},
		time.Minute*30,
		time.Second*5,
	)
}
