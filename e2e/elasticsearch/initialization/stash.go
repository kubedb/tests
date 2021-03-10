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

package initialization

import (
	"fmt"

	catalog "kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	stash_v1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

var _ = Describe("Elasticsearch Initialization", func() {
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

	Context("From Stash Backup", func() {
		Context("Of Elasticsearch with combined nodes", func() {
			Context("Into another Elasticsearch with combined nodes", func() {
				It("should complete successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch()

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					_, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete previous Elasticsearch so that new Elasticsearch can start in hardware with limited CPU & Memory
					By("Deleting source Elasticsearch: " + es.Name)
					err := f.DeleteElasticsearch(es.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another Elasticsearch instance
					dstES := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
					})

					// Get the AppBinding of new Elasticsearch
					f.EventuallyAppBinding(dstES.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstES.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					By("Verify that database is ready after restore")
					f.EventuallyElasticsearchReady(dstES.ObjectMeta).Should(BeTrue())

					// Verify restored data
					f.VerifyElasticsearchRestore(dstES, framework.SampleESIndex)
				})
			})

			Context("Into Elasticsearch with dedicated nodes", func() {
				It("should complete successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch()

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					_, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete previous Elasticsearch so that new Elasticsearch can start in hardware with limited CPU & Memory
					By("Deleting source Elasticsearch: " + es.Name)
					err := f.DeleteElasticsearch(es.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another Elasticsearch instance
					dstES := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
						f.AddDedicatedESNodes(in)
					})

					// Get the AppBinding of new Elasticsearch
					f.EventuallyAppBinding(dstES.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstES.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					By("Verify that database is ready after restore")
					f.EventuallyElasticsearchReady(dstES.ObjectMeta).Should(BeTrue())

					// Verify restored data
					f.VerifyElasticsearchRestore(dstES, framework.SampleESIndex)
				})
			})
		})

		Context("Of Elasticsearch with dedicated nodes", func() {
			Context("Into Elasticsearch with combined nodes", func() {
				It("should complete successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						f.AddDedicatedESNodes(in)
					})

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					_, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete previous Elasticsearch so that new Elasticsearch can start in hardware with limited CPU & Memory
					By("Deleting source Elasticsearch: " + es.Name)
					err := f.DeleteElasticsearch(es.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another Elasticsearch instance
					dstES := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
					})

					// Get the AppBinding of new Elasticsearch
					f.EventuallyAppBinding(dstES.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstES.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					By("Verify that database is ready after restore")
					f.EventuallyElasticsearchReady(dstES.ObjectMeta).Should(BeTrue())

					// Verify restored data
					f.VerifyElasticsearchRestore(dstES, framework.SampleESIndex)
				})
			})

			Context("Into another Elasticsearch with dedicated nodes", func() {
				It("should complete successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						f.AddDedicatedESNodes(in)
					})

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					_, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete previous Elasticsearch so that new Elasticsearch can start in hardware with limited CPU & Memory
					By("Deleting source Elasticsearch: " + es.Name)
					err := f.DeleteElasticsearch(es.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another Elasticsearch instance
					dstES := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
						f.AddDedicatedESNodes(in)
					})

					// Get the AppBinding of new Elasticsearch
					f.EventuallyAppBinding(dstES.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstES.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					By("Verify that database is ready after restore")
					f.EventuallyElasticsearchReady(dstES.ObjectMeta).Should(BeTrue())

					// Verify restored data
					f.VerifyElasticsearchRestore(dstES, framework.SampleESIndex)
				})
			})
		})

		Context("Of ElasticStack variant", func() {
			Context("Into SearchGuard variant", func() {
				It("should complete successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroElasticStack)
					})

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					_, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete previous Elasticsearch so that new Elasticsearch can start in hardware with limited CPU & Memory
					By("Deleting source Elasticsearch: " + es.Name)
					err := f.DeleteElasticsearch(es.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another Elasticsearch instance
					dstES := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroSearchGuard)
					})

					// Get the AppBinding of new Elasticsearch
					f.EventuallyAppBinding(dstES.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstES.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						rs.Spec.Task.Params = []stash_v1beta1.Param{
							{
								Name:  framework.ParamKeyArgs,
								Value: "--ignoreType=template,settings",
							},
						}
					})

					By("Verify that database is ready after restore")
					f.EventuallyElasticsearchReady(dstES.ObjectMeta).Should(BeTrue())

					// Verify restored data
					f.VerifyElasticsearchRestore(dstES, framework.SampleESIndex)
				})
			})

			Context("Into OpenDistro variant", func() {
				It("should complete successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroElasticStack)
					})

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					_, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete previous Elasticsearch so that new Elasticsearch can start in hardware with limited CPU & Memory
					By("Deleting source Elasticsearch: " + es.Name)
					err := f.DeleteElasticsearch(es.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another Elasticsearch instance
					dstES := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroOpenDistro)
					})

					// Get the AppBinding of new Elasticsearch
					f.EventuallyAppBinding(dstES.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstES.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						rs.Spec.Task.Params = []stash_v1beta1.Param{
							{
								Name:  framework.ParamKeyArgs,
								Value: "--ignoreType=template,settings",
							},
						}
					})

					By("Verify that database is ready after restore")
					f.EventuallyElasticsearchReady(dstES.ObjectMeta).Should(BeTrue())

					// Verify restored data
					f.VerifyElasticsearchRestore(dstES, framework.SampleESIndex)
				})
			})
		})

		Context("Of SearchGuard variant", func() {
			Context("Into ElasticStack variant", func() {
				It("should complete successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroSearchGuard)
					})

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					_, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete previous Elasticsearch so that new Elasticsearch can start in hardware with limited CPU & Memory
					By("Deleting source Elasticsearch: " + es.Name)
					err := f.DeleteElasticsearch(es.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another Elasticsearch instance
					dstES := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroElasticStack)
					})

					// Get the AppBinding of new Elasticsearch
					f.EventuallyAppBinding(dstES.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstES.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						rs.Spec.Task.Params = []stash_v1beta1.Param{
							{
								Name:  framework.ParamKeyArgs,
								Value: "--ignoreType=template,settings",
							},
						}
					})

					By("Verify that database is ready after restore")
					f.EventuallyElasticsearchReady(dstES.ObjectMeta).Should(BeTrue())

					// Verify restored data
					f.VerifyElasticsearchRestore(dstES, framework.SampleESIndex)
				})
			})

			Context("Into OpenDistro variant", func() {
				It("should complete successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroSearchGuard)
					})

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					_, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete previous Elasticsearch so that new Elasticsearch can start in hardware with limited CPU & Memory
					By("Deleting source Elasticsearch: " + es.Name)
					err := f.DeleteElasticsearch(es.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another Elasticsearch instance
					dstES := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroOpenDistro)
					})

					// Get the AppBinding of new Elasticsearch
					f.EventuallyAppBinding(dstES.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstES.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						rs.Spec.Task.Params = []stash_v1beta1.Param{
							{
								Name:  framework.ParamKeyArgs,
								Value: "--ignoreType=template,settings",
							},
						}
					})

					By("Verify that database is ready after restore")
					f.EventuallyElasticsearchReady(dstES.ObjectMeta).Should(BeTrue())

					// Verify restored data
					f.VerifyElasticsearchRestore(dstES, framework.SampleESIndex)
				})
			})
		})

		Context("Of OpenDistro variant", func() {
			Context("Into ElasticStack variant", func() {
				It("should complete successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroOpenDistro)
					})

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					_, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete previous Elasticsearch so that new Elasticsearch can start in hardware with limited CPU & Memory
					By("Deleting source Elasticsearch: " + es.Name)
					err := f.DeleteElasticsearch(es.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another Elasticsearch instance
					dstES := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroElasticStack)
					})

					// Get the AppBinding of new Elasticsearch
					f.EventuallyAppBinding(dstES.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstES.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						rs.Spec.Task.Params = []stash_v1beta1.Param{
							{
								Name:  framework.ParamKeyArgs,
								Value: "--ignoreType=template,settings",
							},
						}
					})

					By("Verify that database is ready after restore")
					f.EventuallyElasticsearchReady(dstES.ObjectMeta).Should(BeTrue())

					// Verify restored data
					f.VerifyElasticsearchRestore(dstES, framework.SampleESIndex)
				})
			})

			Context("Into SearchGuard variant", func() {
				It("should complete successfully", func() {
					// Deploy a Elasticsearch instance
					es := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroOpenDistro)
					})

					// Populate the Elasticsearch with some sample data
					f.PopulateElasticsearch(es, framework.SampleESIndex)

					// Backup the database
					_, repo := f.BackupDatabase(es.ObjectMeta, 1, func(bc *stash_v1beta1.BackupConfiguration) {
						bc.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
					})

					// Delete previous Elasticsearch so that new Elasticsearch can start in hardware with limited CPU & Memory
					By("Deleting source Elasticsearch: " + es.Name)
					err := f.DeleteElasticsearch(es.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another Elasticsearch instance
					dstES := f.DeployElasticsearch(func(in *api.Elasticsearch) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
						in.Spec.Version = f.NearestESVariant(catalog.ElasticsearchDistroSearchGuard)
					})

					// Get the AppBinding of new Elasticsearch
					f.EventuallyAppBinding(dstES.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstES.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo, func(rs *stash_v1beta1.RestoreSession) {
						rs.Spec.InterimVolumeTemplate = f.NewInterimVolumeTemplate()
						rs.Spec.Task.Params = []stash_v1beta1.Param{
							{
								Name:  framework.ParamKeyArgs,
								Value: "--ignoreType=template,settings",
							},
						}
					})

					By("Verify that database is ready after restore")
					f.EventuallyElasticsearchReady(dstES.ObjectMeta).Should(BeTrue())

					// Verify restored data
					f.VerifyElasticsearchRestore(dstES, framework.SampleESIndex)
				})
			})
		})
	})
})
