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
	"crypto/x509"
	"fmt"
	"path/filepath"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	"github.com/appscode/go/ioutil"
	"github.com/pkg/errors"
	"gomodules.xyz/cert"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kmodules.xyz/client-go/tools/exec"
)

const (
	certPath = "/tmp/mongodb/"
)

// GetSSLCertificate gets ssl certificate of mongodb and creates a client certificate in certPath
func (f *Framework) GetSSLCertificate(meta metav1.ObjectMeta) error {
	mg, err := f.GetMongoDB(meta)
	if err != nil {
		return err
	}

	certSecret, err := f.kubeClient.CoreV1().Secrets(mg.Namespace).Get(context.TODO(), mg.GetCertSecretName(api.MongoDBClientCert, ""), metav1.GetOptions{})
	if err != nil {
		return err
	}

	caCertBytes := certSecret.Data[string(api.TLSCACertFileName)]
	certBytes := append(certSecret.Data[string(tlsCertFileKey)], certSecret.Data[string(tlsKeyFileKey)]...)

	if !ioutil.WriteString(filepath.Join(certPath, string(api.TLSCACertFileName)), string(caCertBytes)) {
		return errors.New("failed to write client certificate")
	}

	if !ioutil.WriteString(filepath.Join(certPath, string(api.MongoPemFileName)), string(certBytes)) {
		return errors.New("failed to write client certificate")
	}

	return nil
}

func (f *Framework) GetCertificateFromPod(pod *core.Pod, path string) (*x509.Certificate, error) {
	command := []string{"cat", path}
	certString, err := exec.ExecIntoPod(f.restConfig, pod, exec.Command(command...))
	if err != nil {
		return nil, err
	}
	certs, err := cert.ParseCertsPEM([]byte(certString))
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, errors.New("certificate is empty")
	}
	return certs[0], nil
}

func (f *Framework) GetCertificateFromSecret(secretName, namespace string) (*x509.Certificate, error) {
	secret, err := f.kubeClient.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var crt []byte
	if value, ok := secret.Data[core.TLSCertKey]; ok {
		crt = value
	} else {
		return nil, errors.New(fmt.Sprintf("tls.crt is missing in secret: %s/%s", namespace, secretName))
	}

	certs, err := cert.ParseCertsPEM(crt)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, errors.New("certificate is empty")
	}
	return certs[0], nil
}
