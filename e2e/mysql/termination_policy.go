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

package mysql

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("MySQL", func() {

	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !RunTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !RunTestCommunity(framework.TerminationPolicy) {
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
					// MySQL ObjectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
						// Set termination policy DoNotTerminate to rejects attempt to delete database using ValidationWebhook
						in.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        "",
					}
					fi.EventuallyDBReady(my, dbInfo)
					fi.PopulateMySQL(my.ObjectMeta, dbInfo)

					By("Fail to delete mysql")
					err = fi.DeleteMySQL(myMeta)
					Expect(err).Should(HaveOccurred())

					By("Check for Running mysql")
					fi.EventuallyMySQLReady(myMeta).Should(BeTrue())

					By("Update mysql to set spec.terminationPolicy = WipeOut")
					_, err = fi.PatchMySQL(myMeta, func(in *api.MySQL) *api.MySQL {
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
						return in
					})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("with TerminationPolicyHalt", func() {

				It("should run successfully", func() {
					// MySQL ObjectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        "",
					}
					fi.EventuallyDBReady(my, dbInfo)
					fi.PopulateMySQL(my.ObjectMeta, dbInfo)

					By("Halt MySQL: Update mysql to set spec.halted = true")
					_, err = fi.PatchMySQL(myMeta, func(in *api.MySQL) *api.MySQL {
						in.Spec.Halted = true
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Wait for halted mysql")
					fi.EventuallyMySQLPhase(myMeta).Should(Equal(api.DatabasePhaseHalted))

					By("Resume MySQL: Update mysql to set spec.halted = false")
					_, err = fi.PatchMySQL(myMeta, func(in *api.MySQL) *api.MySQL {
						in.Spec.Halted = false
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Wait for Running mysql")
					fi.EventuallyMySQLReady(myMeta).Should(BeTrue())

					By("Deleting MySQL crd")
					err = fi.DeleteMySQL(myMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mysql to be deleted")
					fi.EventuallyMySQL(myMeta).Should(BeFalse())

					By("Checking PVC hasn't been deleted")
					fi.EventuallyPVCCount(myMeta, api.MySQL{}.ResourceFQN()).Should(Equal(1))

					By("Checking Secret hasn't been deleted")
					fi.EventuallyDBSecretCount(myMeta, api.MySQL{}.ResourceFQN()).Should(Equal(1))

					// Create MySQL object again to resume it
					_, err = fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())

					By("Checking row count of table")
					fi.EventuallyCountRow(myMeta, dbInfo).Should(Equal(3))
				})
			})

			Context("with TerminationPolicyDelete", func() {

				It("should run successfully", func() {
					// MySQL ObjectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
						// Set termination policy Delete to deletes database pods, service, pvcs but leave the mysql auth secret.
						in.Spec.TerminationPolicy = api.TerminationPolicyDelete
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        "",
					}
					fi.EventuallyDBReady(my, dbInfo)
					fi.PopulateMySQL(my.ObjectMeta, dbInfo)

					By("Delete mysql")
					err = fi.DeleteMySQL(myMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mysql to be deleted")
					fi.EventuallyMySQL(myMeta).Should(BeFalse())

					By("Checking PVC has been deleted")
					fi.EventuallyPVCCount(myMeta, api.MySQL{}.ResourceFQN()).Should(Equal(0))

					By("Checking Secret hasn't been deleted")
					fi.EventuallyDBSecretCount(myMeta, api.MySQL{}.ResourceFQN()).Should(Equal(1))

					// Need to delete existing MySQL secret
					secretMeta := metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-%s", myMeta.Name, "auth"),
						Namespace: myMeta.Namespace,
					}
					err = fi.DeleteSecret(secretMeta)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("with TerminationPolicyWipeOut", func() {

				It("should not create database and should wipeOut all", func() {
					// MySQL ObjectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        "",
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Delete mysql")
					err = fi.DeleteMySQL(myMeta)
					Expect(err).NotTo(HaveOccurred())

					By("wait until mysql is deleted")
					fi.EventuallyMySQL(myMeta).Should(BeFalse())

					By("Checking PVCs has been deleted")
					fi.EventuallyPVCCount(myMeta, api.MySQL{}.ResourceFQN()).Should(Equal(0))

					By("Checking Secrets has been deleted")
					fi.EventuallyDBSecretCount(myMeta, api.MySQL{}.ResourceFQN()).Should(Equal(0))
				})
			})
		})

	})
})
