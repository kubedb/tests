/*
Copyright AppsCode Inc. and Contributors

Licensed under the PolyForm Noncommercial License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/PolyForm-Noncommercial-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e_test

import (
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/tests/e2e/framework"
	"kubedb.dev/tests/e2e/matcher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	stashV1alpha1 "stash.appscode.dev/apimachinery/apis/stash/v1alpha1"
	stashV1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

var _ = Describe("Initialize With Stash", func() {
	// To run this test,
	// 1st: Deploy stash latest operator
	// 2nd: create mongodb related tasks and functions from
	// `kubedb.dev/mongodb/hack/dev/examples/stash01_config.yaml`
	var bc *stashV1beta1.BackupConfiguration
	var rs *stashV1beta1.RestoreSession
	var repo *stashV1alpha1.Repository
	var err error
	to := testOptions{}

	BeforeEach(func() {
		f := framework.NewInvocation()
		to = testOptions{
			Invocation:       f,
			mongodb:          to.MongoDBStandalone(),
			skipMessage:      "",
			garbageMongoDB:   new(api.MongoDBList),
			snapshotPVC:      nil,
			secret:           nil,
			verifySharding:   false,
			enableSharding:   false,
			garbageCASecrets: []*core.Secret{},
			anotherMongoDB:   f.MongoDBStandalone(),
		}
		if to.StorageClass == "" {
			Skip("Missing StorageClassName. Provide as flag to test this.")
		}

		if !to.FoundStashCRDs() {
			Skip("Skipping tests for stash integration. reason: stash operator is not running.")
		}
	})

	AfterEach(func() {
		// Cleanup
		By("Cleanup Left Overs")
		By("Delete left over MongoDB objects")
		to.CleanMongoDB()
		By("Delete left over workloads if exists any")
		to.CleanWorkloadLeftOvers()
		if to.snapshotPVC != nil {
			err := to.DeletePersistentVolumeClaim(to.snapshotPVC.ObjectMeta)
			if err != nil && !kerr.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}

		// Delete test resource
		to.deleteTestResource()

		for _, mg := range to.garbageMongoDB.Items {
			*to.mongodb = mg
			// Delete test resource
			to.deleteTestResource()
		}

		if to.secret != nil {
			err := to.DeleteSecret(to.secret.ObjectMeta)
			if !kerr.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}

		By("Deleting RestoreSession")
		err = to.DeleteRestoreSession(rs.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Deleting Repository")
		err = to.DeleteRepository(repo.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	var createAndWaitForInitializing = func() {
		By("Creating MongoDB: " + to.mongodb.Name)
		err = to.CreateMongoDB(to.mongodb)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Initializing mongodb")
		to.EventuallyMongoDBPhase(to.mongodb.ObjectMeta).Should(Equal(api.DatabasePhaseInitializing))

		By("Wait for AppBinding to create")
		to.EventuallyAppBinding(to.mongodb.ObjectMeta).Should(BeTrue())

		By("Check valid AppBinding Specs")
		err = to.CheckMongoDBAppBindingSpec(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Ping mongodb database")
		to.EventuallyPingMongo(to.mongodb.ObjectMeta)
	}

	var shouldInitializeFromStash = func() {
		// Create and wait for running MongoDB
		if to.mongodb.Spec.SSLMode != api.SSLModeDisabled && to.anotherMongoDB.Spec.SSLMode != api.SSLModeDisabled {
			to.addIssuerRef()
		}

		to.createAndWaitForRunning(true)

		if to.enableSharding {
			By("Enable sharding for db:" + dbName)
			to.EventuallyEnableSharding(to.mongodb.ObjectMeta, dbName).Should(BeTrue())
		}
		if to.verifySharding {
			By("Check if db " + dbName + " is set to partitioned")
			to.EventuallyCollectionPartitioned(to.mongodb.ObjectMeta, dbName).Should(Equal(to.enableSharding))
		}

		By("Insert Document Inside DB")
		to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 25).Should(BeTrue())

		By("Checking Inserted Document")
		to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 25).Should(BeTrue())

		By("Create Secret")
		_, err = to.CreateSecret(to.secret)
		Expect(err).NotTo(HaveOccurred())

		By("Create Repositories")
		err = to.CreateRepository(repo)
		Expect(err).NotTo(HaveOccurred())

		By("Create BackupConfiguration")
		err = to.CreateBackupConfiguration(bc)
		Expect(err).NotTo(HaveOccurred())

		By("Check for snapshot count in stash-repository")
		to.EventuallySnapshotInRepository(repo.ObjectMeta).Should(matcher.MoreThan(2))

		By("Delete BackupConfiguration to stop backup scheduling")
		err = to.DeleteBackupConfiguration(bc.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		oldMongoDB, err := to.GetMongoDB(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		to.garbageMongoDB.Items = append(to.garbageMongoDB.Items, *oldMongoDB)

		By("Create mongodb from stash")
		to.mongodb = to.anotherMongoDB // without value?
		rs = to.RestoreSession(to.mongodb.ObjectMeta, repo)
		to.mongodb.Spec.DatabaseSecret = oldMongoDB.Spec.DatabaseSecret
		to.mongodb.Spec.Init = &api.InitSpec{
			StashRestoreSession: &core.LocalObjectReference{
				Name: rs.Name,
			},
		}
		// Create and wait for running MongoDB
		createAndWaitForInitializing()

		By("Create Stash-RestoreSession")
		err = to.CreateRestoreSession(rs)
		Expect(err).NotTo(HaveOccurred())

		// eventually backupsession succeeded
		By("Check for Succeeded restoreSession")
		to.EventuallyRestoreSessionPhase(rs.ObjectMeta).Should(Equal(stashV1beta1.RestoreSucceeded))

		By("Wait for Running mongodb")
		to.EventuallyMongoDBRunning(to.mongodb.ObjectMeta).Should(BeTrue())

		if to.verifySharding && to.mongodb.Spec.ShardTopology != nil {
			By("Check if db " + dbName + " is set to partitioned")
			to.EventuallyCollectionPartitioned(to.mongodb.ObjectMeta, dbName).Should(Equal(to.enableSharding))
		}

		By("Checking previously Inserted Document")
		to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 25).Should(BeTrue())
	}

	Context("From "+framework.StorageProvider+" backend", func() {

		BeforeEach(func() {
			to.secret = to.SecretForBackend()
			to.secret = to.PatchSecretForRestic(to.secret)
			repo = to.Repository(to.mongodb.ObjectMeta, to.secret.Name)
			bc = to.BackupConfiguration(to.mongodb.ObjectMeta, repo)
		})

		Context("-", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
				to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
			})
			It("should run successfully", shouldInitializeFromStash)
		})

		Context("Standalone with SSL", func() {
			BeforeEach(func() {
				if !framework.SSLEnabled {
					Skip("Enable SSL to test this")
				}
			})
			Context("with requireSSL sslMode", func() {
				BeforeEach(func() {
					to.mongodb.Spec.SSLMode = api.SSLModeRequireSSL
					to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)

					to.anotherMongoDB.Spec.SSLMode = api.SSLModeRequireSSL
					to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
				})

				It("should initialize database successfully", shouldInitializeFromStash)
			})

			Context("with allowSSL sslMode", func() {
				BeforeEach(func() {
					to.mongodb.Spec.SSLMode = api.SSLModeAllowSSL
					to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)

					to.anotherMongoDB.Spec.SSLMode = api.SSLModeAllowSSL
					to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
				})

				It("should initialize database successfully", shouldInitializeFromStash)
			})

			Context("with preferSSL sslMode", func() {
				BeforeEach(func() {
					to.mongodb.Spec.SSLMode = api.SSLModePreferSSL
					to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)

					to.anotherMongoDB.Spec.SSLMode = api.SSLModePreferSSL
					to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
				})

				It("should initialize database successfully", shouldInitializeFromStash)
			})
		})

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.anotherMongoDB = to.MongoDBRS()
				to.secret = to.SecretForBackend()
				to.secret = to.PatchSecretForRestic(to.secret)
				repo = to.Repository(to.mongodb.ObjectMeta, to.secret.Name)
				bc = to.BackupConfiguration(to.mongodb.ObjectMeta, repo)
			})

			Context("-", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
					to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
				})
				It("should take Snapshot successfully", shouldInitializeFromStash)
			})

			Context("with SSL", func() {
				BeforeEach(func() {
					if !framework.SSLEnabled {
						Skip("Enable SSL to test this")
					}
				})
				Context("with requireSSL sslMode", func() {
					BeforeEach(func() {
						to.mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeX509
						to.mongodb.Spec.SSLMode = api.SSLModeRequireSSL
						to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)

						to.anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeX509
						to.anotherMongoDB.Spec.SSLMode = api.SSLModeRequireSSL
						to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
					})

					It("should initialize database successfully", shouldInitializeFromStash)
				})

				Context("with allowSSL sslMode", func() {
					BeforeEach(func() {
						to.mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeKeyFile
						to.mongodb.Spec.SSLMode = api.SSLModeAllowSSL
						to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)

						to.anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeKeyFile
						to.anotherMongoDB.Spec.SSLMode = api.SSLModeAllowSSL
						to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
					})

					It("should initialize database successfully", shouldInitializeFromStash)
				})

				Context("with preferSSL sslMode", func() {
					BeforeEach(func() {
						to.mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeX509
						to.mongodb.Spec.SSLMode = api.SSLModePreferSSL
						to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)

						to.anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeX509
						to.anotherMongoDB.Spec.SSLMode = api.SSLModePreferSSL
						to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
					})

					It("should initialize database successfully", shouldInitializeFromStash)
				})
			})
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.verifySharding = true
				to.anotherMongoDB = to.MongoDBShard()
				to.mongodb = to.MongoDBShard()
				to.secret = to.SecretForBackend()
				to.secret = to.PatchSecretForRestic(to.secret)
				repo = to.Repository(to.mongodb.ObjectMeta, to.secret.Name)
				bc = to.BackupConfiguration(to.mongodb.ObjectMeta, repo)
			})

			Context("With Sharding disabled database", func() {
				BeforeEach(func() {
					to.enableSharding = false
					to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
					to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
				})

				It("should initialize database successfully", shouldInitializeFromStash)
			})

			Context("With Sharding Enabled database", func() {
				BeforeEach(func() {
					to.enableSharding = true
					to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
					to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
				})

				It("should initialize database successfully", shouldInitializeFromStash)
			})

			Context("with SSL", func() {
				BeforeEach(func() {
					to.enableSharding = true
					if !framework.SSLEnabled {
						Skip("Enable SSL to test this")
					}
				})

				Context("with requireSSL sslMode", func() {
					BeforeEach(func() {
						to.mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeX509
						to.mongodb.Spec.SSLMode = api.SSLModeRequireSSL
						to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)

						to.anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeX509
						to.anotherMongoDB.Spec.SSLMode = api.SSLModeRequireSSL
						to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
					})

					It("should initialize database successfully", shouldInitializeFromStash)
				})

				Context("with allowSSL sslMode", func() {
					BeforeEach(func() {
						to.mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeKeyFile
						to.mongodb.Spec.SSLMode = api.SSLModeAllowSSL
						to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)

						to.anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeKeyFile
						to.anotherMongoDB.Spec.SSLMode = api.SSLModeAllowSSL
						to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
					})

					It("should initialize database successfully", shouldInitializeFromStash)
				})

				Context("with preferSSL sslMode", func() {
					BeforeEach(func() {
						to.mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeX509
						to.mongodb.Spec.SSLMode = api.SSLModePreferSSL
						to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)

						to.anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeX509
						to.anotherMongoDB.Spec.SSLMode = api.SSLModePreferSSL
						to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
					})

					It("should initialize database successfully", shouldInitializeFromStash)
				})
			})
		})

		Context("From Sharding to standalone", func() {
			var customAppBindingName string
			BeforeEach(func() {
				to.enableSharding = true
				to.mongodb = to.MongoDBShard()
				to.anotherMongoDB = to.MongoDBStandalone()
				customAppBindingName = to.mongodb.Name + "custom"
				repo = to.Repository(to.mongodb.ObjectMeta, to.secret.Name)
				bc = to.BackupConfiguration(to.mongodb.ObjectMeta, repo)

				to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
				to.anotherMongoDB = to.MongoDBWithFlexibleProbeTimeout(to.anotherMongoDB)
			})

			AfterEach(func() {
				By("Deleting custom AppBindig")
				err = to.DeleteAppBinding(metav1.ObjectMeta{Name: customAppBindingName, Namespace: to.mongodb.Namespace})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should take Snapshot successfully", func() {
				// Create and wait for running MongoDB
				to.createAndWaitForRunning()

				if to.enableSharding {
					By("Enable sharding for db:" + dbName)
					to.EventuallyEnableSharding(to.mongodb.ObjectMeta, dbName).Should(BeTrue())
				}
				if to.verifySharding {
					By("Check if db " + dbName + " is set to partitioned")
					to.EventuallyCollectionPartitioned(to.mongodb.ObjectMeta, dbName).Should(Equal(to.enableSharding))
				}

				By("Insert Document Inside DB")
				to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 50).Should(BeTrue())

				By("Checking Inserted Document")
				to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 50).Should(BeTrue())

				err = to.EnsureCustomAppBinding(to.mongodb, customAppBindingName)
				Expect(err).NotTo(HaveOccurred())

				By("Create Secret")
				_, err = to.CreateSecret(to.secret)
				Expect(err).NotTo(HaveOccurred())

				By("Create Repositories")
				err = to.CreateRepository(repo)
				Expect(err).NotTo(HaveOccurred())

				bc.Spec.Target.Ref.Name = customAppBindingName

				By("Create BackupConfiguration")
				err = to.CreateBackupConfiguration(bc)
				Expect(err).NotTo(HaveOccurred())

				By("Check for snapshot count in stash-repository")
				to.EventuallySnapshotInRepository(repo.ObjectMeta).Should(matcher.MoreThan(2))

				By("Delete BackupConfiguration to stop backup scheduling")
				err = to.DeleteBackupConfiguration(bc.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				oldMongoDB, err := to.GetMongoDB(to.mongodb.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				to.garbageMongoDB.Items = append(to.garbageMongoDB.Items, *oldMongoDB)

				By("Create mongodb from stash")
				to.mongodb = to.anotherMongoDB // without value?
				rs = to.RestoreSession(to.mongodb.ObjectMeta, repo)
				to.mongodb.Spec.DatabaseSecret = oldMongoDB.Spec.DatabaseSecret
				to.mongodb.Spec.Init = &api.InitSpec{
					StashRestoreSession: &core.LocalObjectReference{
						Name: rs.Name,
					},
				}

				// Create and wait for running MongoDB
				createAndWaitForInitializing()

				By("Create RestoreSession")
				err = to.CreateRestoreSession(rs)
				Expect(err).NotTo(HaveOccurred())

				// eventually backupsession succeeded
				By("Check for Succeeded restoreSession")
				to.EventuallyRestoreSessionPhase(rs.ObjectMeta).Should(Equal(stashV1beta1.RestoreSucceeded))

				By("Wait for Running mongodb")
				to.EventuallyMongoDBRunning(to.mongodb.ObjectMeta).Should(BeTrue())

				if to.verifySharding && to.mongodb.Spec.ShardTopology != nil {
					By("Check if db " + dbName + " is set to partitioned")
					to.EventuallyCollectionPartitioned(to.mongodb.ObjectMeta, dbName).Should(Equal(to.enableSharding))
				}

				By("Checking previously Inserted Document")
				to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 50).Should(BeTrue())
			})
		})
	})
})
