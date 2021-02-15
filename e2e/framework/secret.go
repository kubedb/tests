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

	"kubedb.dev/apimachinery/apis/kubedb"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/log"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	meta_util "kmodules.xyz/client-go/meta"
)

const (
	KeyMySQLPassword = "password"
)

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

func (f *Framework) EventuallyDBSecretCount(meta metav1.ObjectMeta, fqn string) GomegaAsyncAssertion {
	labelMap := map[string]string{
		meta_util.NameLabelKey:     fqn,
		meta_util.InstanceLabelKey: meta.Name,
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

func (fi *Invocation) SecretForDatabaseAuthentication(meta metav1.ObjectMeta, mangedByKubeDB bool) *core.Secret {
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

func (f *Framework) SelfSignedCASecret(meta metav1.ObjectMeta, fqn string) *core.Secret {
	caSecretName := rand.WithUniqSuffix(meta.Name + "-self-signed-ca")

	labelMap := map[string]string{
		meta_util.NameLabelKey:     fqn,
		meta_util.InstanceLabelKey: meta.Name,
	}
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      caSecretName,
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
