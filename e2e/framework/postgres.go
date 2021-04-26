
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
	"gomodules.xyz/pointer"
	policy "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"kubedb.dev/apimachinery/apis/kubedb"
	"strconv"
	"time"

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

func (fi *Invocation) PostgresDefinition(version string) *api.Postgres {
	return &api.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(api.ResourceSingularPostgres),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: api.PostgresSpec{
			Version:           version,
			Replicas: pointer.Int32P(1),
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
					},
				},
				StorageClassName: pointer.StringP(fi.StorageClass),
			},
			TerminationPolicy: api.TerminationPolicyHalt,
		},
	}
}

func (fi *Invocation) CreatePostgres(obj *api.Postgres) (*api.Postgres, error) {
	return fi.dbClient.KubedbV1alpha2().Postgreses(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func (fi *Invocation) GetPostgres(meta metav1.ObjectMeta) (*api.Postgres, error) {
	return fi.dbClient.KubedbV1alpha2().Postgreses(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
}

func (fi *Invocation) PatchPostgres(meta metav1.ObjectMeta, transform func(postgres *api.Postgres) *api.Postgres) (*api.Postgres, error) {
	postgres, err := fi.dbClient.KubedbV1alpha2().Postgreses(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	postgres, _, err = util.PatchPostgres(context.TODO(), fi.dbClient.KubedbV1alpha2(), postgres, transform, metav1.PatchOptions{})
	return postgres, err
}

func (fi *Invocation) DeletePostgres(meta metav1.ObjectMeta) error {
	return fi.dbClient.KubedbV1alpha2().Postgreses(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}
func (fi *Invocation) EventuallyPostgres(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := fi.dbClient.KubedbV1alpha2().Postgreses(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
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

func (fi *Invocation) EventuallyPostgresPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DatabasePhase {
			db, err := fi.dbClient.KubedbV1alpha2().Postgreses(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		Timeout,
		RetryInterval,
	)
}
func (f *Invocation) EventuallyPostgresPodCount(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() int32 {
			st, err := f.kubeClient.AppsV1beta1().StatefulSets(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return -1
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			return st.Status.ReadyReplicas
		},
		time.Minute*5,
		time.Second*5,
	)
}
func (fi *Invocation) EventuallyPostgresReady(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			pg, err := fi.dbClient.KubedbV1alpha2().Postgreses(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return pg.Status.Phase == api.DatabasePhaseReady
		},
		Timeout,
		RetryInterval,
	)
}

func (f *Invocation) CleanPostgres() {
	postgresList, err := f.dbClient.KubedbV1alpha2().Postgreses(f.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range postgresList.Items {
		if _, _, err := util.PatchPostgres(context.TODO(), f.dbClient.KubedbV1alpha2(), &e, func(in *api.Postgres) *api.Postgres {
			in.ObjectMeta.Finalizers = nil
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		}, metav1.PatchOptions{}); err != nil {
			fmt.Printf("error Patching Postgres. error: %v", err)
		}
	}
	if err := f.dbClient.KubedbV1alpha2().Postgreses(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInForeground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of Postgres. Error: %v", err)
	}
}



func (f *Invocation) EvictPodsFromStatefulSetPostgres(meta metav1.ObjectMeta) error {
	var err error
	labelSelector := labels.Set{
		meta_util.NameLabelKey:      api.Postgres{}.ResourceFQN(),
		meta_util.InstanceLabelKey:  meta.Name,
		meta_util.ManagedByLabelKey: kubedb.GroupName,
	}
	// get sts in the namespace
	stsList, err := f.kubeClient.AppsV1().StatefulSets(meta.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return err
	}
	for _, sts := range stsList.Items {
		// if PDB is not found, send error
		var pdb *policy.PodDisruptionBudget
		pdb, err = f.kubeClient.PolicyV1beta1().PodDisruptionBudgets(sts.Namespace).Get(context.TODO(), sts.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		eviction := &policy.Eviction{
			TypeMeta: metav1.TypeMeta{
				APIVersion: policy.SchemeGroupVersion.String(),
				Kind:       kindEviction,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      sts.Name,
				Namespace: sts.Namespace,
			},
			DeleteOptions: &metav1.DeleteOptions{},
		}

		if pdb.Spec.MaxUnavailable == nil {
			return fmt.Errorf("found pdb %s spec.maxUnavailable nil", pdb.Name)
		}

		// try to evict as many pod as allowed in pdb. No err should occur
		maxUnavailable := pdb.Spec.MaxUnavailable.IntValue()
		for i := 0; i < maxUnavailable; i++ {
			eviction.Name = sts.Name + "-" + strconv.Itoa(i)

			err := f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(context.TODO(), eviction)
			if err != nil {
				return err
			}
		}

		// try to evict one extra pod. TooManyRequests err should occur
		eviction.Name = sts.Name + "-" + strconv.Itoa(maxUnavailable)
		err = f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(context.TODO(), eviction)
		if kerr.IsTooManyRequests(err) {
			err = nil
		} else if err != nil {
			return err
		} else {
			return fmt.Errorf("expected pod %s/%s to be not evicted due to pdb %s", sts.Namespace, eviction.Name, pdb.Name)
		}
	}
	return err
}


// emon TODO: need to change
// Don't call this if the db is already created
// Don't call this more than once for any db
func (fi *Invocation) PopulatePostgres(pg *api.Postgres, dbInfo PostgresInfo) {

	if dbInfo.DatabaseName != api.PostgresDefaultUsername {
		// mysql db is created by default
		By("Creating test Database")
		fi.EventuallyCreateTestDBPostgres(pg.ObjectMeta, dbInfo).Should(BeTrue())
	}

	By("Checking if test Database exist")
	fi.EventuallyExistsDBPostgres(pg.ObjectMeta, dbInfo).Should(Equal(1))

	By("Creating Table")
	fi.EventuallyCreateTablePostgres(pg, dbInfo.DatabaseName,dbInfo.User,3).Should(BeTrue())

	By("Checking Table")
	fi.EventuallyCountTablePostgres(pg, dbInfo.DatabaseName, dbInfo.User).Should(Equal(3))

	By("Halt Postgres: Update postgres to set spec.halted = true")
	_, err := fi.PatchPostgres(pg.ObjectMeta, func(in *api.Postgres) *api.Postgres {
		in.Spec.Halted = true
		return in
	})
	Expect(err).NotTo(HaveOccurred())

	By("Wait for halted postgres")
	fi.EventuallyPostgresPhase(pg.ObjectMeta).Should(Equal(api.DatabasePhaseHalted))

	By("Resume Postgres: Update postgres to set spec.halted = false")
	_, err = fi.PatchPostgres(pg.ObjectMeta, func(in *api.Postgres) *api.Postgres {
		in.Spec.Halted = false
		return in
	})
	Expect(err).NotTo(HaveOccurred())

	By("Wait for Running postgres")
	fi.EventuallyPostgresReady(pg.ObjectMeta).Should(BeTrue())

	By("Checking Table")
	fi.EventuallyCountTablePostgres(pg, dbInfo.DatabaseName, dbInfo.User).Should(Equal(3))
}

func (fi *Invocation) EnableSSLPostgres(pg *api.Postgres, transformFuncs ...func(in *api.Postgres)) {
	// Create Issuer
	issuer, err := fi.EnsureIssuer(pg.ObjectMeta, api.ResourceKindPostgres)
	Expect(err).NotTo(HaveOccurred())


	pg.Spec.TLS = NewTLSConfiguration(issuer)
	for _, fn := range transformFuncs {
		fn(pg)
	}
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

func (fi *Invocation) CreatePostgresAndWaitForRunning(version string, transformFuncs ...func(in *api.Postgres)) (*api.Postgres, error) {
	// Generate Postgres definition
	pg := fi.PostgresDefinition(version)
	// transformFunc provide a function that made test specific change on the MariaDB
	// apply these test specific changes
	for _, fn := range transformFuncs {
		fn(pg)
	}

	By("Create Postgres: " + pg.Namespace + "/" + pg.Name)
	pg, err := fi.CreatePostgres(pg)
	if err != nil {
		return pg, err
	}
	fi.AppendToCleanupList(pg)

	By("Wait for Running postgres")
	fi.EventuallyPostgresReady(pg.ObjectMeta).Should(BeTrue())

	By("Wait for AppBinding to create")
	fi.EventuallyAppBinding(pg.ObjectMeta).Should(BeTrue())

	By("Check valid AppBinding Specs")
	err = fi.CheckMariaDBAppBindingSpec(pg.ObjectMeta)

	return pg, err
}

func (fi *Invocation) EventuallyDBReadyPostgres(pg *api.Postgres, dbInfo PostgresInfo) {
	if pg.Spec.TLS == nil {
		for i := int32(0); i < *pg.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", pg.Name, i))
			fi.EventuallyDBConnectionPostgres(pg.ObjectMeta, dbInfo).Should(BeTrue())
		}
	} else {
		requireSecureTransport := func(requireSSL bool) string {
			if requireSSL {
				return RequiredSecureTransportON
			} else {
				return RequiredSecureTransportOFF
			}
		}(pg.Spec.RequireSSL)

		fi.EventuallyCheckConnectionRootUserPostgres(pg, requireSecureTransport, dbInfo)

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
			fi.EventuallyCheckSSLSettingsPostgres(pg.ObjectMeta, dbInfo, cfg).Should(matcher.HaveSSL(cfg))
		}
	}
}

func (fi *Invocation) EventuallyCheckConnectionRootUserPostgres(pg *api.Postgres, requireSecureTransport string, dbInfo PostgresInfo) {
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
		for i := int32(0); i < *pg.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", pg.Name, i))
			fi.EventuallyDBConnectionPostgres(pg.ObjectMeta, dbInfo).Should(BeTrue())
		}
	}
}

func (fi *Invocation) EventuallyCheckConnectionRequiredSSLUserPostgres(pg *api.Postgres, dbInfo PostgresInfo) {
	params := []string{
		fmt.Sprintf("tls=%s", TLSSkibVerify),
		fmt.Sprintf("tls=%s", TLSCustomConfig),
	}
	for _, param := range params {
		dbInfo.Param = param
		By(fmt.Sprintf("Checking ssl required User connection with tls: %s", param))
		for i := int32(0); i < *pg.Spec.Replicas; i++ {
			By(fmt.Sprintf("Waiting for database to be ready for pod '%s-%d'", pg.Name, i))
			fi.EventuallyDBConnectionPostgres(pg.ObjectMeta, dbInfo).Should(BeTrue())
		}
	}
}

func SslEnabledPostgres(pg *api.Postgres) bool {
	return pg.Spec.TLS != nil
}
