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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MySQL", func() {

	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !runTestCommunity(framework.StorageType) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.StorageType))
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
		Context("StorageType ", func() {
			Context("Ephemeral", func() {
				It("General Behaviour", func() {
					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.StorageType = api.StorageTypeEphemeral
						in.Spec.Storage = nil
						// Set termination policy WipeOut to delete all mysql resources permanently
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
					fi.EventuallyDBReady(my, dbInfo)

					By("Creating Table")
					fi.EventuallyCreateTable(my.ObjectMeta, dbInfo).Should(BeTrue())

					By("Inserting Rows")
					fi.EventuallyInsertRow(my.ObjectMeta, dbInfo, 3).Should(BeTrue())

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
				})
			})
			Context("With TerminationPolicyHalt", func() {
				It("should reject to create MySQL object", func() {
					// Create MySQL standalone and wait for running
					_, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.StorageType = api.StorageTypeEphemeral
						in.Spec.Storage = nil
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
