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

	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	meta_util "kmodules.xyz/client-go/meta"
)

func (f *Framework) EventuallyPVCCount(meta metav1.ObjectMeta, fqn string) GomegaAsyncAssertion {
	labelMap := map[string]string{
		meta_util.NameLabelKey:     fqn,
		meta_util.InstanceLabelKey: meta.Name,
	}
	labelSelector := labels.SelectorFromSet(labelMap)

	return Eventually(
		func() int {
			pvcList, err := f.kubeClient.CoreV1().PersistentVolumeClaims(meta.Namespace).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			Expect(err).NotTo(HaveOccurred())

			return len(pvcList.Items)
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (fi *Invocation) PersistentVolumeClaim() *core.PersistentVolumeClaim {
	return &core.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fi.app,
			Namespace: fi.namespace,
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadWriteOnce,
			},
			StorageClassName: &fi.StorageClass,
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceName(core.ResourceStorage): resource.MustParse("50Mi"),
				},
			},
		},
	}
}

func (i *Invocation) GetPersistentVolumeClaim(name string) (*core.PersistentVolumeClaim, error) {
	return i.kubeClient.CoreV1().PersistentVolumeClaims(i.Namespace()).Get(context.TODO(), name, metav1.GetOptions{})
}

func (fi *Invocation) DeletePersistentVolumeClaim(meta metav1.ObjectMeta) error {
	return fi.kubeClient.CoreV1().PersistentVolumeClaims(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}
