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
	"gomodules.xyz/pointer"
	core_util "kmodules.xyz/client-go/core/v1"
	stash_v1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
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
					It("should backup and restore successfully", func() {
						// Deploy a Elasticsearch instance
						es := f.DeployElasticsearch()

						// Populate the Elasticsearch with some sample data
						f.PopulateElasticsearch(es, framework.SampleESIndex)

						// Backup the database
						appBinding, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
							bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						})

						// Delete sample data
						f.SimulateElasticsearchDisaster(es, framework.SampleESIndex)

						// Restore the database
						f.RestoreDatabase(appBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
							rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						})

						// Verify restored data
						f.VerifyElasticsearchRestore(es, framework.SampleESIndex)
					})
				})
				Context("With TLS", func() {
					It("should backup and restore successfully", func() {
						// Deploy a Elasticsearch instance
						es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
							f.EnableElasticsearchSSL(in)
						})

						// Populate the Elasticsearch with some sample data
						f.PopulateElasticsearch(es, framework.SampleESIndex)

						// Backup the database
						appBinding, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
							bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						})

						// Delete sample data
						f.SimulateElasticsearchDisaster(es, framework.SampleESIndex)

						// Restore the database
						f.RestoreDatabase(appBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
							rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						})

						// Verify restored data
						f.VerifyElasticsearchRestore(es, framework.SampleESIndex)
					})
				})
			})

			Context("Of Multiple Replicas", func() {
				Context("Without TLS", func() {
					It("should backup and restore successfully", func() {
						// Deploy a Elasticsearch instance
						es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
							in.Spec.Replicas = pointer.Int32P(3)
						})

						// Populate the Elasticsearch with some sample data
						f.PopulateElasticsearch(es, framework.SampleESIndex)

						// Backup the database
						appBinding, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
							bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						})

						// Delete sample data
						f.SimulateElasticsearchDisaster(es, framework.SampleESIndex)

						// Restore the database
						f.RestoreDatabase(appBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
							rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						})

						// Verify restored data
						f.VerifyElasticsearchRestore(es, framework.SampleESIndex)
					})
				})
				Context("With TLS", func() {
					It("should backup and restore successfully", func() {
						// Deploy a Elasticsearch instance
						es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
							in.Spec.Replicas = pointer.Int32P(3)
							f.EnableElasticsearchSSL(in)
						})

						// Populate the Elasticsearch with some sample data
						f.PopulateElasticsearch(es, framework.SampleESIndex)

						// Backup the database
						appBinding, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
							bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						})

						// Delete sample data
						f.SimulateElasticsearchDisaster(es, framework.SampleESIndex)

						// Restore the database
						f.RestoreDatabase(appBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
							rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						})

						// Verify restored data
						f.VerifyElasticsearchRestore(es, framework.SampleESIndex)
					})
				})
			})
		})

		Context("With Dedicated Nodes", func() {
			Context("Without TLS", func() {
				It("should backup and restore successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						f.AddDedicatedESNodes(in)
					})

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					appBinding, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete sample data
					f.SimulateElasticsearchDisaster(es, framework.SampleESIndex)

					// Restore the database
					f.RestoreDatabase(appBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Verify restored data
					f.VerifyElasticsearchRestore(es, framework.SampleESIndex)
				})
			})
			Context("With TLS", func() {
				It("should backup and restore successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						f.AddDedicatedESNodes(in)
						f.EnableElasticsearchSSL(in)
					})

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					appBinding, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete sample data
					f.SimulateElasticsearchDisaster(es, framework.SampleESIndex)

					// Restore the database
					f.RestoreDatabase(appBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Verify restored data
					f.VerifyElasticsearchRestore(es, framework.SampleESIndex)
				})
			})
		})

		Context("Passing custom parameters", func() {
			It("should backup only the specified database", func() {
				// Deploy a Elasticsearch instance
				es := f.DeployElasticsearch()

				// Populate the Elasticsearch with some sample data
				f.PopulateElasticsearch(es, framework.SampleESIndex)

				// Backup the database. Pass "--ignoreType=template" as parameter to the backup task.
				appBinding, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
					bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					bc.Spec.Task.Params = []stash_v1beta1.Param{
						{
							Name: framework.ParamKeyArgs,
							// The default params from catalog will be overwritten by this params. Hence, we are including regex for ignoring for both
							// SearchGuard and OpenDistro specific indexes  so that this test work with every ES variants.
							Value: "--match=^(?![.])(?!searchguard|security-auditlog).+ --ignoreType=template",
						},
					}
				})

				// Delete sample data
				f.SimulateElasticsearchDisaster(es, framework.SampleESIndex)

				// Restore the database. Pass "--ignoreType=template" as parameter to the restore task.
				f.RestoreDatabase(appBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
					rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					rs.Spec.Task.Params = []stash_v1beta1.Param{
						{
							Name:  framework.ParamKeyArgs,
							Value: "--ignoreType=template",
						},
					}
				})

				// Verify restored data
				f.VerifyElasticsearchRestore(es, framework.SampleESIndex)
			})
		})

		Context("Using Auto-Backup", func() {
			It("should take backup successfully with default configurations", func() {
				// Deploy a Elasticsearch instance
				es := f.DeployElasticsearch()

				// Populate the Elasticsearch with some sample data
				f.PopulateElasticsearch(es, framework.SampleESIndex)

				// Configure Auto-backup
				backupConfig, repo := f.ConfigureAutoBackup(es, es.ObjectMeta)

				// Simulate a backup run
				f.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)

				// Remove the auto-backup annotations so that Stash does not re-create
				// the auto-backup resources during cleanup step.
				f.RemoveAutoBackupAnnotations(es)
			})

			It("should use schedule from the annotations", func() {
				// Deploy a Elasticsearch instance
				es := f.DeployElasticsearch()

				// Populate the Elasticsearch with some sample data
				f.PopulateElasticsearch(es, framework.SampleESIndex)

				schedule := "0 0 * 11 *"
				// Configure Auto-backup with custom schedule
				backupConfig, _ := f.ConfigureAutoBackup(es, es.ObjectMeta, func(annotations map[string]string) {
					_ = core_util.UpsertMap(annotations, map[string]string{
						stash_v1beta1.KeySchedule: schedule,
					})
				})

				// Verify that the BackupConfiguration is using the custom schedule
				f.VerifyCustomSchedule(backupConfig, schedule)

				// Remove the auto-backup annotations so that Stash does not re-create
				// the auto-backup resources during cleanup step.
				f.RemoveAutoBackupAnnotations(es)
			})

			It("should use custom parameters from the annotations", func() {
				// Deploy a Elasticsearch instance
				es := f.DeployElasticsearch()

				// Populate the Elasticsearch with some sample data
				f.PopulateElasticsearch(es, framework.SampleESIndex)

				args := "--match=^(?![.])(?!searchguard|security-auditlog).+ --ignoreType=template"
				// Configure Auto-backup with custom schedule
				backupConfig, repo := f.ConfigureAutoBackup(es, es.ObjectMeta, func(annotations map[string]string) {
					_ = core_util.UpsertMap(annotations, map[string]string{
						fmt.Sprintf("%s/%s", stash_v1beta1.KeyParams, framework.ParamKeyArgs): args,
					})
				})

				// Simulate a backup run
				f.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, 1)

				// Verify that the BackupConfiguration is using the custom schedule
				f.VerifyParameterPassed(backupConfig.Spec.Task.Params, framework.ParamKeyArgs, args)

				// Remove the auto-backup annotations so that Stash does not re-create
				// the auto-backup resources during cleanup step.
				f.RemoveAutoBackupAnnotations(es)
			})
		})
	})
})
