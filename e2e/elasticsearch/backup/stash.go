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

	"kubedb.dev/tests/e2e/framework"
	stash_v1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stash Backup", func() {
	var f *framework.Invocation

	BeforeEach(func() {
		f = framework.NewInvocation()

		// If backup test isn't covered by test profiles, then skip running the backup tests
		if !framework.CoveredByTestProfiles(framework.StashBackup) {
			Skip(fmt.Sprintf("Profile: %q is not covered by the test profiles: %v.", framework.StashBackup, framework.TestProfiles))
		}

		// If Stash operator hasn't been installed, then skip running the backup tests
		if !f.StashInstalled() {
			Skip("Stash is not running in the cluster. Please install Stash to run the backup tests.")
		}

		// Skip if addon name or addon version is missing
		if framework.StashAddonName == "" || framework.StashAddonVersion == "" {
			Skip("Missing Stash addon name or version")
		}

		// If the provided addon does not exist, then skip running the test
		exist, err := f.AddonExist()
		Expect(err).NotTo(HaveOccurred())
		if !exist {
			Skip(fmt.Sprintf("Stash addon name: %s version: %s does not exist", framework.StashAddonName, framework.StashAddonVersion))
		}
	})

	JustAfterEach(func() {
		// If the test fail, then print the necessary information that can help to debug
		f.PrintDebugInfoOnFailure()
	})

	AfterEach(func() {
		By("Cleaning test resources")
		err := f.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("For Elasticsearch", func() {
		Context("With Combined Nodes", func() {
			Context("Of Single Replica", func() {
				Context("Without TLS", func() {
					FIt("should backup and restore successfully", func() {
						// Deploy a Elasticsearch instance
						es := f.DeployElasticsearch()

						// Populate the Elasticsearch with some sample data
						f.PopulateElasticsearch(es, framework.SampleESIndex)

						// Backup the database
						appBinding, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
							bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
							bc.Spec.Task.Params = []stash_v1beta1.Param{
								{
									Name:  "args",
									Value: "--ignoreType='template,settings,alias'",
								},
							}
						})

						// Delete sample data
						f.SimulateElasticsearchDisaster(es, framework.SampleESIndex)

						// Restore the database
						f.RestoreDatabase(appBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
							rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
							rs.Spec.Task.Params = []stash_v1beta1.Param{
								{
									Name:  "args",
									Value: "--ignoreChildError=true",
								},
							}
						})

						// Verify restored data
						f.VerifyElasticsearchRestore(es, framework.SampleESIndex)
					})
				})
			})

			//Context("With Multiple Nodes", func() {
			//	BeforeEach(func() {
			//		if !framework.SSLEnabled {
			//			Skip("Skipping test. Reason: SSL is disabled")
			//		}
			//	})
			//
			//	Context("With SSL mode: requireSSL", func() {
			//		It("should backup & restore in the same database", func() {
			//			// Deploy a Elasticsearch instance
			//			mg := f.DeployElasticsearch(func(in *api.Elasticsearch) {
			//				f.EnableMongoSSL(in, api.SSLModeRequireSSL)
			//			})
			//
			//			// Populate the Elasticsearch with some sample data
			//			f.PopulateElasticsearch(mg, framework.SampleDB)
			//
			//			// Backup the database
			//			appBinding, repo := f.BackupDatabase(mg.ObjectMeta, 1)
			//
			//			// Delete sample data
			//			f.SimulateElasticsearchDisaster(mg.ObjectMeta, framework.SampleDB)
			//
			//			// Restore the database
			//			f.RestoreDatabase(appBinding, repo)
			//
			//			// Verify restored data
			//			f.VerifyElasticsearchRestore(mg.ObjectMeta, framework.SampleDB)
			//		})
			//	})
			//
			//	Context("With SSL mode: preferSSL", func() {
			//		It("should backup & restore in the same database", func() {
			//			// Deploy a Elasticsearch instance
			//			mg := f.DeployElasticsearch(func(in *api.Elasticsearch) {
			//				f.EnableMongoSSL(in, api.SSLModePreferSSL)
			//			})
			//
			//			// Populate the Elasticsearch with some sample data
			//			f.PopulateElasticsearch(mg, framework.SampleDB)
			//
			//			// Backup the database
			//			appBinding, repo := f.BackupDatabase(mg.ObjectMeta, 1)
			//
			//			// Delete sample data
			//			f.SimulateElasticsearchDisaster(mg.ObjectMeta, framework.SampleDB)
			//
			//			// Restore the database
			//			f.RestoreDatabase(appBinding, repo)
			//
			//			// Verify restored data
			//			f.VerifyElasticsearchRestore(mg.ObjectMeta, framework.SampleDB)
			//		})
			//	})
			//
			//	Context("With SSL mode: allowSSL", func() {
			//		It("should backup & restore in the same database", func() {
			//			// Deploy a Elasticsearch instance
			//			mg := f.DeployElasticsearch(func(in *api.Elasticsearch) {
			//				f.EnableMongoSSL(in, api.SSLModeAllowSSL)
			//			})
			//
			//			// Populate the Elasticsearch with some sample data
			//			f.PopulateElasticsearch(mg, framework.SampleDB)
			//
			//			// Backup the database
			//			appBinding, repo := f.BackupDatabase(mg.ObjectMeta, 1)
			//
			//			// Delete sample data
			//			f.SimulateElasticsearchDisaster(mg.ObjectMeta, framework.SampleDB)
			//
			//			// Restore the database
			//			f.RestoreDatabase(appBinding, repo)
			//
			//			// Verify restored data
			//			f.VerifyElasticsearchRestore(mg.ObjectMeta, framework.SampleDB)
			//		})
			//	})
			//})
			//
			//Context("Passing custom parameters", func() {
			//	It("should backup only the specified database", func() {
			//		// Deploy a Elasticsearch instance
			//		mg := f.DeployElasticsearch()
			//
			//		// Populate the Elasticsearch with multiple databases
			//		f.PopulateElasticsearch(mg, framework.SampleDB, framework.AnotherDB)
			//
			//		// Backup only "sampleDB" database
			//		appBinding, repo := f.BackupDatabase(mg.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
			//			bc.Spec.Task.Params = []stash_v1beta1.Param{
			//				{
			//					Name:  framework.ParamKeyArgs,
			//					Value: fmt.Sprintf("--db=%s", framework.SampleDB),
			//				},
			//			}
			//		})
			//
			//		// Delete both databases
			//		f.SimulateElasticsearchDisaster(mg.ObjectMeta, framework.SampleDB, framework.AnotherDB)
			//
			//		// Restore from backup. Since we have backed up only one database,
			//		// only "sampleDB" should be restored.
			//		f.RestoreDatabase(appBinding, repo)
			//
			//		By("Verifying that db: " + framework.SampleDB + " has been restored")
			//		dbExist, err := f.DatabaseExists(mg.ObjectMeta, framework.SampleDB)
			//		Expect(err).NotTo(HaveOccurred())
			//		Expect(dbExist).Should(BeTrue())
			//
			//		By("Verifying that db: " + framework.AnotherDB + " hasn't been restored")
			//		dbExist, err = f.DatabaseExists(mg.ObjectMeta, framework.AnotherDB)
			//		Expect(err).NotTo(HaveOccurred())
			//		Expect(dbExist).Should(BeFalse())
			//	})
			//
			//	It("should delete the old database before restoring", func() {
			//		// Deploy a Elasticsearch instance
			//		mg := f.DeployElasticsearch()
			//
			//		// Populate the Elasticsearch with multiple databases. This will create two databases "sampleDB" and "anotherDB".
			//		// Each of the database will have two collection "sampleCollection", "anotherCollection".
			//		f.PopulateElasticsearch(mg, framework.SampleDB, framework.AnotherDB)
			//
			//		// Backup only the "sampleDB" database
			//		appBinding, repo := f.BackupDatabase(mg.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
			//			bc.Spec.Task.Params = []stash_v1beta1.Param{
			//				{
			//					Name:  framework.ParamKeyArgs,
			//					Value: fmt.Sprintf("--db=%s", framework.SampleDB),
			//				},
			//			}
			//		})
			//
			//		// Lets update the "sampleCollection" of "sampleDB" database.
			//		f.UpdateCollection(mg.ObjectMeta, framework.SampleDB, framework.UpdatedCollection)
			//
			//		// Also, update the "sampleCollection" of "anotherDB" database
			//		f.UpdateCollection(mg.ObjectMeta, framework.AnotherDB, framework.UpdatedCollection)
			//
			//		// Now, we are going to restore the database with "--drop" argument. Since we have backed up only the "sampleDB",
			//		// only it will be restored.
			//		f.RestoreDatabase(appBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
			//			rs.Spec.Task.Params = []stash_v1beta1.Param{
			//				{
			//					Name:  framework.ParamKeyArgs,
			//					Value: "--drop",
			//				},
			//			}
			//		})
			//
			//		// Since, we have used "--drop" parameter during restoring, the current state "sampleDB" should be overwritten
			//		// by the backed up state. So, the update we made in "sampleCollection" collection of "sampleDB" database,
			//		// should be overwritten.
			//		By("Verifying that collection: " + framework.SampleCollection.Name + "of db: " + framework.SampleDB + " has been overwritten by restored data")
			//		resp, err := f.GetDocument(mg.ObjectMeta, framework.SampleDB, framework.SampleCollection.Name)
			//		Expect(err).NotTo(HaveOccurred())
			//		Expect(resp.State).Should(Equal(framework.SampleCollection.Document.State))
			//
			//		// However, we haven't backed up "anotherDB". So, its updated state shouldn't be overwritten by the
			//		// restored data.
			//		By("Verifying that collection: " + framework.SampleCollection.Name + " of db: " + framework.AnotherDB + " hasn't been overwritten")
			//		resp, err = f.GetDocument(mg.ObjectMeta, framework.AnotherDB, framework.SampleCollection.Name)
			//		Expect(err).NotTo(HaveOccurred())
			//		Expect(resp.State).Should(Equal(framework.UpdatedCollection.Document.State))
			//	})
			//})
			//
			//Context("Using Auto-Backup", func() {
			//	It("should take backup successfully with default configurations", func() {
			//		// Deploy a Elasticsearch instance
			//		mg := f.DeployElasticsearch()
			//
			//		// Populate Elasticsearch with some sample data
			//		f.PopulateElasticsearch(mg, framework.SampleDB)
			//
			//		// Configure Auto-backup
			//		backupConfig, repo := f.ConfigureAutoBackup(mg, mg.ObjectMeta)
			//
			//		// Simulate a backup run
			//		f.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)
			//
			//		// Remove the auto-backup annotations so that Stash does not re-create
			//		// the auto-backup resources during cleanup step.
			//		f.RemoveAutoBackupAnnotations(mg)
			//	})
			//
			//	It("should use schedule from the annotations", func() {
			//		// Deploy a Elasticsearch instance
			//		mg := f.DeployElasticsearch()
			//
			//		// Populate Elasticsearch with some sample data
			//		f.PopulateElasticsearch(mg, framework.SampleDB)
			//
			//		schedule := "0 0 * 11 *"
			//		// Configure Auto-backup with custom schedule
			//		backupConfig, _ := f.ConfigureAutoBackup(mg, mg.ObjectMeta, func(annotations map[string]string) {
			//			_ = core_util.UpsertMap(annotations, map[string]string{
			//				stash_v1beta1.KeySchedule: schedule,
			//			})
			//		})
			//
			//		// Verify that the BackupConfiguration is using the custom schedule
			//		f.VerifyCustomSchedule(backupConfig, schedule)
			//
			//		// Remove the auto-backup annotations so that Stash does not re-create
			//		// the auto-backup resources during cleanup step.
			//		f.RemoveAutoBackupAnnotations(mg)
			//	})
			//
			//	It("should use custom parameters from the annotations", func() {
			//		// Deploy a Elasticsearch instance
			//		mg := f.DeployElasticsearch()
			//
			//		// Populate Elasticsearch with some sample data
			//		f.PopulateElasticsearch(mg, framework.SampleDB)
			//
			//		args := fmt.Sprintf("--db=%s", framework.SampleDB)
			//		// Configure Auto-backup with custom schedule
			//		backupConfig, repo := f.ConfigureAutoBackup(mg, mg.ObjectMeta, func(annotations map[string]string) {
			//			_ = core_util.UpsertMap(annotations, map[string]string{
			//				fmt.Sprintf("%s/%s", stash_v1beta1.KeyParams, framework.ParamKeyArgs): args,
			//			})
			//		})
			//
			//		// Simulate a backup run
			//		f.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)
			//
			//		// Verify that the BackupConfiguration is using the custom schedule
			//		f.VerifyParameterPassed(backupConfig.Spec.Task.Params, framework.ParamKeyArgs, args)
			//
			//		// Remove the auto-backup annotations so that Stash does not re-create
			//		// the auto-backup resources during cleanup step.
			//		f.RemoveAutoBackupAnnotations(mg)
			//	})
			//})
		})

		//Context("With Topology", func() {
		//	Context("With SSL Disabled", func() {
		//		It("should backup and restore in the same database", func() {
		//			// Deploy a Elasticsearch instance
		//			mg := f.DeployElasticsearch(func(in *api.Elasticsearch) {
		//				f.EnableMongoReplication(in)
		//			})
		//
		//			// Populate the Elasticsearch with some sample data
		//			f.PopulateElasticsearch(mg, framework.SampleDB)
		//
		//			// Backup the database
		//			appBinding, repo := f.BackupDatabase(mg.ObjectMeta, 1)
		//
		//			// Delete sample data
		//			f.SimulateElasticsearchDisaster(mg.ObjectMeta, framework.SampleDB)
		//
		//			// Restore the database
		//			f.RestoreDatabase(appBinding, repo)
		//
		//			// Verify restored data
		//			f.VerifyElasticsearchRestore(mg.ObjectMeta, framework.SampleDB)
		//		})
		//	})
		//
		//	Context("With SSL Enabled", func() {
		//		BeforeEach(func() {
		//			if !framework.SSLEnabled {
		//				Skip("Skipping test. Reason: SSL is disabled")
		//			}
		//		})
		//
		//		Context("With SSL mode: requireSSL", func() {
		//			It("should backup and restore in the same database", func() {
		//				// Deploy a Elasticsearch instance
		//				mg := f.DeployElasticsearch(func(in *api.Elasticsearch) {
		//					f.EnableMongoReplication(in)
		//					f.EnableMongoSSL(in, api.SSLModeRequireSSL)
		//				})
		//
		//				// Populate the Elasticsearch with some sample data
		//				f.PopulateElasticsearch(mg, framework.SampleDB)
		//
		//				// Backup the database
		//				appBinding, repo := f.BackupDatabase(mg.ObjectMeta, 1)
		//
		//				// Delete sample data
		//				f.SimulateElasticsearchDisaster(mg.ObjectMeta, framework.SampleDB)
		//
		//				// Restore the database
		//				f.RestoreDatabase(appBinding, repo)
		//
		//				// Verify restored data
		//				f.VerifyElasticsearchRestore(mg.ObjectMeta, framework.SampleDB)
		//			})
		//		})
		//
		//		Context("With SSL mode: preferSSL", func() {
		//			It("should backup and restore in the same database", func() {
		//				// Deploy a Elasticsearch instance
		//				mg := f.DeployElasticsearch(func(in *api.Elasticsearch) {
		//					f.EnableMongoReplication(in)
		//					f.EnableMongoSSL(in, api.SSLModePreferSSL)
		//				})
		//
		//				// Populate the Elasticsearch with some sample data
		//				f.PopulateElasticsearch(mg, framework.SampleDB)
		//
		//				// Backup the database
		//				appBinding, repo := f.BackupDatabase(mg.ObjectMeta, 1)
		//
		//				// Delete sample data
		//				f.SimulateElasticsearchDisaster(mg.ObjectMeta, framework.SampleDB)
		//
		//				// Restore the database
		//				f.RestoreDatabase(appBinding, repo)
		//
		//				// Verify restored data
		//				f.VerifyElasticsearchRestore(mg.ObjectMeta, framework.SampleDB)
		//			})
		//		})
		//
		//		Context("With SSL mode: allowSSL", func() {
		//			It("should backup and restore in the same database", func() {
		//				// Deploy a Elasticsearch instance
		//				mg := f.DeployElasticsearch(func(in *api.Elasticsearch) {
		//					f.EnableMongoReplication(in)
		//					f.EnableMongoSSL(in, api.SSLModeAllowSSL)
		//				})
		//
		//				// Populate the Elasticsearch with some sample data
		//				f.PopulateElasticsearch(mg, framework.SampleDB)
		//
		//				// Backup the database
		//				appBinding, repo := f.BackupDatabase(mg.ObjectMeta, 1)
		//
		//				// Delete sample data
		//				f.SimulateElasticsearchDisaster(mg.ObjectMeta, framework.SampleDB)
		//
		//				// Restore the database
		//				f.RestoreDatabase(appBinding, repo)
		//
		//				// Verify restored data
		//				f.VerifyElasticsearchRestore(mg.ObjectMeta, framework.SampleDB)
		//			})
		//		})
		//	})
		//})
	})

})
