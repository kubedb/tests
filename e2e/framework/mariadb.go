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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

func (fi *Invocation) MariaDBDefinition(version string) *api.MariaDB {
	return &api.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mariadb"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: api.MariaDBSpec{
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

func (fi *Invocation) CreateMariaDB(obj *api.MariaDB) (*api.MariaDB, error) {
	return fi.dbClient.KubedbV1alpha2().MariaDBs(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func (fi *Invocation) GetMariaDB(meta metav1.ObjectMeta) (*api.MariaDB, error) {
	return fi.dbClient.KubedbV1alpha2().MariaDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
}

func (fi *Invocation) PatchMariaDB(meta metav1.ObjectMeta, transform func(*api.MariaDB) *api.MariaDB) (*api.MariaDB, error) {
	mariadb, err := fi.dbClient.KubedbV1alpha2().MariaDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	mariadb, _, err = util.PatchMariaDB(context.TODO(), fi.dbClient.KubedbV1alpha2(), mariadb, transform, metav1.PatchOptions{})
	return mariadb, err
}

func (fi *Invocation) EventuallyMariaDBPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DatabasePhase {
			db, err := fi.dbClient.KubedbV1alpha2().MariaDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		Timeout,
		RetryInterval,
	)
}

func (fi *Invocation) EventuallyMariaDBReady(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			md, err := fi.dbClient.KubedbV1alpha2().MariaDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return md.Status.Phase == api.DatabasePhaseReady
		},
		Timeout,
		RetryInterval,
	)
}

// Don't call this if the db is already created(except mysql)
// Don't call this more than once for any db
func (fi *Invocation) PopulateMariaDB(md *api.MariaDB, dbInfo MariaDBInfo) {

	if dbInfo.DatabaseName != DBMySQL {
		// mysql db is created by default
		By("Creating test Database")
		fi.EventuallyCreateTestDBMD(md.ObjectMeta, dbInfo).Should(BeTrue())
	}

	By("Checking if test Database exist")
	fi.EventuallyExistsDBMD(md.ObjectMeta, dbInfo).Should(BeTrue())

	By("Creating Table")
	fi.EventuallyCreateTableMD(md.ObjectMeta, dbInfo).Should(BeTrue())

	By("Inserting Rows")
	fi.EventuallyInsertRowMD(md.ObjectMeta, dbInfo, 3).Should(BeTrue())

	By("Checking Row Count of Table")
	fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
}

func (fi *Invocation) EnableSSLMariaDB(md *api.MariaDB, requiredSSL bool, transformFuncs ...func(in *api.MariaDB)) {
	// Create Issuer
	issuer, err := fi.EnsureIssuer(md.ObjectMeta, api.ResourceKindMariaDB)
	Expect(err).NotTo(HaveOccurred())

	// Enable SSL in MariaDB
	md.Spec.RequireSSL = true
	md.Spec.TLS = NewTLSConfiguration(issuer)
	for _, fn := range transformFuncs {
		fn(md)
	}
}

