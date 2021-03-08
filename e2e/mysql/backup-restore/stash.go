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

package backup_restore

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"
	"kubedb.dev/tests/e2e/matcher"
	"kubedb.dev/tests/e2e/mysql"

	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gomodules.xyz/x/crypto/rand"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core_util "kmodules.xyz/client-go/core/v1"
	stash_v1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

var _ = Describe("Stash Backup For MySQL", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !mysql.RunTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !mysql.RunTestCommunity(framework.Community) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Community))
		}
		if !mysql.RunTestEnterprise(framework.Enterprise) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Enterprise))
		}

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

	Context("For Standalone MySQL", func() {
		Context("With SSL Disabled", func() {
			It("should backup and restore in the same database", func() {
				// Deploy a MySQL instance
				my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// Database connection information
				dbInfo := framework.DatabaseConnectionInfo{
					DatabaseName: framework.DBMySQL,
					User:         framework.MySQLRootUser,
					Param:        "",
				}
				fi.EventuallyDBReady(my, dbInfo)

				// create testdb for backup and restore
				By("Create MySQL testdb Database")
				queries := []string{
					fmt.Sprintf("CREATE DATABASE %s;", framework.MySQLTestDB),
				}
				fi.EventuallyCreate(my.ObjectMeta, dbInfo, queries).Should(BeTrue())
				dbInfo.DatabaseName = framework.MySQLTestDB
				fi.PopulateMySQL(my.ObjectMeta, dbInfo)

				By("Get AppBinding for Backup Purpose")
				appBinding, err := fi.Framework.GetAppBinding(my.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Setup Database Backup")
				backupConfig, repo, err := fi.SetupDatabaseBackup(appBinding)
				Expect(err).NotTo(HaveOccurred())

				// Simulate a backup run
				fi.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)

				By("Simulate disaster by dropping testdb")
				dbInfo.DatabaseName = framework.DBMySQL
				queries = []string{
					fmt.Sprintf("DROP DATABASE %s;", framework.MySQLTestDB),
				}
				fi.EventuallyDrop(my.ObjectMeta, dbInfo, queries).Should(BeTrue())

				By("Checking if test Database doesn't exist")
				fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).ShouldNot(matcher.HaveExists(framework.MySQLTestDB))

				// Restore the database
				fi.RestoreDatabase(appBinding, repo)

				By("Checking if test Database restored")
				fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).Should(matcher.HaveExists(framework.MySQLTestDB))

				By("Checking Row Count of Table")
				dbInfo.DatabaseName = framework.MySQLTestDB
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
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
					issuerMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.EnsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
					Expect(err).NotTo(HaveOccurred())
					// Deploy a MySQL instance
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, framework.AddTLSConfig(issuer.ObjectMeta))
					Expect(err).NotTo(HaveOccurred())

					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
					}
					fi.EventuallyDBReady(my, dbInfo)

					// Create a mysql User with required SSL
					By("Create mysql User with required SSL")
					fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, dbInfo).Should(BeTrue())
					dbInfo.User = framework.MySQLRequiredSSLUser
					fi.EventuallyCheckConnectionRequiredSSLUser(my, dbInfo)

					// create testdb for backup and restore
					By("Create MySQL testdb Database")
					queries := []string{
						fmt.Sprintf("CREATE DATABASE %s;", framework.MySQLTestDB),
					}
					fi.EventuallyCreate(my.ObjectMeta, dbInfo, queries).Should(BeTrue())
					dbInfo.DatabaseName = framework.MySQLTestDB
					fi.PopulateMySQL(my.ObjectMeta, dbInfo)

					By("Get AppBinding for Backup Purpose")
					appBinding, err := fi.Framework.GetAppBinding(my.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Setup Database Backup")
					backupConfig, repo, err := fi.SetupDatabaseBackup(appBinding)
					Expect(err).NotTo(HaveOccurred())

					// Simulate a backup run
					fi.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)

					By("Simulate disaster by dropping testdb")
					dbInfo.DatabaseName = framework.DBMySQL
					queries = []string{
						fmt.Sprintf("DROP DATABASE %s;", framework.MySQLTestDB),
					}
					fi.EventuallyDrop(my.ObjectMeta, dbInfo, queries).Should(BeTrue())

					By("Checking if test Database doesn't exist")
					fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).ShouldNot(matcher.HaveExists(framework.MySQLTestDB))

					// Restore the database
					fi.RestoreDatabase(appBinding, repo)

					By("Checking if test Database restored")
					fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).Should(matcher.HaveExists(framework.MySQLTestDB))

					By("Checking Row Count of Table")
					dbInfo.DatabaseName = framework.MySQLTestDB
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
				})
			})
		})

		Context("Passing custom parameters", func() {
			It("should backup only the specified database", func() {
				// Deploy a MySQL instance
				my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// Database connection information
				dbInfo := framework.DatabaseConnectionInfo{
					DatabaseName: framework.DBMySQL,
					User:         framework.MySQLRootUser,
					Param:        "",
				}
				fi.EventuallyDBReady(my, dbInfo)

				// create testdb that will be deleted later for restoring
				By("Create MySQL testdb Database")
				queries := []string{
					fmt.Sprintf("CREATE DATABASE %s;", framework.MySQLTestDB),
				}
				fi.EventuallyCreate(my.ObjectMeta, dbInfo, queries).Should(BeTrue())
				dbInfo.DatabaseName = framework.MySQLTestDB
				fi.PopulateMySQL(my.ObjectMeta, dbInfo)

				// create anotherdb that will be deleted later for restoring
				By("Create MySQL testdb Database")
				dbInfo.DatabaseName = framework.DBMySQL
				queries = []string{
					fmt.Sprintf("CREATE DATABASE %s;", framework.AnotherDB),
				}
				fi.EventuallyCreate(my.ObjectMeta, dbInfo, queries).Should(BeTrue())
				dbInfo.DatabaseName = framework.AnotherDB
				fi.PopulateMySQL(my.ObjectMeta, dbInfo)

				appBinding, repo := fi.BackupDatabase(my.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
					bc.Spec.Task.Params = []stash_v1beta1.Param{
						{
							Name:  framework.ParamKeyArgs,
							Value: fmt.Sprintf("--databases %s", framework.MySQLTestDB),
						},
					}
				})

				By("Simulate disaster by dropping testdb")
				dbInfo.DatabaseName = framework.DBMySQL
				queries = []string{
					fmt.Sprintf("DROP DATABASE %s;", framework.MySQLTestDB),
				}
				fi.EventuallyDrop(my.ObjectMeta, dbInfo, queries).Should(BeTrue())

				By("Simulate disaster by dropping anotherdb")
				queries = []string{
					fmt.Sprintf("DROP DATABASE %s;", framework.AnotherDB),
				}
				fi.EventuallyDrop(my.ObjectMeta, dbInfo, queries).Should(BeTrue())

				By("Checking if test Database doesn't exist")
				fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).ShouldNot(matcher.HaveExists(framework.MySQLTestDB))

				By("Checking if another Database doesn't exist")
				fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).ShouldNot(matcher.HaveExists(framework.AnotherDB))

				// Restore from backup. Since we have backed up only one database,
				// only "testDB" should be restored.
				fi.RestoreDatabase(appBinding, repo)

				By("Checking if test Database restored")
				fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).Should(matcher.HaveExists(framework.MySQLTestDB))

				// another database will not be restored because we didn't backup another database
				By("Checking if another Database doesn't restored")
				fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).ShouldNot(matcher.HaveExists(framework.AnotherDB))

				By("Checking Row Count of Table")
				dbInfo.DatabaseName = framework.MySQLTestDB
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
			})

			It("should delete the old database before restoring", func() {
				// Deploy a MySQL instance
				my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// Database connection information
				dbInfo := framework.DatabaseConnectionInfo{
					DatabaseName: framework.DBMySQL,
					User:         framework.MySQLRootUser,
					Param:        "",
				}
				fi.EventuallyDBReady(my, dbInfo)

				// create testdb that will be deleted later for restoring
				By("Create MySQL testdb Database")
				queries := []string{
					fmt.Sprintf("CREATE DATABASE %s;", framework.MySQLTestDB),
				}
				fi.EventuallyCreate(my.ObjectMeta, dbInfo, queries).Should(BeTrue())
				dbInfo.DatabaseName = framework.MySQLTestDB
				fi.PopulateMySQL(my.ObjectMeta, dbInfo)

				// create anotherdb that will be deleted later for restoring
				By("Create MySQL testdb Database")
				dbInfo.DatabaseName = framework.DBMySQL
				queries = []string{
					fmt.Sprintf("CREATE DATABASE %s;", framework.AnotherDB),
				}
				fi.EventuallyCreate(my.ObjectMeta, dbInfo, queries).Should(BeTrue())
				dbInfo.DatabaseName = framework.AnotherDB
				fi.PopulateMySQL(my.ObjectMeta, dbInfo)

				appBinding, repo := fi.BackupDatabase(my.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
					bc.Spec.Task.Params = []stash_v1beta1.Param{
						{
							Name:  framework.ParamKeyArgs,
							Value: fmt.Sprintf("--databases %s", framework.MySQLTestDB),
						},
					}
				})

				By("Inserting Rows in " + framework.MySQLTestDB + "'s Table")
				dbInfo.DatabaseName = framework.MySQLTestDB
				fi.EventuallyInsertRow(my.ObjectMeta, dbInfo, 5).Should(BeTrue())
				By("Checking Row Count of " + framework.MySQLTestDB + "'s Table")
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(8))

				By("Inserting Rows in " + framework.MySQLTestDB + "'s Table")
				dbInfo.DatabaseName = framework.AnotherDB
				fi.EventuallyInsertRow(my.ObjectMeta, dbInfo, 5).Should(BeTrue())
				By("Checking Row Count of " + framework.MySQLTestDB + "'s Table")
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(8))

				// Restore from backup. Since we have backed up only one database,
				// only "testDB" should be restored.
				fi.RestoreDatabase(appBinding, repo)

				By("Checking Row Count of " + framework.MySQLTestDB + "'s Table")
				dbInfo.DatabaseName = framework.MySQLTestDB
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))

				By("Checking Row Count of " + framework.AnotherDB + "'s Table")
				dbInfo.DatabaseName = framework.AnotherDB
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(8))
			})
		})

		Context("Using Auto-Backup", func() {
			It("should take backup successfully with default configurations", func() {
				// Deploy a MySQL instance
				my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// Database connection information
				dbInfo := framework.DatabaseConnectionInfo{
					DatabaseName: framework.DBMySQL,
					User:         framework.MySQLRootUser,
					Param:        "",
				}
				fi.EventuallyDBReady(my, dbInfo)
				fi.PopulateMySQL(my.ObjectMeta, dbInfo)

				// configure Auto-Backup
				backupConfig, repo := fi.ConfigureAutoBackup(my, my.ObjectMeta)

				// simulate a backup run
				fi.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)

				// remove the auto-backup annotations so that Stash does not re-create
				// the auto-backup resources during cleanup step.
				fi.RemoveAutoBackupAnnotations(my)
			})

			It("should use schedule from the annotations", func() {
				// Deploy a MySQL instance
				my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// Database connection information
				dbInfo := framework.DatabaseConnectionInfo{
					DatabaseName: framework.DBMySQL,
					User:         framework.MySQLRootUser,
					Param:        "",
				}
				fi.EventuallyDBReady(my, dbInfo)
				fi.PopulateMySQL(my.ObjectMeta, dbInfo)

				schedule := "0 0 * 11 *"
				// Configure Auto-backup with custom schedule
				backupConfig, _ := fi.ConfigureAutoBackup(my, my.ObjectMeta, func(annotations map[string]string) {
					_ = core_util.UpsertMap(annotations, map[string]string{
						stash_v1beta1.KeySchedule: schedule,
					})
				})

				// Verify that the BackupConfiguration is using the custom schedule
				fi.VerifyCustomSchedule(backupConfig, schedule)

				// Remove the auto-backup annotations so that Stash does not re-create
				// the auto-backup resources during cleanup step.
				fi.RemoveAutoBackupAnnotations(my)
			})

			It("should use custom parameters from the annotations", func() {
				// Deploy a MySQL instance
				my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())

				// Database connection information
				dbInfo := framework.DatabaseConnectionInfo{
					DatabaseName: framework.DBMySQL,
					User:         framework.MySQLRootUser,
					Param:        "",
				}
				fi.EventuallyDBReady(my, dbInfo)
				fi.PopulateMySQL(my.ObjectMeta, dbInfo)

				args := fmt.Sprintf("--databases %s", framework.DBMySQL)
				// Configure Auto-backup with custom schedule
				backupConfig, repo := fi.ConfigureAutoBackup(my, my.ObjectMeta, func(annotations map[string]string) {
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
				fi.RemoveAutoBackupAnnotations(my)
			})
		})
	})
	Context("For MySQL Group Replication", func() {
		Context("With SSL Disabled", func() {
			It("should backup and restore in the same database", func() {
				// Deploy a MySQL instance
				my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
					in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
					clusterMode := api.MySQLClusterModeGroup
					in.Spec.Topology = &api.MySQLClusterTopology{
						Mode: &clusterMode,
						Group: &api.MySQLGroupSpec{
							Name: "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
						},
					}
				})
				Expect(err).NotTo(HaveOccurred())

				// Database connection information
				dbInfo := framework.DatabaseConnectionInfo{
					DatabaseName: framework.DBMySQL,
					User:         framework.MySQLRootUser,
					Param:        "",
				}
				fi.EventuallyDBReady(my, dbInfo)

				// create testdb that will be deleted later for restoring
				By("Create MySQL testdb Database")
				queries := []string{
					fmt.Sprintf("CREATE DATABASE %s;", framework.MySQLTestDB),
				}
				fi.EventuallyCreate(my.ObjectMeta, dbInfo, queries).Should(BeTrue())
				dbInfo.DatabaseName = framework.MySQLTestDB
				fi.PopulateMySQL(my.ObjectMeta, dbInfo)

				appBinding, repo := fi.BackupDatabase(my.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
					bc.Spec.Task.Params = []stash_v1beta1.Param{
						{
							Name:  framework.ParamKeyArgs,
							Value: "--single-transaction --all-databases --set-gtid-purged=OFF",
						},
					}
				})

				By("Simulate disaster by dropping testdb")
				dbInfo.DatabaseName = framework.DBMySQL
				queries = []string{
					fmt.Sprintf("DROP DATABASE %s;", framework.MySQLTestDB),
				}
				fi.EventuallyDrop(my.ObjectMeta, dbInfo, queries).Should(BeTrue())

				By("Checking if test Database doesn't exist")
				fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).ShouldNot(matcher.HaveExists(framework.MySQLTestDB))

				// Restore the database
				fi.RestoreDatabase(appBinding, repo)

				By("Checking if test Database restored")
				fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).Should(matcher.HaveExists(framework.MySQLTestDB))

				By("Checking Row Count of Table")
				dbInfo.DatabaseName = framework.MySQLTestDB
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})

		Context("With SSL Enabled", func() {
			BeforeEach(func() {
				if !framework.SSLEnabled {
					Skip("Skipping test. Reason: SSL is disabled")
				}
			})

			Context("With require SSL true", func() {
				It("should backup and restore in the same database", func() {
					issuerMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.EnsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
					Expect(err).NotTo(HaveOccurred())
					// Deploy a MySQL instance
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
						clusterMode := api.MySQLClusterModeGroup
						in.Spec.Topology = &api.MySQLClusterTopology{
							Mode: &clusterMode,
							Group: &api.MySQLGroupSpec{
								Name: "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
							},
						}
					}, framework.AddTLSConfig(issuer.ObjectMeta))
					Expect(err).NotTo(HaveOccurred())

					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
					}
					fi.EventuallyDBReady(my, dbInfo)

					// Create a mysql User with required SSL
					By("Create mysql User with required SSL")
					fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, dbInfo).Should(BeTrue())
					dbInfo.User = framework.MySQLRequiredSSLUser
					fi.EventuallyCheckConnectionRequiredSSLUser(my, dbInfo)

					// create testdb that will be deleted later for restoring
					By("Create MySQL testdb Database")
					queries := []string{
						fmt.Sprintf("CREATE DATABASE %s;", framework.MySQLTestDB),
					}
					fi.EventuallyCreate(my.ObjectMeta, dbInfo, queries).Should(BeTrue())
					dbInfo.DatabaseName = framework.MySQLTestDB
					fi.PopulateMySQL(my.ObjectMeta, dbInfo)

					appBinding, repo := fi.BackupDatabase(my.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.Task.Params = []stash_v1beta1.Param{
							{
								Name:  framework.ParamKeyArgs,
								Value: "--single-transaction --all-databases --set-gtid-purged=OFF",
							},
						}
					})

					By("Simulate disaster by dropping testdb")
					dbInfo.DatabaseName = framework.DBMySQL
					queries = []string{
						fmt.Sprintf("DROP DATABASE %s;", framework.MySQLTestDB),
					}
					fi.EventuallyDrop(my.ObjectMeta, dbInfo, queries).Should(BeTrue())

					By("Checking if test Database doesn't exist")
					fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).ShouldNot(matcher.HaveExists(framework.MySQLTestDB))

					// Restore the database
					fi.RestoreDatabase(appBinding, repo)

					By("Checking if test Database restored")
					fi.EventuallyExists(my.ObjectMeta, dbInfo, framework.ShowDatabases).Should(matcher.HaveExists(framework.MySQLTestDB))

					By("Checking Row Count of Table")
					dbInfo.DatabaseName = framework.MySQLTestDB
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
				})
			})
		})
	})
})
