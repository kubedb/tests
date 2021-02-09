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
			FIt("should backup only the specified database", func() {
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
							Value: fmt.Sprintf("--databases=%s", framework.TestDBMySQL),
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

				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, testdbInfo).Should(Equal(3))

				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, anotherdbInfo).Should(Equal(3))

				//
				//
				//
				//
				//
				//
				//
				//// Deploy a MongoDB instance
				//mg := fi.DeployMongoDB()
				//
				//// Populate the MongoDB with multiple databases
				//fi.PopulateMongoDB(mg, framework.SampleDB, framework.AnotherDB)
				//
				//// Backup only "sampleDB" database
				//appBinding, repo := fi.BackupDatabase(mg.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
				//	bc.Spec.Task.Params = []stash_v1beta1.Param{
				//		{
				//			Name:  framework.ParamKeyArgs,
				//			Value: fmt.Sprintf("--db=%s", framework.SampleDB),
				//		},
				//	}
				//})
				//
				//// Delete both databases
				//fi.SimulateMongoDBDisaster(mg.ObjectMeta, framework.SampleDB, framework.AnotherDB)
				//
				//// Restore from backup. Since we have backed up only one database,
				//// only "sampleDB" should be restored.
				//fi.RestoreDatabase(appBinding, repo)
				//
				//By("Verifying that db: " + framework.SampleDB + " has been restored")
				//dbExist, err := fi.DatabaseExists(mg.ObjectMeta, framework.SampleDB)
				//Expect(err).NotTo(HaveOccurred())
				//Expect(dbExist).Should(BeTrue())
				//
				//By("Verifying that db: " + framework.AnotherDB + " hasn't been restored")
				//dbExist, err = fi.DatabaseExists(mg.ObjectMeta, framework.AnotherDB)
				//Expect(err).NotTo(HaveOccurred())
				//Expect(dbExist).Should(BeFalse())
			})

			It("should delete the old database before restoring", func() {
				// Deploy a MongoDB instance
				mg := fi.DeployMongoDB()

				// Populate the MongoDB with multiple databases. This will create two databases "sampleDB" and "anotherDB".
				// Each of the database will have two collection "sampleCollection", "anotherCollection".
				fi.PopulateMongoDB(mg, framework.SampleDB, framework.AnotherDB)

				// Backup only the "sampleDB" database
				appBinding, repo := fi.BackupDatabase(mg.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
					bc.Spec.Task.Params = []stash_v1beta1.Param{
						{
							Name:  framework.ParamKeyArgs,
							Value: fmt.Sprintf("--db=%s", framework.SampleDB),
						},
					}
				})

				// Lets update the "sampleCollection" of "sampleDB" database.
				fi.UpdateCollection(mg.ObjectMeta, framework.SampleDB, framework.UpdatedCollection)

				// Also, update the "sampleCollection" of "anotherDB" database
				fi.UpdateCollection(mg.ObjectMeta, framework.AnotherDB, framework.UpdatedCollection)

				// Now, we are going to restore the database with "--drop" argument. Since we have backed up only the "sampleDB",
				// only it will be restored.
				fi.RestoreDatabase(appBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
					rs.Spec.Task.Params = []stash_v1beta1.Param{
						{
							Name:  framework.ParamKeyArgs,
							Value: "--drop",
						},
					}
				})

				// Since, we have used "--drop" parameter during restoring, the current state "sampleDB" should be overwritten
				// by the backed up state. So, the update we made in "sampleCollection" collection of "sampleDB" database,
				// should be overwritten.
				By("Verifying that collection: " + framework.SampleCollection.Name + "of db: " + framework.SampleDB + " has been overwritten by restored data")
				resp, err := fi.GetDocument(mg.ObjectMeta, framework.SampleDB, framework.SampleCollection.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.State).Should(Equal(framework.SampleCollection.Document.State))

				// However, we haven't backed up "anotherDB". So, its updated state shouldn't be overwritten by the
				// restored data.
				By("Verifying that collection: " + framework.SampleCollection.Name + " of db: " + framework.AnotherDB + " hasn't been overwritten")
				resp, err = fi.GetDocument(mg.ObjectMeta, framework.AnotherDB, framework.SampleCollection.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.State).Should(Equal(framework.UpdatedCollection.Document.State))
			})
		})
	})
})