func (f *Framework) CleanMariaDB() {
	mariadbList, err := f.dbClient.KubedbV1alpha2().MariaDBs(f.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range mariadbList.Items {
		if _, _, err := util.PatchMariaDB(context.TODO(), f.dbClient.KubedbV1alpha2(), &e, func(in *api.MariaDB) *api.MariaDB {
			in.ObjectMeta.Finalizers = nil
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		}, metav1.PatchOptions{}); err != nil {
			fmt.Printf("error Patching MariaDB. error: %v", err)
		}
	}
	if err := f.dbClient.KubedbV1alpha2().MariaDBs(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInForeground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of MariaDB. Error: %v", err)
	}
}

func (fi *Invocation) EventuallyMariaDB(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := fi.dbClient.KubedbV1alpha2().MariaDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
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

func (fi *Invocation) DeleteMariaDB(meta metav1.ObjectMeta) error {
	return fi.dbClient.KubedbV1alpha2().MariaDBs(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}

//// SimulateMariaDBDisaster simulates accidental database deletion. It drops the database specified by "dbName" variable.
//func (fi *Invocation) SimulateMariaDBDisaster(meta metav1.ObjectMeta, dbInfo MariaDBInfo) {
//	// Dropping Database
//	By("Deleting database: " + dbInfo.DatabaseName)
//	fi.EventuallyDropDatabaseMD(meta, dbInfo).Should(BeTrue())
//	By("Verifying that database: " +  dbInfo.DatabaseName + " has been removed")
//	dbExist, err := fi.DatabaseExistsMD(meta, dbInfo)
//	Expect(err).NotTo(HaveOccurred())
//	Expect(dbExist).To(BeFalse())
//}

func (fi *Invocation) CreateMariaDBAndWaitForRunning(version string, transformFuncs ...func(in *api.MariaDB)) (*api.MariaDB, error) {
	// Generate MariaDB definition
	md := fi.MariaDBDefinition(version)
	// transformFunc provide a function that made test specific change on the MariaDB
	// apply these test specific changes
	for _, fn := range transformFuncs {
		fn(md)
	}

	By("Create MariaDB: " + md.Namespace + "/" + md.Name)
	md, err := fi.CreateMariaDB(md)
	if err != nil {
		return md, err
	}
	fi.AppendToCleanupList(md)

	By("Wait for Running mariadb")
	fi.EventuallyMariaDBReady(md.ObjectMeta).Should(BeTrue())

	By("Wait for AppBinding to create")
	fi.EventuallyAppBinding(md.ObjectMeta).Should(BeTrue())

	By("Check valid AppBinding Specs")
	err = fi.CheckMariaDBAppBindingSpec(md.ObjectMeta)

	return md, err
}

func (fi *Invocation) EventuallyDBReadyMD(md *api.MariaDB, dbInfo MariaDBInfo) {
	if md.Spec.TLS == nil {
		for i := int32(0); i < *md.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", md.Name, i))
			fi.EventuallyDBConnectionMD(md.ObjectMeta, dbInfo).Should(BeTrue())
		}
	} else {
		requireSecureTransport := func(requireSSL bool) string {
			if requireSSL {
				return RequiredSecureTransportON
			} else {
				return RequiredSecureTransportOFF
			}
		}(md.Spec.RequireSSL)

		fi.EventuallyCheckConnectionRootUserMD(md, requireSecureTransport, dbInfo)

		By("Checking MariaDB SSL server settings")
		sslConfigVar := []string{
			// get requireSecureTransport
			fmt.Sprintf("require_secure_transport=%s", requireSecureTransport),
			"have_ssl=YES",
			"have_openssl=YES",
			// in MariaDB, certs are stored in "/etc/mysql/certs/server" path
			"ssl_ca=/etc/mysql/certs/server/ca.crt",
			"ssl_cert=/etc/mysql/certs/server/tls.crt",
			"ssl_key=/etc/mysql/certs/server/tls.key",
		}

		for _, cfg := range sslConfigVar {
			dbInfo.Param = fmt.Sprintf("tls=%s", TLSCustomConfig)
			fi.EventuallyCheckSSLSettingsMD(md.ObjectMeta, dbInfo, cfg).Should(matcher.HaveSSL(cfg))
		}
	}
}

func (fi *Invocation) EventuallyCheckConnectionRootUserMD(md *api.MariaDB, requireSecureTransport string, dbInfo MariaDBInfo) {
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
		for i := int32(0); i < *md.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", md.Name, i))
			fi.EventuallyDBConnectionMD(md.ObjectMeta, dbInfo).Should(BeTrue())
		}
	}
}

func (fi *Invocation) EventuallyCheckConnectionRequiredSSLUserMD(md *api.MariaDB, dbInfo MariaDBInfo) {
	params := []string{
		fmt.Sprintf("tls=%s", TLSSkibVerify),
		fmt.Sprintf("tls=%s", TLSCustomConfig),
	}
	for _, param := range params {
		dbInfo.Param = param
		By(fmt.Sprintf("Checking ssl required User connection with tls: %s", param))
		for i := int32(0); i < *md.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", md.Name, i))
			fi.EventuallyDBConnectionMD(md.ObjectMeta, dbInfo).Should(BeTrue())
		}
	}
}

func SslEnabledMariaDB(md *api.MariaDB) bool {
	return md.Spec.TLS != nil
}
