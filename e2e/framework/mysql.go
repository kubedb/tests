/*
Copyright The KubeDB Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
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

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/matcher"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

func (fi *Invocation) MySQLDefinition(version string) *api.MySQL {
	return &api.MySQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mysql"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: api.MySQLSpec{
			Version:           version,
			TerminationPolicy: api.TerminationPolicyWipeOut,
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
					},
				},
				AccessModes: []core.PersistentVolumeAccessMode{
					core.ReadWriteOnce,
				},
				StorageClassName: types.StringP(fi.StorageClass),
			},
		},
	}
}

func (fi *Invocation) CreateMySQL(obj *api.MySQL) (*api.MySQL, error) {
	return fi.dbClient.KubedbV1alpha1().MySQLs(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func (f *Framework) GetMySQL(meta metav1.ObjectMeta) (*api.MySQL, error) {
	return f.dbClient.KubedbV1alpha1().MySQLs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
}

func (f *Framework) EventuallyMySQLRunning(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			my, err := f.dbClient.KubedbV1alpha1().MySQLs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return my.Status.Phase == api.DatabasePhaseRunning
		},
		Timeout,
		RetryInterval,
	)
}

func (f *Framework) DeleteMySQL(meta metav1.ObjectMeta) error {
	return f.dbClient.KubedbV1alpha1().MySQLs(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}

func (fi *Invocation) CreateMySQLAndWaitForRunning(version string, transformFuncs ...func(in *api.MySQL)) (*api.MySQL, error) {
	// Generate MySQL definition
	my := fi.MySQLDefinition(version)
	// transformFunc provide a function that made test specific change on the MySQL
	// apply these test specific changes
	for _, fn := range transformFuncs {
		fn(my)
	}

	By("Create MySQL: " + my.Namespace + "/" + my.Name)
	my, err := fi.CreateMySQL(my)
	if err != nil {
		return my, err
	}
	fi.AppendToCleanupList(my)

	By("Wait for Running mysql")
	fi.EventuallyMySQLRunning(my.ObjectMeta).Should(BeTrue())

	By("Wait for AppBinding to create")
	fi.EventuallyAppBinding(my.ObjectMeta).Should(BeTrue())

	By("Check valid AppBinding Specs")
	err = fi.CheckMySQLAppBindingSpec(my.ObjectMeta)
	if err != nil {
		return my, err
	}

	if my.Spec.TLS == nil {
		for i := int32(0); i < *my.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", my.Name, i))
			fi.EventuallyDBConnection(my.ObjectMeta, 0, int(i), MySQLRootUser, "").Should(BeTrue())
		}

		if my.Spec.Topology != nil && my.Spec.Topology.Mode != nil {
			for i := int32(0); i < *my.Spec.Replicas; i++ {
				By(fmt.Sprintf("Checking ONLINE member count from Pod '%s-%d'", my.Name, i))
				fi.EventuallyONLINEMembersCount(my.ObjectMeta, 0, int(i), MySQLRootUser, "").Should(Equal(int(*my.Spec.Replicas)))
			}
		}
	}

	if my.Spec.TLS != nil {
		// get requireSecureTransport
		requireSecureTransport := func(requireSSL bool) string {
			if requireSSL {
				return RequiredSecureTransportON
			} else {
				return RequiredSecureTransportOFF
			}
		}(my.Spec.RequireSSL)

		fi.checkClientConnectionForRootUser(my, requireSecureTransport)

		By("Checking MySQL SSL server settings")
		sslConfigVar := []string{
			fmt.Sprintf("require_secure_transport=%s", requireSecureTransport),
			"have_ssl=YES",
			"have_openssl=YES",
			// in MySQL, certs are stored in "/etc/mysql/certs" path
			"ssl_ca=/etc/mysql/certs/ca.crt",
			"ssl_cert=/etc/mysql/certs/server.crt",
			"ssl_key=/etc/mysql/certs/server.key",
		}

		for _, cfg := range sslConfigVar {
			fi.EventuallyCheckSSLSettings(my.ObjectMeta, fmt.Sprintf("tls=%s", TLSCustomConfig), cfg).Should(matcher.HaveSSL(cfg))
		}

		// create a mysql user with required SSL
		By("Create mysql user with required SSL")
		fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, fmt.Sprintf("tls=%s", TLSCustomConfig)).Should(BeTrue())

		fi.checkClientConnectionForRequiredSSLUser(my)
	}

	return my, err
}

func (f *Framework) checkClientConnectionForRootUser(my *api.MySQL, requireSecureTransport string) {
	params := []string{
		fmt.Sprintf("tls=%s", TLSSkibVerify),
		fmt.Sprintf("tls=%s", TLSCustomConfig),
	}
	if requireSecureTransport == RequiredSecureTransportOFF {
		params = append(params, fmt.Sprintf("tls=%s", TLSFalse))
	}

	for _, param := range params {
		By(fmt.Sprintf("checking root user connection with tls: %s", param))
		for i := int32(0); i < *my.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", my.Name, i))
			f.EventuallyDBConnection(my.ObjectMeta, 0, int(i), MySQLRootUser, param).Should(BeTrue())
		}

		if my.Spec.Topology != nil && my.Spec.Topology.Mode != nil {
			for i := int32(0); i < *my.Spec.Replicas; i++ {
				By(fmt.Sprintf("Checking ONLINE member count from Pod '%s-%d'", my.Name, i))
				f.EventuallyONLINEMembersCount(my.ObjectMeta, 0, int(i), MySQLRootUser, param).Should(Equal(int(*my.Spec.Replicas)))
			}
		}
	}
}

func (f *Framework) checkClientConnectionForRequiredSSLUser(my *api.MySQL) {
	params := []string{
		fmt.Sprintf("tls=%s", TLSSkibVerify),
		fmt.Sprintf("tls=%s", TLSCustomConfig),
	}
	for _, param := range params {
		By(fmt.Sprintf("checking ssl required user connection with tls: %s", param))
		for i := int32(0); i < *my.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", my.Name, i))
			f.EventuallyDBConnection(my.ObjectMeta, 0, int(i), MySQLRequiredSSLUser, param).Should(BeTrue())
		}

		if my.Spec.Topology != nil && my.Spec.Topology.Mode != nil {
			for i := int32(0); i < *my.Spec.Replicas; i++ {
				By(fmt.Sprintf("Checking ONLINE member count from Pod '%s-%d'", my.Name, i))
				f.EventuallyONLINEMembersCount(my.ObjectMeta, 0, int(i), MySQLRootUser, param).Should(Equal(int(*my.Spec.Replicas)))
			}
		}
	}
}
