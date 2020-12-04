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

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha2/util"
	"kubedb.dev/tests/e2e/matcher"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
	meta_util "kmodules.xyz/client-go/meta"
	ofst "kmodules.xyz/offshoot-api/api/v1"
)

var (
	DBDefaultCPUSize    = "500m"
	DBDefaultMemorySize = "1024Mi"
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
			PodTemplate: ofst.PodTemplateSpec{
				Spec: ofst.PodSpec{
					Resources: core.ResourceRequirements{
						Limits: core.ResourceList{
							core.ResourceCPU: resource.MustParse(DBDefaultCPUSize),
						},
						Requests: core.ResourceList{
							core.ResourceMemory: resource.MustParse(DBDefaultMemorySize),
						},
					},
				},
			},
		},
	}
}

func AddTLSConfig(issuerMeta metav1.ObjectMeta) func(in *api.MySQL) {
	return func(in *api.MySQL) {
		in.Spec.RequireSSL = true
		in.Spec.TLS = &kmapi.TLSConfig{
			IssuerRef: &core.TypedLocalObjectReference{
				Name:     issuerMeta.Name,
				Kind:     "Issuer",
				APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
			},
			Certificates: []kmapi.CertificateSpec{
				{
					Alias: string(api.MySQLServerCert),
					Subject: &kmapi.X509Subject{
						Organizations: []string{
							"kubedb:server",
						},
					},
					DNSNames: []string{
						"localhost",
					},
					IPAddresses: []string{
						"127.0.0.1",
					},
				},
			},
		}
	}
}

func (fi *Invocation) CreateMySQL(obj *api.MySQL) (*api.MySQL, error) {
	return fi.dbClient.KubedbV1alpha2().MySQLs(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func (fi *Invocation) GetMySQL(meta metav1.ObjectMeta) (*api.MySQL, error) {
	return fi.dbClient.KubedbV1alpha2().MySQLs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
}

func (fi *Invocation) PatchMySQL(meta metav1.ObjectMeta, transform func(*api.MySQL) *api.MySQL) (*api.MySQL, error) {
	mysql, err := fi.dbClient.KubedbV1alpha2().MySQLs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	mysql, _, err = util.PatchMySQL(context.TODO(), fi.dbClient.KubedbV1alpha2(), mysql, transform, metav1.PatchOptions{})
	return mysql, err
}

func (fi *Invocation) EventuallyMySQLPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DatabasePhase {
			db, err := fi.dbClient.KubedbV1alpha2().MySQLs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		Timeout,
		RetryInterval,
	)
}

func (fi *Invocation) EventuallyMySQLReady(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			my, err := fi.dbClient.KubedbV1alpha2().MySQLs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return my.Status.Phase == api.DatabasePhaseReady
		},
		Timeout,
		RetryInterval,
	)
}

func (fi *Invocation) EventuallyMySQL(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := fi.dbClient.KubedbV1alpha2().MySQLs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
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

func (fi *Invocation) DeleteMySQL(meta metav1.ObjectMeta) error {
	return fi.dbClient.KubedbV1alpha2().MySQLs(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
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
	fi.EventuallyMySQLReady(my.ObjectMeta).Should(BeTrue())

	By("Wait for AppBinding to create")
	fi.EventuallyAppBinding(my.ObjectMeta).Should(BeTrue())

	By("Check valid AppBinding Specs")
	err = fi.CheckMySQLAppBindingSpec(my.ObjectMeta)

	return my, err
}

func (fi *Invocation) EventuallyDBReady(my *api.MySQL, dbInfo DatabaseConnectionInfo) {
	if my.Spec.TLS == nil {
		By(fmt.Sprintf("Waiting for database to be ready for db %s", my.Name))
		fi.EventuallyDBConnection(my.ObjectMeta, dbInfo).Should(BeTrue())

		if my.Spec.Topology != nil && my.Spec.Topology.Mode != nil {
			By(fmt.Sprintf("Checking ONLINE member count from db %s", my.Name))
			fi.EventuallyONLINEMembersCount(my.ObjectMeta, dbInfo).Should(Equal(int(*my.Spec.Replicas)))
		}
	} else {
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

func (fi *Invocation) EventuallyCheckConnectionRootUser(my *api.MySQL, requireSecureTransport string, dbInfo DatabaseConnectionInfo) {
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
		By(fmt.Sprintf("Waiting for database to be ready for pod %s", my.Name))
		fi.EventuallyDBConnection(my.ObjectMeta, dbInfo).Should(BeTrue())

		if my.Spec.Topology != nil && my.Spec.Topology.Mode != nil {
			By(fmt.Sprintf("Checking ONLINE member count from Pod %s", my.Name))
			fi.EventuallyONLINEMembersCount(my.ObjectMeta, dbInfo).Should(Equal(int(*my.Spec.Replicas)))
		}
	}
}

func (fi *Invocation) EventuallyCheckConnectionRequiredSSLUser(my *api.MySQL, dbInfo DatabaseConnectionInfo) {
	params := []string{
		fmt.Sprintf("tls=%s", TLSSkibVerify),
		fmt.Sprintf("tls=%s", TLSCustomConfig),
	}
	for _, param := range params {
		dbInfo.Param = param
		By(fmt.Sprintf("Checking ssl required User connection with tls: %s", param))
		By(fmt.Sprintf("Waiting for database to be ready for pod %s", my.Name))
		fi.EventuallyDBConnection(my.ObjectMeta, dbInfo).Should(BeTrue())

		if my.Spec.Topology != nil && my.Spec.Topology.Mode != nil {
			dbInfo.User = MySQLRootUser
			By(fmt.Sprintf("Checking ONLINE member count from Pod %s", my.Name))
			fi.EventuallyONLINEMembersCount(my.ObjectMeta, dbInfo).Should(Equal(int(*my.Spec.Replicas)))
		}
	}
}

func (fi *Invocation) PopulateMySQL(dbMeta metav1.ObjectMeta, dbInfo DatabaseConnectionInfo) {
	By("Creating Table")
	fi.EventuallyCreateTable(dbMeta, dbInfo).Should(BeTrue())

	By("Inserting Rows")
	fi.EventuallyInsertRow(dbMeta, dbInfo, 3).Should(BeTrue())

	By("Checking Row Count of Table")
	fi.EventuallyCountRow(dbMeta, dbInfo).Should(Equal(3))
}
