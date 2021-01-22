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

package mariadb

import (
	"fmt"
	//"github.com/davecgh/go-spew/spew"
 	"encoding/json"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/log"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

var _ = Describe("MariaDB", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()
		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMariaDB))
		}
		if !runTestCommunity(framework.General) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.General))
		}
	})

	JustAfterEach(func() {
		fi.PrintDebugInfoOnFailure()
	})

	AfterEach(func() {
		err := fi.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())

	})

	Context("General", func() {

		Context("-", func() {
			It("should run successfully", func() {
				// MariaDB ObjectMeta
				mdMeta := metav1.ObjectMeta{
					Name:      rand.WithUniqSuffix("md"),
					Namespace: fi.Namespace(),
				}
				// Create MariaDB standalone and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Name = mdMeta.Name
					in.Namespace = mdMeta.Namespace
					// Set termination policy Halt to leave the PVCs and secrets intact for reuse
					in.Spec.TerminationPolicy = api.TerminationPolicyHalt
				})
				Expect(err).NotTo(HaveOccurred())
				// Database connection information
				dbInfo := framework.DatabaseConnectionInfo{
					StatefulSetOrdinal: 0,
					ClientPodIndex:     0,
					DatabaseName:       framework.DBMySQL,
					User:               framework.MySQLRootUser,
					Param:              "",
				}
				fi.EventuallyDBReadyMD(md, dbInfo)

				By("Creating Table")
				fi.EventuallyCreateTableMD(mdMeta, dbInfo).Should(BeTrue())

				By("Inserting Rows")
				fi.EventuallyInsertRowMD(mdMeta, dbInfo, 3).Should(BeTrue())

				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))

				By("Delete mariadb: " + mdMeta.Namespace + "/" + mdMeta.Name)
				err = fi.DeleteMariaDB(mdMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Wait for mariadb to be deleted")
				fi.EventuallyMariaDB(mdMeta).Should(BeFalse())

				// Create MariaDB object again to resume it
				md, err = fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Name = mdMeta.Name
					in.Namespace = mdMeta.Namespace
					// Set termination policy WipeOut to delete all mysql resources permanently
					in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				})
				Expect(err).NotTo(HaveOccurred())
				fi.EventuallyDBReadyMD(md, dbInfo)

				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))
			})
		})

		Context("with custom SA Name", func() {
			var ensureCustomSecret = func(objMeta metav1.ObjectMeta) (*core.Secret, error) {
				By("Create Custom secret for MariaDB")
				secret := fi.GetAuthSecret(objMeta, false)
				secret, err := fi.CreateSecret(secret)
				if err != nil {
					return nil, err
				}
				fi.AppendToCleanupList(secret)
				return secret, err
			}

			It("should start and resume successfully", func() {
				// MariaDB objectMeta
				mdMeta := metav1.ObjectMeta{
					Name:      rand.WithUniqSuffix("md"),
					Namespace: fi.Namespace(),
				}
				// Create custom Secret for MariaDB
				customSecret, err := ensureCustomSecret(mdMeta)
				Expect(err).NotTo(HaveOccurred())
				// Create MariaDB standalone and wait for running
				_, err = fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Name = mdMeta.Name
					in.Namespace = mdMeta.Namespace
					in.Spec.AuthSecret = &core.LocalObjectReference{
						Name: customSecret.Name,
					}
					in.Spec.PodTemplate.Spec.ServiceAccountName = "md-custom-sa"
					// Set termination policy Halt to leave the PVCs and secrets intact for reuse
					in.Spec.TerminationPolicy = api.TerminationPolicyHalt
				})
				Expect(err).NotTo(HaveOccurred())

				By("Check if MariaDB " + mdMeta.Namespace + "/" + mdMeta.Name + " exists.")
				_, err = fi.GetMariaDB(mdMeta)
				if err != nil {
					if kerr.IsNotFound(err) {
						// MariaDB was not created. Hence, rest of cleanup is not necessary.
						return
					}
					Expect(err).NotTo(HaveOccurred())
				}

				By("Delete mariadb: " + mdMeta.Namespace + "/" + mdMeta.Name)
				err = fi.DeleteMariaDB(mdMeta)
				if err != nil {
					if kerr.IsNotFound(err) {
						// MariaDB was not created. Hence, rest of cleanup is not necessary.
						log.Infof("Skipping rest of cleanup. Reason: MariaDB %s/%s is not found.", mdMeta.Namespace, mdMeta.Name)
						return
					}
					Expect(err).NotTo(HaveOccurred())
				}

				By("Wait for mariadb to be deleted " + mdMeta.Namespace + "/" + mdMeta.Name)
				fi.EventuallyMariaDB(mdMeta).Should(BeFalse())

				// Create MariaDB object again to resume it
				_, err = fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Name = mdMeta.Name
					in.Namespace = mdMeta.Namespace
					in.Spec.AuthSecret = &core.LocalObjectReference{
						Name: customSecret.Name,
					}
					// Set termination policy WipeOut to delete all mysql resources permanently
					in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("PDB", func() {
			It("should run eviction successfully", func() {
				// Create MariaDB standalone and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Spec.Replicas = types.Int32P(3)
					// Set termination policy WipeOut to delete all mysql resources permanently
					in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				})
				Expect(err).NotTo(HaveOccurred())

				//Evict MariaDB pods
				By("Try to evict pods")
				err = fi.EvictPodsFromStatefulSet(md.ObjectMeta, api.MariaDB{}.ResourceFQN())
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
