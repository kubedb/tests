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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kmodules.xyz/client-go/meta"
	store "kmodules.xyz/objectstore-api/api/v1"
	stash_v1alpha1 "stash.appscode.dev/apimachinery/apis/stash/v1alpha1"
	stash_v1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

func (fi *Invocation) NewBackupBlueprint(secretName string) *stash_v1beta1.BackupBlueprint {
	return &stash_v1beta1.BackupBlueprint{
		ObjectMeta: metav1.ObjectMeta{
			Name: meta.NameWithSuffix("blueprint", fi.app),
		},
		Spec: stash_v1beta1.BackupBlueprintSpec{
			RepositorySpec: stash_v1alpha1.RepositorySpec{
				Backend: store.Backend{
					S3: &store.S3Spec{
						Endpoint: fi.MinioServiceAddres(),
						Bucket:   fi.app,
						Prefix:   fmt.Sprintf("kubedb/%s/%s", fi.namespace, fi.app),
					},
					StorageSecretName: secretName,
				},
				WipeOut: false,
			},
			Schedule: "0 0 * 12 *",
			RetentionPolicy: stash_v1alpha1.RetentionPolicy{
				Name:     "keep-last-5",
				KeepLast: 5,
				Prune:    true,
			},
		},
	}
}

func (fi *Invocation) CreateBackupBlueprint() *stash_v1beta1.BackupBlueprint {
	// Create Secret for BackupBlueprint
	secret := fi.CreateSecretForMinioBackend()

	// Generate BackupBlueprint definition
	bb := fi.NewBackupBlueprint(secret.Name)
	bb.Spec.Task.Name = getBackupAddonName()

	By(fmt.Sprintf("Creating BackupBlueprint: %s", bb.Name))
	createdBB, err := fi.StashClient.StashV1beta1().BackupBlueprints().Create(context.TODO(), bb, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	fi.AppendToCleanupList(createdBB)
	return createdBB
}
