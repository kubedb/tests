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

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MongoDB Initialization", func() {
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

	Context("From Stash Backup", func() {
		Context("Of Standalone MongoDB", func() {
			Context("Into Another Standalone MongoDB", func() {
				It("should complete successfully", func() {
					// Deploy a MongoDB instance
					mg := f.DeployMongoDB()

					// Populate the MongoDB with some sample data
					By("Populating MongoDB with sample data")
					f.EventuallyInsertDocument(mg.ObjectMeta, framework.SampleDB, 50)

					// Backup the database
					_, repo := f.BackupDatabase(mg.ObjectMeta, 1)

					// Delete previous MongoDB so that new MongoDB can start in hardware with limited CPU & Memory
					By("Deleting source MongoDB: " + mg.Name)
					err := f.DeleteMongoDB(mg.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another MongoDB instance
					dstMongo := f.DeployMongoDB(func(in *api.MongoDB) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
					})

					// Get the AppBinding of new mongoDB
					f.EventuallyAppBinding(dstMongo.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstMongo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo)

					By("Verify that database is healthy after restore")
					f.EventuallyMongoDBRunning(dstMongo.ObjectMeta).Should(BeTrue())

					// Verify restored data
					By("Verifying that the data has been restored from backup")
					f.EventuallyDocumentExists(dstMongo.ObjectMeta, framework.SampleDB, 50)
				})
			})

			Context("Into MongoDB ReplicaSet", func() {
				It("should complete successfully", func() {
					// Deploy a MongoDB instance
					mg := f.DeployMongoDB()

					// Populate the MongoDB with some sample data
					By("Populating MongoDB with sample data")
					f.EventuallyInsertDocument(mg.ObjectMeta, framework.SampleDB, 50)

					// Backup the database
					_, repo := f.BackupDatabase(mg.ObjectMeta, 1)

					// Delete previous MongoDB so that new MongoDB can start in hardware with limited CPU & Memory
					By("Deleting source MongoDB: " + mg.Name)
					err := f.DeleteMongoDB(mg.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another MongoDB instance
					dstMongo := f.DeployMongoDB(func(in *api.MongoDB) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						f.EnableMongoReplication(in)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
					})

					// Get the AppBinding of new mongoDB
					f.EventuallyAppBinding(dstMongo.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstMongo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Since our backup was taken from a standalone MongoDB, we have to use custom AppBinding to restore.
					// Otherwise, Stash will assume the backup was taken from ReplicaSet and restore will fail.
					customAppBinding, err := f.CreateCustomAppBinding(dstAppBinding)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(customAppBinding, repo)

					By("Verify that database is healthy after restore")
					f.EventuallyMongoDBRunning(dstMongo.ObjectMeta).Should(BeTrue())

					// Verify restored data
					By("Verifying that the data has been restored from backup")
					f.EventuallyDocumentExists(dstMongo.ObjectMeta, framework.SampleDB, 50)
				})
			})

			Context("Into Sharded MongoDB", func() {
				It("should complete successfully", func() {
					// Deploy a MongoDB instance
					mg := f.DeployMongoDB()

					// Populate the MongoDB with some sample data
					By("Populating MongoDB with sample data")
					f.EventuallyInsertDocument(mg.ObjectMeta, framework.SampleDB, 50)

					// Backup the database
					_, repo := f.BackupDatabase(mg.ObjectMeta, 1)

					// Delete previous MongoDB so that new MongoDB can start in hardware with limited CPU & Memory
					By("Deleting source MongoDB: " + mg.Name)
					err := f.DeleteMongoDB(mg.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another MongoDB instance
					dstMongo := f.DeployMongoDB(func(in *api.MongoDB) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						f.EnableMongoSharding(in)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
					})

					// Get the AppBinding of new mongoDB
					f.EventuallyAppBinding(dstMongo.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstMongo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Since our backup was taken from a standalone MongoDB, we have to use custom AppBinding to restore.
					// Otherwise, Stash will assume the backup was taken from ReplicaSet and restore will fail.
					customAppBinding, err := f.CreateCustomAppBinding(dstAppBinding)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(customAppBinding, repo)

					By("Verify that database is healthy after restore")
					f.EventuallyMongoDBRunning(dstMongo.ObjectMeta).Should(BeTrue())

					// Verify restored data
					By("Verifying that the data has been restored from backup")
					f.EventuallyDocumentExists(dstMongo.ObjectMeta, framework.SampleDB, 50)
				})
			})
		})

		Context("Of MongoDB ReplicaSet", func() {
			Context("Into Standalone MongoDB", func() {
				It("should complete successfully", func() {
					// Deploy a MongoDB instance
					mg := f.DeployMongoDB(func(in *api.MongoDB) {
						f.EnableMongoReplication(in)
					})

					// Populate the MongoDB with some sample data
					By("Populating MongoDB with sample data")
					f.EventuallyInsertDocument(mg.ObjectMeta, framework.SampleDB, 50)

					// Backup the database
					_, repo := f.BackupDatabase(mg.ObjectMeta, 1)

					// Delete previous MongoDB so that new MongoDB can start in hardware with limited CPU & Memory
					By("Deleting source MongoDB: " + mg.Name)
					err := f.DeleteMongoDB(mg.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another MongoDB instance
					dstMongo := f.DeployMongoDB(func(in *api.MongoDB) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
					})

					// Get the AppBinding of new mongoDB
					f.EventuallyAppBinding(dstMongo.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstMongo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo)

					By("Verify that database is healthy after restore")
					f.EventuallyMongoDBRunning(dstMongo.ObjectMeta).Should(BeTrue())

					// Verify restored data
					By("Verifying that the data has been restored from backup")
					f.EventuallyDocumentExists(dstMongo.ObjectMeta, framework.SampleDB, 50)
				})
			})

			Context("Into another MongoDB ReplicaSet", func() {
				It("should complete successfully", func() {
					// Deploy a MongoDB instance
					mg := f.DeployMongoDB(func(in *api.MongoDB) {
						f.EnableMongoReplication(in)
					})

					// Populate the MongoDB with some sample data
					By("Populating MongoDB with sample data")
					f.EventuallyInsertDocument(mg.ObjectMeta, framework.SampleDB, 50)

					// Backup the database
					_, repo := f.BackupDatabase(mg.ObjectMeta, 1)

					// Delete previous MongoDB so that new MongoDB can start in hardware with limited CPU & Memory
					By("Deleting source MongoDB: " + mg.Name)
					err := f.DeleteMongoDB(mg.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another MongoDB instance
					dstMongo := f.DeployMongoDB(func(in *api.MongoDB) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						f.EnableMongoReplication(in)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
					})

					// Get the AppBinding of new mongoDB
					f.EventuallyAppBinding(dstMongo.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstMongo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo)

					By("Verify that database is healthy after restore")
					f.EventuallyMongoDBRunning(dstMongo.ObjectMeta).Should(BeTrue())

					// Verify restored data
					By("Verifying that the data has been restored from backup")
					f.EventuallyDocumentExists(dstMongo.ObjectMeta, framework.SampleDB, 50)
				})
			})

			Context("Into Sharded MongoDB", func() {
				It("should complete successfully", func() {
					// Deploy a MongoDB instance
					mg := f.DeployMongoDB(func(in *api.MongoDB) {
						f.EnableMongoReplication(in)
					})

					// Populate the MongoDB with some sample data
					By("Populating MongoDB with sample data")
					f.EventuallyInsertDocument(mg.ObjectMeta, framework.SampleDB, 50)

					// Backup the database
					_, repo := f.BackupDatabase(mg.ObjectMeta, 1)

					// Delete previous MongoDB so that new MongoDB can start in hardware with limited CPU & Memory
					By("Deleting source MongoDB: " + mg.Name)
					err := f.DeleteMongoDB(mg.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another MongoDB instance
					dstMongo := f.DeployMongoDB(func(in *api.MongoDB) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						f.EnableMongoSharding(in)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
					})

					// Get the AppBinding of new mongoDB
					f.EventuallyAppBinding(dstMongo.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstMongo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Since our backup was taken from a standalone MongoDB, we have to use custom AppBinding to restore.
					// Otherwise, Stash will assume the backup was taken from ReplicaSet and restore will fail.
					customAppBinding, err := f.CreateCustomAppBinding(dstAppBinding)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(customAppBinding, repo)

					By("Verify that database is healthy after restore")
					f.EventuallyMongoDBRunning(dstMongo.ObjectMeta).Should(BeTrue())

					// Verify restored data
					By("Verifying that the data has been restored from backup")
					f.EventuallyDocumentExists(dstMongo.ObjectMeta, framework.SampleDB, 50)
				})
			})
		})

		Context("Of Sharded MongoDB", func() {
			Context("Into Standalone MongoDB", func() {
				It("should complete successfully", func() {
					// Deploy a MongoDB instance
					mg := f.DeployMongoDB(func(in *api.MongoDB) {
						f.EnableMongoSharding(in)
					})

					// Populate the MongoDB with some sample data
					By("Populating MongoDB with sample data")
					f.EventuallyInsertDocument(mg.ObjectMeta, framework.SampleDB, 50)

					// Backup the database
					_, repo := f.BackupDatabase(mg.ObjectMeta, 3)

					// Delete previous MongoDB so that new MongoDB can start in hardware with limited CPU & Memory
					By("Deleting source MongoDB: " + mg.Name)
					err := f.DeleteMongoDB(mg.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another MongoDB instance
					dstMongo := f.DeployMongoDB(func(in *api.MongoDB) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
					})

					// Get the AppBinding of new mongoDB
					f.EventuallyAppBinding(dstMongo.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstMongo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo)

					By("Verify that database is healthy after restore")
					f.EventuallyMongoDBRunning(dstMongo.ObjectMeta).Should(BeTrue())

					// Verify restored data
					By("Verifying that the data has been restored from backup")
					f.EventuallyDocumentExists(dstMongo.ObjectMeta, framework.SampleDB, 50)
				})
			})

			Context("Into MongoDB ReplicaSet", func() {
				It("should complete successfully", func() {
					// Deploy a MongoDB instance
					mg := f.DeployMongoDB(func(in *api.MongoDB) {
						f.EnableMongoSharding(in)
					})

					// Populate the MongoDB with some sample data
					By("Populating MongoDB with sample data")
					f.EventuallyInsertDocument(mg.ObjectMeta, framework.SampleDB, 50)

					// Backup the database
					_, repo := f.BackupDatabase(mg.ObjectMeta, 3)

					// Delete previous MongoDB so that new MongoDB can start in hardware with limited CPU & Memory
					By("Deleting source MongoDB: " + mg.Name)
					err := f.DeleteMongoDB(mg.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another MongoDB instance
					dstMongo := f.DeployMongoDB(func(in *api.MongoDB) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						f.EnableMongoReplication(in)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
					})

					// Get the AppBinding of new mongoDB
					f.EventuallyAppBinding(dstMongo.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstMongo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Since our backup was taken from a standalone MongoDB, we have to use custom AppBinding to restore.
					// Otherwise, Stash will assume the backup was taken from ReplicaSet and restore will fail.
					customAppBinding, err := f.CreateCustomAppBinding(dstAppBinding)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(customAppBinding, repo)

					By("Verify that database is healthy after restore")
					f.EventuallyMongoDBRunning(dstMongo.ObjectMeta).Should(BeTrue())

					// Verify restored data
					By("Verifying that the data has been restored from backup")
					f.EventuallyDocumentExists(dstMongo.ObjectMeta, framework.SampleDB, 50)
				})
			})

			Context("Into another Sharded MongoDB", func() {
				It("should complete successfully", func() {
					// Deploy a MongoDB instance
					mg := f.DeployMongoDB(func(in *api.MongoDB) {
						f.EnableMongoSharding(in)
					})

					// Populate the MongoDB with some sample data
					By("Populating MongoDB with sample data")
					f.EventuallyInsertDocument(mg.ObjectMeta, framework.SampleDB, 50)

					// Backup the database
					_, repo := f.BackupDatabase(mg.ObjectMeta, 3)

					// Delete previous MongoDB so that new MongoDB can start in hardware with limited CPU & Memory
					By("Deleting source MongoDB: " + mg.Name)
					err := f.DeleteMongoDB(mg.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Deploy another MongoDB instance
					dstMongo := f.DeployMongoDB(func(in *api.MongoDB) {
						in.Name = fmt.Sprintf("%s-restored", in.Name)
						f.EnableMongoSharding(in)
						in.Spec.Init = f.WaitForInitialRestore(in.Spec.Init)
					})

					// Get the AppBinding of new mongoDB
					f.EventuallyAppBinding(dstMongo.ObjectMeta).Should(BeTrue())
					dstAppBinding, err := f.GetAppBinding(dstMongo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Restore the database
					f.RestoreDatabase(dstAppBinding, repo)

					By("Verify that database is healthy after restore")
					f.EventuallyMongoDBRunning(dstMongo.ObjectMeta).Should(BeTrue())

					// Verify restored data
					By("Verifying that the data has been restored from backup")
					f.EventuallyDocumentExists(dstMongo.ObjectMeta, framework.SampleDB, 50)
				})
			})
		})
	})
})
