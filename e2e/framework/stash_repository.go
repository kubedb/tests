/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Free Trial License 1.0.0 (the "License");
you may not use this ile except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Free-Trial-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the speciic language governing permissions and
limitations under the License.
*/

package framework

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core_util "kmodules.xyz/client-go/core/v1"
	"kmodules.xyz/client-go/meta"
	store "kmodules.xyz/objectstore-api/api/v1"
	stash_v1alpha1 "stash.appscode.dev/apimachinery/apis/stash/v1alpha1"
	"stash.appscode.dev/apimachinery/pkg/restic"
)

const (
	RESTIC_PASSWORD = "not@secret"
)

func (i *Invocation) NewMinioRepository(secretName string) *stash_v1alpha1.Repository {
	return &stash_v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.NameWithSuffix("minio", i.app),
			Namespace: i.namespace,
		},
		Spec: stash_v1alpha1.RepositorySpec{
			Backend: store.Backend{
				S3: &store.S3Spec{
					Endpoint: i.MinioServiceAddres(),
					Bucket:   i.app,
					Prefix:   "kubedb/backup",
				},
				StorageSecretName: secretName,
			},
			WipeOut: false,
		},
	}
}

func (i *Invocation) SetupMinioRepository() (*stash_v1alpha1.Repository, error) {
	// Create Storage Secret
	By("Creating Storage Secret")
	cred := i.SecretForMinioBackend(true)

	// Generate Repository Definition
	repo := i.NewMinioRepository(cred.Name)

	// Create Repository
	By("Creating Repository")
	repo, err := i.StashClient.StashV1alpha1().Repositories(repo.Namespace).Create(context.TODO(), repo, metav1.CreateOptions{})
	if err != nil {
		return repo, err
	}
	i.AppendToCleanupList(repo)

	// Set Repository as the owner of the storageSecret so that when we delete
	// the Repository, the storage secret get garbage collected
	repo.GetObjectKind().SetGroupVersionKind(stash_v1alpha1.SchemeGroupVersion.WithKind(stash_v1alpha1.ResourceKindRepository))
	_, _, err = core_util.CreateOrPatchSecret(context.TODO(), i.kubeClient, cred.ObjectMeta, func(in *core.Secret) *core.Secret {
		core_util.EnsureOwnerReference(in, core_util.NewOwnerRef(repo, repo.GroupVersionKind()))
		in.Data = cred.Data
		return in
	}, metav1.PatchOptions{})
	return repo, err
}

func (i *Invocation) SecretForMinioBackend(includeCacert bool) *core.Secret {
	secret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.NameWithSuffix("minio", i.app),
			Namespace: i.namespace,
		},
		Data: map[string][]byte{
			restic.RESTIC_PASSWORD:       []byte(RESTIC_PASSWORD),
			restic.AWS_ACCESS_KEY_ID:     []byte(MINIO_ACCESS_KEY_ID),
			restic.AWS_SECRET_ACCESS_KEY: []byte(MINIO_SECRET_ACCESS_KEY),
		},
	}
	if includeCacert {
		secret.Data[restic.CA_CERT_DATA] = i.CertStore.CACertBytes()
	}
	return secret
}

func (i Invocation) CreateSecretForMinioBackend() *core.Secret {
	// Create Storage Secret
	cred := i.SecretForMinioBackend(true)

	By(fmt.Sprintf("Creating Storage Secret for Minio: %s/%s", cred.Namespace, cred.Name))
	createdCred, err := i.CreateSecret(cred)
	Expect(err).NotTo(HaveOccurred())
	i.AppendToCleanupList(&cred)

	return createdCred
}

func (f *Framework) EventuallyRepositoryCreated(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(func() bool {
		_, err := f.StashClient.StashV1alpha1().Repositories(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
		if err == nil && !kerr.IsNotFound(err) {
			return true
		}
		return false
	},
		WaitTimeOut,
		PullInterval,
	)
}
