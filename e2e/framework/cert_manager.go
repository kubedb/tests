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

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/log"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

const (
	IssuerName     = "self-signed-issuer"
	tlsCertFileKey = "tls.crt"
	tlsKeyFileKey  = "tls.key"
)

func (f *Framework) IssuerForDB(dbMeta, caSecretMeta metav1.ObjectMeta, fqn string) *cm_api.Issuer {
	thisIssuerName := rand.WithUniqSuffix(IssuerName)
	labelMap := map[string]string{
		meta_util.NameLabelKey:     fqn,
		meta_util.InstanceLabelKey: dbMeta.Name,
	}
	return &cm_api.Issuer{
		TypeMeta: metav1.TypeMeta{
			Kind: cm_api.IssuerKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      thisIssuerName,
			Namespace: dbMeta.Namespace,
			Labels:    labelMap,
		},
		Spec: cm_api.IssuerSpec{
			IssuerConfig: cm_api.IssuerConfig{
				CA: &cm_api.CAIssuer{
					SecretName: caSecretMeta.Name,
				},
			},
		},
	}
}

func (f *Framework) CreateIssuer(obj *cm_api.Issuer) (*cm_api.Issuer, error) {
	return f.certManagerClient.CertmanagerV1beta1().Issuers(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func (f *Framework) UpdateIssuer(meta metav1.ObjectMeta, transformer func(cm_api.Issuer) cm_api.Issuer) error {
	attempt := 0
	for ; attempt < maxAttempts; attempt = attempt + 1 {
		cur, err := f.certManagerClient.CertmanagerV1beta1().Issuers(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(err) {
			return nil
		} else if err == nil {
			modified := transformer(*cur)
			_, err = f.certManagerClient.CertmanagerV1beta1().Issuers(cur.Namespace).Update(context.TODO(), &modified, metav1.UpdateOptions{})
			if err == nil {
				return nil
			}
		}
		log.Errorf("Attempt %d failed to update Issuer %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(updateRetryInterval)
	}
	return fmt.Errorf("failed to update Issuer %s@%s after %d attempts", meta.Name, meta.Namespace, attempt)
}

func (f *Framework) DeleteIssuer(meta metav1.ObjectMeta) error {
	return f.certManagerClient.CertmanagerV1beta1().Issuers(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}

func (fi *Invocation) InsureIssuer(myMeta metav1.ObjectMeta, fqn string) (*cm_api.Issuer, error) {
	//create cert-manager ca secret
	clientCASecret := fi.SelfSignedCASecret(myMeta, fqn)
	secret, err := fi.CreateSecret(clientCASecret)
	if err != nil {
		return nil, err
	}
	fi.AppendToCleanupList(secret)
	//create issuer
	issuer := fi.IssuerForDB(myMeta, clientCASecret.ObjectMeta, fqn)
	issuer, err = fi.CreateIssuer(issuer)
	if err != nil {
		return nil, err
	}
	fi.AppendToCleanupList(issuer)
	return issuer, err
}
