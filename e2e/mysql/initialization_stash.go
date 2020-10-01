/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Community License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md

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
	core "k8s.io/api/core/v1"
	stashV1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

var _ = Describe("MySQL", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !runTestCommunity(framework.Initialize) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Initialize))
		}
	})

	JustAfterEach(func() {
		fi.PrintDebugInfoOnFailure()
	})

	AfterEach(func() {
		err := fi.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())

	})

	Describe("General Test", func() {
		// To run this test,
		// 1st: Deploy stash latest operator
		// 2nd: create mysql related tasks and functions either
		// 		from `kubedb.dev/mysql/hack/dev/examples/stash01_config.yaml`
		//	 or	from helm chart in `stash.appscode.dev/mysql/chart/mysql-stash`
		Context("With Stash/Restic", func() {

			BeforeEach(func() {
				if !fi.FoundStashCRDs() {
					Skip("Skipping tests for stash integration. reason: stash operator is not running.")
				}
			})

			Context("From GCS backend", func() {

				It("should run successfully", func() {
					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
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

					// take schedule backup to GCS bucket
					repository, err := fi.BackupDataIntoGCSBucket(my.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Initialize mysql from stash backup")
					rs := fi.RestoreSession(my.ObjectMeta, repository)
					my, err = fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.DatabaseSecret = my.Spec.DatabaseSecret
						in.Spec.Init = &api.InitSpec{
							StashRestoreSession: &core.LocalObjectReference{
								Name: rs.Name,
							},
						}
					})

					By("Create RestoreSession")
					rs, err = fi.CreateRestoreSession(rs)
					fi.AppendToCleanupList(rs)
					Expect(err).NotTo(HaveOccurred())

					// eventually RestoreSession succeeded
					By("Check for Succeeded restoreSession")
					fi.EventuallyRestoreSessionPhase(rs.ObjectMeta).Should(Equal(stashV1beta1.RestoreSucceeded))

					By("Wait for Running mysql")
					fi.EventuallyMySQLRunning(my.ObjectMeta).Should(BeTrue())

					fi.EventuallyDBReady(my, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
				})
			})

		})
	})
})
