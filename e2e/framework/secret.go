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
	"os"
	"time"

	"kubedb.dev/apimachinery/apis/kubedb"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/log"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1 "kmodules.xyz/client-go/core/v1"
	meta_util "kmodules.xyz/client-go/meta"
	"kmodules.xyz/constants/aws"
	"kmodules.xyz/constants/azure"
	"kmodules.xyz/constants/google"
	"kmodules.xyz/constants/openstack"
	"stash.appscode.dev/apimachinery/pkg/restic"
)

const (
	StorageProviderGCS   = "gcs"
	StorageProviderAzure = "azure"
	StorageProviderS3    = "s3"
	StorageProviderMinio = "minio"
	StorageProviderSwift = "swift"
	KeyMySQLPassword     = "password"
)

func (i *Invocation) SecretForBackend() *core.Secret {
	secret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-" + StorageProvider),
			Namespace: i.namespace,
		},
	}

	switch StorageProvider {
	case StorageProviderGCS:
		secret.Data = i.gcsCredentials()
	case StorageProviderS3:
		secret.Data = i.s3Credentials()
	case StorageProviderMinio:
		secret.Data = i.minioCredentials()
	case StorageProviderAzure:
		secret.Data = i.azureCredentials()
	case StorageProviderSwift:
		secret.Data = i.swiftCredentials()
	}

	return secret
}

func (i *Invocation) s3Credentials() map[string][]byte {
	if os.Getenv(aws.AWS_ACCESS_KEY_ID) == "" ||
		os.Getenv(aws.AWS_SECRET_ACCESS_KEY) == "" {
		return nil
	}

	return map[string][]byte{
		aws.AWS_ACCESS_KEY_ID:     []byte(os.Getenv(aws.AWS_ACCESS_KEY_ID)),
		aws.AWS_SECRET_ACCESS_KEY: []byte(os.Getenv(aws.AWS_SECRET_ACCESS_KEY)),
	}
}

func (i *Invocation) minioCredentials() map[string][]byte {
	return map[string][]byte{
		aws.AWS_ACCESS_KEY_ID:     []byte(MINIO_ACCESS_KEY_ID),
		aws.AWS_SECRET_ACCESS_KEY: []byte(MINIO_SECRET_ACCESS_KEY),
		aws.CA_CERT_DATA:          i.CertStore.CACertBytes(),
	}
}

func (i *Invocation) gcsCredentials() map[string][]byte {
	jsonKey := google.ServiceAccountFromEnv()
	if jsonKey == "" || os.Getenv(google.GOOGLE_PROJECT_ID) == "" {
		return nil
	}

	return map[string][]byte{
		google.GOOGLE_PROJECT_ID:               []byte(os.Getenv(google.GOOGLE_PROJECT_ID)),
		google.GOOGLE_SERVICE_ACCOUNT_JSON_KEY: []byte(jsonKey),
	}
}

func (i *Invocation) azureCredentials() map[string][]byte {
	if os.Getenv(azure.AZURE_ACCOUNT_NAME) == "" ||
		os.Getenv(azure.AZURE_ACCOUNT_KEY) == "" {
		return nil
	}

	return map[string][]byte{
		azure.AZURE_ACCOUNT_NAME: []byte(os.Getenv(azure.AZURE_ACCOUNT_NAME)),
		azure.AZURE_ACCOUNT_KEY:  []byte(os.Getenv(azure.AZURE_ACCOUNT_KEY)),
	}
}

func (i *Invocation) swiftCredentials() map[string][]byte {
	if os.Getenv(openstack.OS_AUTH_URL) == "" ||
		(os.Getenv(openstack.OS_TENANT_ID) == "" && os.Getenv(openstack.OS_TENANT_NAME) == "") ||
		os.Getenv(openstack.OS_USERNAME) == "" ||
		os.Getenv(openstack.OS_PASSWORD) == "" {
		return nil
	}

	return map[string][]byte{
		openstack.OS_AUTH_URL:    []byte(os.Getenv(openstack.OS_AUTH_URL)),
		openstack.OS_TENANT_ID:   []byte(os.Getenv(openstack.OS_TENANT_ID)),
		openstack.OS_TENANT_NAME: []byte(os.Getenv(openstack.OS_TENANT_NAME)),
		openstack.OS_USERNAME:    []byte(os.Getenv(openstack.OS_USERNAME)),
		openstack.OS_PASSWORD:    []byte(os.Getenv(openstack.OS_PASSWORD)),
		openstack.OS_REGION_NAME: []byte(os.Getenv(openstack.OS_REGION_NAME)),
	}
}

