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
	core_util "kmodules.xyz/client-go/core/v1"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	stash_v1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

var _ = Describe("Stash Backup For MariaDB", func() {
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

	Context("For Standalone MariaDB", func() {
		Context("With SSL Disabled", func() {
			It("should backup and restore in the same database", func() {
				// Deploy a MariaDB instance
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// Database connection information
				mydbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, mydbInfo)

				fi.PopulateMariaDB(md, mydbInfo)

				// create testdb that will be deleted later for restoring
				testdbinfo := framework.GetMariaDBInfo(framework.TestDBMySQL, framework.MySQLRootUser, "")
				fi.PopulateMariaDB(md, testdbinfo)

				By("Get AppBinding for Backup Purpose")
				appBinding, err := fi.Framework.GetAppBinding(md.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Setup Database Backup")
				backupConfig, repo, err := fi.SetupDatabaseBackup(appBinding)
				Expect(err).NotTo(HaveOccurred())

				// Simulate a backup run
				fi.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)

				By("Simulate disaster by dropping testdb")
				fi.EventuallyDropDatabaseMD(md.ObjectMeta, testdbinfo).Should(BeTrue())

				By("Checking if test Database exist")
				fi.EventuallyExistsDBMD(md.ObjectMeta, testdbinfo).Should(BeFalse())

				// Restore the database
				fi.RestoreDatabase(appBinding, repo)

				By("Checking if test Database restored")
				fi.EventuallyExistsDBMD(md.ObjectMeta, testdbinfo).Should(BeTrue())

				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, testdbinfo).Should(Equal(3))

			})
		})

		Context("With SSL Enabled", func() {
			BeforeEach(func() {
				if !framework.SSLEnabled {
					Skip("Skipping test. Reason: SSL is disabled")
				}
			})

			Context("With requireSSL true", func() {
				It("should backup & restore in the same database", func() {

					// Deploy a MariaDB instance
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						fi.EnableSSLMariaDB(in, true)
					})
					Expect(err).NotTo(HaveOccurred())

					// database connection information
					mydbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, fmt.Sprintf("tls=%s", framework.TLSCustomConfig))
					fi.EventuallyDBReadyMD(md, mydbInfo)

					By("Create mysql User with required SSL")
					fi.EventuallyCreateUserWithRequiredSSLMD(md.ObjectMeta, mydbInfo).Should(BeTrue())

					mydbInfo.User = framework.MySQLRequiredSSLUser
					fi.EventuallyCheckConnectionRequiredSSLUserMD(md, mydbInfo)

					fi.PopulateMariaDB(md, mydbInfo)

					testdbinfo := framework.GetMariaDBInfo(framework.TestDBMySQL, framework.MySQLRequiredSSLUser, fmt.Sprintf("tls=%s", framework.TLSCustomConfig))
					fi.PopulateMariaDB(md, testdbinfo)

					By("Get AppBinding for Backup Purpose")
					appBinding, err := fi.Framework.GetAppBinding(md.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Setup Database Backup")
					backupConfig, repo, err := fi.SetupDatabaseBackup(appBinding)
					Expect(err).NotTo(HaveOccurred())

					// Simulate a backup run
					fi.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)

					By("Simulate disaster by dropping testdb")
					fi.EventuallyDropDatabaseMD(md.ObjectMeta, testdbinfo).Should(BeTrue())

					By("Checking if test Database exist")
					fi.EventuallyExistsDBMD(md.ObjectMeta, testdbinfo).Should(BeFalse())

					// Restore the database
					fi.RestoreDatabase(appBinding, repo)

					By("Checking if test Database restored")
					fi.EventuallyExistsDBMD(md.ObjectMeta, testdbinfo).Should(BeTrue())

					By("Checking Row Count of Table")
					fi.EventuallyCountRowMD(md.ObjectMeta, testdbinfo).Should(Equal(3))
				})
			})
		})

		Context("Passing custom parameters", func() {
			It("should backup only the specified database", func() {
				// Deploy a MariaDB instance
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// Database connection information
				mydbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, mydbInfo)

				fi.PopulateMariaDB(md, mydbInfo)

				// create testdb that will be deleted later for restoring
				testdbInfo := framework.GetMariaDBInfo(framework.TestDBMySQL, framework.MySQLRootUser, "")
				fi.PopulateMariaDB(md, testdbInfo)

				// create testdb that will be deleted later for restoring
				anotherdbInfo := framework.GetMariaDBInfo(framework.AnotherDB, framework.MySQLRootUser, "")
				fi.PopulateMariaDB(md, anotherdbInfo)

				appBinding, repo := fi.BackupDatabase(md.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
					bc.Spec.Task.Params = []stash_v1beta1.Param{
						{
							Name:  framework.ParamKeyArgs,
							Value: fmt.Sprintf("--databases %s", framework.TestDBMySQL),
						},
					}
				})

				By("Simulate disaster by dropping testdb")
				fi.EventuallyDropDatabaseMD(md.ObjectMeta, testdbInfo).Should(BeTrue())

				By("Simulate disaster by dropping testdb")
				fi.EventuallyDropDatabaseMD(md.ObjectMeta, anotherdbInfo).Should(BeTrue())

				// Restore from backup. Since we have backed up only one database,
				// only "testDB" should be restored.
				fi.RestoreDatabase(appBinding, repo)

				By("Verifying that db: " + framework.TestDBMySQL + " has been restored")
				fi.EventuallyExistsDBMD(md.ObjectMeta, testdbInfo).Should(BeTrue())

				By("Verifying that db: " + framework.AnotherDBMySQL + " hasn't been restored")
				fi.EventuallyExistsDBMD(md.ObjectMeta, anotherdbInfo).Should(BeFalse())

				By("Checking Row Count of " + framework.TestDBMySQL + "'s Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, testdbInfo).Should(Equal(3))

			})

			It("should delete the old database before restoring", func() {

				// Deploy a MariaDB instance
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// Database connection information
				mydbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, mydbInfo)

				fi.PopulateMariaDB(md, mydbInfo)

				// create testdb that will be deleted later for restoring
				testdbInfo := framework.GetMariaDBInfo(framework.TestDBMySQL, framework.MySQLRootUser, "")
				fi.PopulateMariaDB(md, testdbInfo)

				// create testdb that will be deleted later for restoring
				anotherdbInfo := framework.GetMariaDBInfo(framework.AnotherDBMySQL, framework.MySQLRootUser, "")
				fi.PopulateMariaDB(md, anotherdbInfo)

				appBinding, repo := fi.BackupDatabase(md.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
					bc.Spec.Task.Params = []stash_v1beta1.Param{
						{
							Name:  framework.ParamKeyArgs,
							Value: fmt.Sprintf("--databases %s", framework.TestDBMySQL),
						},
					}
				})

				By("Inserting Rows in " + framework.TestDBMySQL + "'s Table")
				fi.EventuallyInsertRowMD(md.ObjectMeta, testdbInfo, 5).Should(BeTrue())

				By("Checking Row Count of " + framework.TestDBMySQL + "'s Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, testdbInfo).Should(Equal(8))

				By("Inserting Rows in " + framework.AnotherDBMySQL + "'s Table")
				fi.EventuallyInsertRowMD(md.ObjectMeta, anotherdbInfo, 5).Should(BeTrue())

				By("Checking Row Count of " + framework.AnotherDBMySQL + "'s Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, anotherdbInfo).Should(Equal(8))

				// Restore from backup. Since we have backed up only one database,
				// only "testDB" should be restored.
				fi.RestoreDatabase(appBinding, repo)

				By("Checking Row Count of " + framework.TestDBMySQL + "'s Table, Overwritten by old data")
				fi.EventuallyCountRowMD(md.ObjectMeta, testdbInfo).Should(Equal(3))

				By("Checking Row Count of " + framework.AnotherDBMySQL + "'s Table, Not backed up so remains same")
				fi.EventuallyCountRowMD(md.ObjectMeta, anotherdbInfo).Should(Equal(8))

			})
		})

		FContext("Using Auto-Backup", func() {
			It("should take backup successfully with default configurations", func() {

				// deploy a MariaDB instance
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)

				// populate Mariadb with some sample data
				fi.PopulateMariaDB(md, dbInfo)

				// configure Auto-Backup
				backupConfig, repo := fi.ConfigureAutoBackup(md, md.ObjectMeta)

				// simulate a backup run
				fi.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)

				// remove the auto-backup annotations so that Stash does not re-create
				// the auto-backup resources during cleanup step.
				fi.RemoveAutoBackupAnnotations(md)
			})

			It("should use schedule from the annotations", func() {
				// Deploy a MariaDB instance
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)

				// populate Mariadb with some sample data
				fi.PopulateMariaDB(md, dbInfo)

				schedule := "0 0 * 11 *"
				// Configure Auto-backup with custom schedule
				backupConfig, _ := fi.ConfigureAutoBackup(md, md.ObjectMeta, func(annotations map[string]string) {
					_ = core_util.UpsertMap(annotations, map[string]string{
						stash_v1beta1.KeySchedule: schedule,
					})
				})

				// Verify that the BackupConfiguration is using the custom schedule
				fi.VerifyCustomSchedule(backupConfig, schedule)

				// Remove the auto-backup annotations so that Stash does not re-create
				// the auto-backup resources during cleanup step.
				fi.RemoveAutoBackupAnnotations(md)
			})

			It("should use custom parameters from the annotations", func() {
				// Deploy a MariaDB instance
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)

				// populate Mariadb with some sample data
				fi.PopulateMariaDB(md, dbInfo)

				args := fmt.Sprintf("--databases %s", framework.DBMySQL)
				// Configure Auto-backup with custom schedule
				backupConfig, repo := fi.ConfigureAutoBackup(md, md.ObjectMeta, func(annotations map[string]string) {
					_ = core_util.UpsertMap(annotations, map[string]string{
						fmt.Sprintf("%s/%s", stash_v1beta1.KeyParams, framework.ParamKeyArgs): args,
					})
				})

				// Simulate a backup run
				fi.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)

				// Verify that the BackupConfiguration is using the custom schedule
				fi.VerifyParameterPassed(backupConfig.Spec.Task.Params, framework.ParamKeyArgs, args)

				// Remove the auto-backup annotations so that Stash does not re-create
				// the auto-backup resources during cleanup step.
				fi.RemoveAutoBackupAnnotations(md)
			})
		})
	})
})
