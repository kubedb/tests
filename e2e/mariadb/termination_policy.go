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

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("MariaDB", func() {

	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMariaDB))
		}
		if !runTestCommunity(framework.TerminationPolicy) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.TerminationPolicy))
		}
	})

	JustAfterEach(func() {
		fi.PrintDebugInfoOnFailure()
	})

	AfterEach(func() {
		err := fi.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())

	})

	Describe("Test", func() {

		Context("Termination Policy", func() {

			Context("with TerminationPolicyDoNotTerminate", func() {

				It("should work successfully", func() {
					// MariaDB ObjectMeta
					mdMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mariadb"),
						Namespace: fi.Namespace(),
					}
					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						// Set termination policy DoNotTerminate to rejects attempt to delete database using ValidationWebhook
						in.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
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

					By("Fail to delete mariadb")
					err = fi.DeleteMariaDB(mdMeta)
					Expect(err).Should(HaveOccurred())

					By("Check for Running mariadb")
					fi.EventuallyMariaDBReady(mdMeta).Should(BeTrue())

					By("Update mariadb to set spec.terminationPolicy = WipeOut")
					_, err = fi.PatchMariaDB(mdMeta, func(in *api.MariaDB) *api.MariaDB {
						// Set termination policy WipeOut to delete all mariadb resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
						return in
					})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("with TerminationPolicyHalt", func() {

				It("should run successfully", func() {
					// MariaDB ObjectMeta
					mdMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mariadb"),
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

					By("Halt MariaDB: Update mariadb to set spec.halted = true")
					_, err = fi.PatchMariaDB(mdMeta, func(in *api.MariaDB) *api.MariaDB {
						in.Spec.Halted = true
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Wait for halted mariadb")
					fi.EventuallyMariaDBPhase(mdMeta).Should(Equal(api.DatabasePhaseHalted))

					By("Resume MariaDB: Update mariadb to set spec.halted = false")
					_, err = fi.PatchMariaDB(mdMeta, func(in *api.MariaDB) *api.MariaDB {
						in.Spec.Halted = false
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Wait for Running mariadb")
					fi.EventuallyMariaDBReady(mdMeta).Should(BeTrue())

					By("Deleting MariaDB crd")
					err = fi.DeleteMariaDB(mdMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mariadb to be deleted")
					fi.EventuallyMariaDB(mdMeta).Should(BeFalse())

					By("Checking PVC hasn't been deleted")
					fi.EventuallyPVCCount(mdMeta, api.MariaDB{}.ResourceFQN()).Should(Equal(1))

					By("Checking Secret hasn't been deleted")
					fi.EventuallyDBSecretCount(mdMeta, api.MariaDB{}.ResourceFQN()).Should(Equal(1))

					// Create MariaDB object again to resume it
					_, err = fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						// Set termination policy WipeOut to delete all mariadb resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())

					By("Checking row count of table")
					fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))
				})
			})

			Context("with TerminationPolicyDelete", func() {

				It("should run successfully", func() {
					// MariaDB ObjectMeta
					mdMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mariadb"),
						Namespace: fi.Namespace(),
					}
					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						// Set termination policy Delete to deletes database pods, service, pvcs but leave the mariadb auth secret.
						in.Spec.TerminationPolicy = api.TerminationPolicyDelete
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

					By("Delete mariadb")
					err = fi.DeleteMariaDB(mdMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mariadb to be deleted")
					fi.EventuallyMariaDB(mdMeta).Should(BeFalse())

					By("Checking PVC has been deleted")
					fi.EventuallyPVCCount(mdMeta, api.MariaDB{}.ResourceFQN()).Should(Equal(0))

					By("Checking Secret hasn't been deleted")
					fi.EventuallyDBSecretCount(mdMeta, api.MariaDB{}.ResourceFQN()).Should(Equal(1))

					// Need to delete existing MariaDB secret
					secretMeta := metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-%s", mdMeta.Name, "auth"),
						Namespace: mdMeta.Namespace,
					}
					err = fi.DeleteSecret(secretMeta)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("with TerminationPolicyWipeOut", func() {

				It("should not create database and should wipeOut all", func() {
					// MariaDB ObjectMeta
					mdMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mariadb"),
						Namespace: fi.Namespace(),
					}
					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						// Set termination policy WipeOut to delete all mariadb resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
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

					By("Delete mariadb")
					err = fi.DeleteMariaDB(mdMeta)
					Expect(err).NotTo(HaveOccurred())

					By("wait until mariadb is deleted")
					fi.EventuallyMariaDB(mdMeta).Should(BeFalse())

					By("Checking PVCs has been deleted")
					fi.EventuallyPVCCount(mdMeta, api.MariaDB{}.ResourceFQN()).Should(Equal(0))

					By("Checking Secrets has been deleted")
					fi.EventuallyDBSecretCount(mdMeta, api.MariaDB{}.ResourceFQN()).Should(Equal(0))
				})
			})
		})

	})
})