func (i *Invocation) PatchSecretForRestic(secret *core.Secret) *core.Secret {
	if secret == nil {
		return secret
	}

	secret.StringData = v1.UpsertMap(secret.StringData, map[string]string{
		restic.RESTIC_PASSWORD: "RESTIC_PASSWORD",
	})

	return secret
}

func (f *Framework) CreateSecret(obj *core.Secret) (*core.Secret, error) {
	return f.kubeClient.CoreV1().Secrets(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func (f *Framework) UpdateSecret(meta metav1.ObjectMeta, transformer func(core.Secret) core.Secret) error {
	attempt := 0
	for ; attempt < maxAttempts; attempt = attempt + 1 {
		cur, err := f.kubeClient.CoreV1().Secrets(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(err) {
			return nil
		} else if err == nil {
			modified := transformer(*cur)
			_, err = f.kubeClient.CoreV1().Secrets(cur.Namespace).Update(context.TODO(), &modified, metav1.UpdateOptions{})
			if err == nil {
				return nil
			}
		}
		log.Errorf("Attempt %d failed to update Secret %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(updateRetryInterval)
	}
	return fmt.Errorf("failed to update Secret %s@%s after %d attempts", meta.Name, meta.Namespace, attempt)
}

func (f *Framework) GetMongoDBRootPassword(mongodb *api.MongoDB) (string, error) {
	secret, err := f.kubeClient.CoreV1().Secrets(mongodb.Namespace).Get(context.TODO(), mongodb.Spec.AuthSecret.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	password := string(secret.Data[core.BasicAuthPasswordKey])
	return password, nil
}

func (f *Framework) DeleteSecret(meta metav1.ObjectMeta) error {
	return f.kubeClient.CoreV1().Secrets(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}

func (f *Framework) EventuallyDBSecretCount(meta metav1.ObjectMeta, kind string) GomegaAsyncAssertion {
	labelMap := map[string]string{
		api.LabelDatabaseKind: kind,
		api.LabelDatabaseName: meta.Name,
	}
	labelSelector := labels.SelectorFromSet(labelMap)

	return Eventually(
		func() int {
			secretList, err := f.kubeClient.CoreV1().Secrets(meta.Namespace).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			Expect(err).NotTo(HaveOccurred())

			return len(secretList.Items)
		},
		WaitTimeOut,
		PullInterval,
	)
}

func (f *Framework) CheckSecret(secret *core.Secret) error {
	_, err := f.kubeClient.CoreV1().Secrets(f.namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
	return err
}

func (i *Invocation) SecretForDatabaseAuthentication(meta metav1.ObjectMeta, mangedByKubeDB bool) *core.Secret {
	//mangedByKubeDB mimics a secret created and manged by kubedb and not User.
	// It should get deleted during wipeout
	randPassword := ""

	// if the password starts with "-" it will cause error in bash scripts (in mongodb-tools)
	for randPassword = rand.GeneratePassword(); randPassword[0] == '-'; randPassword = rand.GeneratePassword() {
	}

	var dbObjectMeta = metav1.ObjectMeta{
		Name:      fmt.Sprintf("kubedb-%v-%v", meta.Name, CustomSecretSuffix),
		Namespace: meta.Namespace,
	}
	if mangedByKubeDB {
		dbObjectMeta.Labels = map[string]string{
			meta_util.ManagedByLabelKey: kubedb.GroupName,
		}
	}

	return &core.Secret{
		ObjectMeta: dbObjectMeta,
		Type:       core.SecretTypeOpaque,
		StringData: map[string]string{
			KeyMongoDBUser:     mongodbUser,
			KeyMongoDBPassword: randPassword,
		},
	}
}

func (f *Framework) SelfSignedCASecret(meta metav1.ObjectMeta, kind string) *core.Secret {
	labelMap := map[string]string{
		api.LabelDatabaseName: meta.Name,
		api.LabelDatabaseKind: kind,
	}
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name + "-self-signed-ca",
			Namespace: meta.Namespace,
			Labels:    labelMap,
		},
		Data: map[string][]byte{
			tlsCertFileKey: f.CertStore.CACertBytes(),
			tlsKeyFileKey:  f.CertStore.CAKeyBytes(),
		},
	}
}

func (f *Framework) GetMySQLRootPassword(my *api.MySQL) (string, error) {
	secret, err := f.kubeClient.CoreV1().Secrets(my.Namespace).Get(context.TODO(), my.Spec.AuthSecret.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	password := string(secret.Data[KeyMySQLPassword])
	return password, nil
}
