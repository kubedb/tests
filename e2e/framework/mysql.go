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
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha2/util"
	"kubedb.dev/tests/e2e/matcher"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
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

func (f *Framework) PatchMySQL(meta metav1.ObjectMeta, transform func(*api.MySQL) *api.MySQL) (*api.MySQL, error) {
	mysql, err := f.dbClient.KubedbV1alpha1().MySQLs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	mysql, _, err = util.PatchMySQL(context.TODO(), f.dbClient.KubedbV1alpha1(), mysql, transform, metav1.PatchOptions{})
	return mysql, err
}

func (f *Framework) EventuallyMySQLPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DatabasePhase {
			db, err := f.dbClient.KubedbV1alpha1().MySQLs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		Timeout,
		RetryInterval,
	)
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

func (f *Framework) EventuallyMySQL(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.dbClient.KubedbV1alpha1().MySQLs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				}
				Expect(err).NotTo(HaveOccurred())
			}
			return true
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

	return my, err
}

func (fi *Invocation) EventuallyDBReady(my *api.MySQL, dbInfo DatabaseConnectionInfo) {
	if my.Spec.TLS == nil {
		for i := int32(0); i < *my.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", my.Name, i))
			dbInfo.ClientPodIndex = int(i)
			fi.EventuallyDBConnection(my.ObjectMeta, dbInfo).Should(BeTrue())
		}

		if my.Spec.Topology != nil && my.Spec.Topology.Mode != nil {
			for i := int32(0); i < *my.Spec.Replicas; i++ {
				By(fmt.Sprintf("Checking ONLINE member count from Pod '%s-%d'", my.Name, i))
				dbInfo.ClientPodIndex = int(i)
				fi.EventuallyONLINEMembersCount(my.ObjectMeta, dbInfo).Should(Equal(int(*my.Spec.Replicas)))
			}
		}
	}

	if my.Spec.TLS != nil {
		requireSecureTransport := func(requireSSL bool) string {
			if requireSSL {
				return RequiredSecureTransportON
			} else {
				return RequiredSecureTransportOFF
			}
		}(my.Spec.RequireSSL)

		fi.EventuallyCheckConnectionRootUser(my, requireSecureTransport, dbInfo)

		By("Checking MySQL SSL server settings")
		sslConfigVar := []string{
			// get requireSecureTransport
			fmt.Sprintf("require_secure_transport=%s", requireSecureTransport),
			"have_ssl=YES",
			"have_openssl=YES",
			// in MySQL, certs are stored in "/etc/mysql/certs" path
			"ssl_ca=/etc/mysql/certs/ca.crt",
			"ssl_cert=/etc/mysql/certs/server.crt",
			"ssl_key=/etc/mysql/certs/server.key",
		}

		for _, cfg := range sslConfigVar {
			dbInfo.Param = fmt.Sprintf("tls=%s", TLSCustomConfig)
			fi.EventuallyCheckSSLSettings(my.ObjectMeta, dbInfo, cfg).Should(matcher.HaveSSL(cfg))
		}
	}
}

func (f *Framework) EventuallyCheckConnectionRootUser(my *api.MySQL, requireSecureTransport string, dbInfo DatabaseConnectionInfo) {
	params := []string{
		fmt.Sprintf("tls=%s", TLSSkibVerify),
		fmt.Sprintf("tls=%s", TLSCustomConfig),
	}
	if requireSecureTransport == RequiredSecureTransportOFF {
		params = append(params, fmt.Sprintf("tls=%s", TLSFalse))
	}

	for _, param := range params {
		dbInfo.Param = param
		By(fmt.Sprintf("Checking root User connection with tls: %s", param))
		for i := int32(0); i < *my.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", my.Name, i))
			dbInfo.ClientPodIndex = int(i)
			f.EventuallyDBConnection(my.ObjectMeta, dbInfo).Should(BeTrue())
		}

		if my.Spec.Topology != nil && my.Spec.Topology.Mode != nil {
			for i := int32(0); i < *my.Spec.Replicas; i++ {
				By(fmt.Sprintf("Checking ONLINE member count from Pod '%s-%d'", my.Name, i))
				dbInfo.ClientPodIndex = int(i)
				f.EventuallyONLINEMembersCount(my.ObjectMeta, dbInfo).Should(Equal(int(*my.Spec.Replicas)))
			}
		}
	}
}

func (f *Framework) EventuallyCheckConnectionRequiredSSLUser(my *api.MySQL, dbInfo DatabaseConnectionInfo) {
	params := []string{
		fmt.Sprintf("tls=%s", TLSSkibVerify),
		fmt.Sprintf("tls=%s", TLSCustomConfig),
	}
	for _, param := range params {
		dbInfo.Param = param
		By(fmt.Sprintf("Checking ssl required User connection with tls: %s", param))
		for i := int32(0); i < *my.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", my.Name, i))
			dbInfo.ClientPodIndex = int(i)
			f.EventuallyDBConnection(my.ObjectMeta, dbInfo).Should(BeTrue())
		}

		if my.Spec.Topology != nil && my.Spec.Topology.Mode != nil {
			dbInfo.User = MySQLRootUser
			for i := int32(0); i < *my.Spec.Replicas; i++ {
				By(fmt.Sprintf("Checking ONLINE member count from Pod '%s-%d'", my.Name, i))
				dbInfo.ClientPodIndex = int(i)
				f.EventuallyONLINEMembersCount(my.ObjectMeta, dbInfo).Should(Equal(int(*my.Spec.Replicas)))
			}
		}
	}
}
