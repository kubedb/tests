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

package backup

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gomodules.xyz/x/crypto/rand"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = FDescribe("Stash Backup For MariaDB", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		// If backup test isn't covered by test profiles, then skip running the backup tests
		if !framework.CoveredByTestProfiles(framework.StashBackup) {
			Skip(fmt.Sprintf("Profile: %q is not covered by the test profiles: %v.", framework.StashBackup, framework.TestProfiles))
		}

		//If Stash operator hasn't been installed, then skip running the backup tests
		if !fi.StashInstalled() {
			Skip("Stash is not running in the cluster. Please install Stash to run the backup tests.")
		}

		// Skip if addon name or addon version is missing
		if framework.StashAddonName == "" || framework.StashAddonVersion == "" {
			Skip("Missing Stash addon name or version")
		}

		// If the provided addon does not exist, then skip running the test
		exist, err := fi.AddonExist()
		Expect(err).NotTo(HaveOccurred())
		if !exist {
			Skip(fmt.Sprintf("Stash addon name: %s version: %s does not exist", framework.StashAddonName, framework.StashAddonVersion))
		}
	})

	JustAfterEach(func() {
		// If the test fail, then print the necessary information that can help to debug
		fi.PrintDebugInfoOnFailure()
	})

	AfterEach(func() {
		By("Cleaning test resources")
		err := fi.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("For Standalone MairaDB", func() {
		Context("With SSL Disabled", func() {
			It("should backup and restore in the same database", func() {
				// Deploy a MairaDB instance
				mdMeta := metav1.ObjectMeta{
					Name:      rand.WithUniqSuffix("mairadb"),
					Namespace: fi.Namespace(),
				}
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Name = mdMeta.Name
					in.Namespace = mdMeta.Namespace
					// Set termination policy Halt to leave the PVCs and secrets intact for reuse
					in.Spec.TerminationPolicy = api.TerminationPolicyDelete
				})
				Expect(err).NotTo(HaveOccurred())
				// Database connection information

				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)

				By("Creating Table")
				fi.EventuallyCreateTableMD(mdMeta, dbInfo).Should(BeTrue())

				By("Inserting Rows")
				fi.EventuallyInsertRowMD(mdMeta, dbInfo, 3).Should(BeTrue())

				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))

				By("Creating test Database")
				fi.EventuallyCreateTestDBMD(mdMeta, dbInfo).Should(BeTrue())


				testdbInfo := framework.GetMariaDBInfo(framework.TestDBMySQL, framework.MySQLRootUser, "")

				By("Checking if test Database exist")
				fi.EventuallyExistsTestDBMD(mdMeta, testdbInfo).Should(BeTrue())

				By("Creating Table")
				fi.EventuallyTestDBCreateTableMD(mdMeta, testdbInfo).Should(BeTrue())

				By("Inserting Rows")
				fi.EventuallyTestDBInsertRowMD(mdMeta, testdbInfo, 3).Should(BeTrue())

				By("Checking Row Count of Table")
				fi.EventuallyTestDBCountRowMD(mdMeta, testdbInfo).Should(Equal(3))


				By("Get AppBinding for Backup Purpose")
				appBinding, err := fi.GetMariaDBAppBinding(mdMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Setup Database Backup")
				backupConfig, repo, err := fi.SetupDatabaseBackup(appBinding)
				Expect(err).NotTo(HaveOccurred())

				// Simulate a backup run
				fi.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)

				By("Simulate disaster by dropping testdb")
				fi.EventuallyDropDatabaseMD(mdMeta, testdbInfo).Should(BeTrue())

				By("Checking if test Database exist")
				fi.EventuallyExistsTestDBMD(mdMeta, testdbInfo).Should(BeFalse())

				// Restore the database
				fi.RestoreDatabase(appBinding, repo)

				By("Checking if test Database restored")
				fi.EventuallyExistsTestDBMD(mdMeta, testdbInfo).Should(BeTrue())

			})
		})
	})
})
