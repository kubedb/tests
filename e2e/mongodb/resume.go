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

package e2e_test

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	kmapi "kmodules.xyz/client-go/api/v1"
)

var _ = Describe("Resume", func() {
	var err error
	to := testOptions{}
	testName := framework.Resume

	BeforeEach(func() {
		f := framework.NewInvocation()
		to = testOptions{
			Invocation:       f,
			mongodb:          f.MongoDBStandalone(),
			skipMessage:      "",
			garbageMongoDB:   new(api.MongoDBList),
			snapshotPVC:      nil,
			secret:           nil,
			verifySharding:   false,
			enableSharding:   false,
			garbageCASecrets: []*core.Secret{},
		}
		if to.StorageClass == "" {
			Skip("Missing StorageClassName. Provide as flag to test this.")
		}
		if !runTestCommunity(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` to test this.", testName))
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
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	var usedInitScript bool
	BeforeEach(func() {
		usedInitScript = false
	})

	Context("Super Fast User - Create-Delete-Create-Delete-Create ", func() {
		It("should resume DormantDatabase successfully", func() {
			// Create and wait for running MongoDB
			to.createAndWaitForRunning()

			By("Insert Document Inside DB")
			to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

			By("Delete mongodb")
			err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for mongodb to be deleted")
			to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

			// Create MongoDB object again to resume it
			By("Create MongoDB: " + to.mongodb.Name)
			err = to.CreateMongoDB(to.mongodb)
			Expect(err).NotTo(HaveOccurred())

			// Delete without caring if DB is resumed
			By("Delete mongodb")
			err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for MongoDB to be deleted")
			to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

			// Create MongoDB object again to resume it
			By("Create MongoDB: " + to.mongodb.Name)
			err = to.CreateMongoDB(to.mongodb)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for Running mongodb")
			to.EventuallyMongoDBRunning(to.mongodb.ObjectMeta).Should(BeTrue())

			By("Ping mongodb database")
			to.EventuallyPingMongo(to.mongodb.ObjectMeta)

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

			_, err = to.GetMongoDB(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Without Init", func() {

		var shouldResumeWithoutInit = func() {
			// Create and wait for running MongoDB
			to.createAndWaitForRunning()

			By("Insert Document Inside DB")
			to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

			By("Delete mongodb")
			err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for mongodb to be paused")
			to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

			// Create MongoDB object again to resume it
			By("Create MongoDB: " + to.mongodb.Name)
			err = to.CreateMongoDB(to.mongodb)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for Running mongodb")
			to.EventuallyMongoDBRunning(to.mongodb.ObjectMeta).Should(BeTrue())

			By("Ping mongodb database")
			to.EventuallyPingMongo(to.mongodb.ObjectMeta)

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

			_, err = to.GetMongoDB(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		}

		It("should resume DormantDatabase successfully", shouldResumeWithoutInit)

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
			})
			It("should take Snapshot successfully", shouldResumeWithoutInit)
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
			})
			It("should take Snapshot successfully", shouldResumeWithoutInit)
		})
	})

	Context("with init Script", func() {
		var configMap *core.ConfigMap

		BeforeEach(func() {
			usedInitScript = true

			configMap = to.ConfigMapForInitialization()
			err := to.CreateConfigMap(configMap)
			Expect(err).NotTo(HaveOccurred())

			to.mongodb.Spec.Init = &api.InitSpec{
				Script: &api.ScriptSourceSpec{
					VolumeSource: core.VolumeSource{
						ConfigMap: &core.ConfigMapVolumeSource{
							LocalObjectReference: core.LocalObjectReference{
								Name: configMap.Name,
							},
						},
					},
				},
			}
		})

		AfterEach(func() {
			err := to.DeleteConfigMap(configMap.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})

		var shouldResumeWithInit = func() {
			// Create and wait for running MongoDB
			to.createAndWaitForRunning()

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

			By("Delete mongodb")
			err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for mongodb to be paused")
			to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

			// Create MongoDB object again to resume it
			By("Create MongoDB: " + to.mongodb.Name)
			err = to.CreateMongoDB(to.mongodb)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for Running mongodb")
			to.EventuallyMongoDBRunning(to.mongodb.ObjectMeta).Should(BeTrue())

			By("Ping mongodb database")
			to.EventuallyPingMongo(to.mongodb.ObjectMeta)

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

			mg, err := to.GetMongoDB(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			*to.mongodb = *mg
			if usedInitScript {
				Expect(to.mongodb.Spec.Init).ShouldNot(BeNil())
				Expect(kmapi.HasCondition(to.mongodb.Status.Conditions, api.DatabaseDataRestored)).To(BeFalse())
			}
		}

		It("should resume DormantDatabase successfully", shouldResumeWithInit)

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Init = &api.InitSpec{
					Script: &api.ScriptSourceSpec{
						VolumeSource: core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: configMap.Name,
								},
							},
						},
					},
				}
			})
			It("should take Snapshot successfully", shouldResumeWithInit)
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Init = &api.InitSpec{
					Script: &api.ScriptSourceSpec{
						VolumeSource: core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: configMap.Name,
								},
							},
						},
					},
				}
				to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
			})
			It("should take Snapshot successfully", shouldResumeWithInit)
		})
	})

	Context("Multiple times with init script", func() {
		var configMap *core.ConfigMap

		BeforeEach(func() {
			usedInitScript = true

			configMap = to.ConfigMapForInitialization()
			err := to.CreateConfigMap(configMap)
			Expect(err).NotTo(HaveOccurred())

			to.mongodb.Spec.Init = &api.InitSpec{
				Script: &api.ScriptSourceSpec{
					VolumeSource: core.VolumeSource{
						ConfigMap: &core.ConfigMapVolumeSource{
							LocalObjectReference: core.LocalObjectReference{
								Name: configMap.Name,
							},
						},
					},
				},
			}
		})

		AfterEach(func() {
			err := to.DeleteConfigMap(configMap.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})

		var shouldResumeMultipleTimes = func() {
			// Create and wait for running MongoDB
			to.createAndWaitForRunning()

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

			for i := 0; i < 3; i++ {
				By(fmt.Sprintf("%v-th", i+1) + " time running.")
				By("Delete mongodb")
				err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Wait for mongodb to be paused")
				to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

				// Create MongoDB object again to resume it
				By("Create MongoDB: " + to.mongodb.Name)
				err = to.CreateMongoDB(to.mongodb)
				Expect(err).NotTo(HaveOccurred())

				By("Wait for Running mongodb")
				to.EventuallyMongoDBRunning(to.mongodb.ObjectMeta).Should(BeTrue())

				_, err := to.GetMongoDB(to.mongodb.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Ping mongodb database")
				to.EventuallyPingMongo(to.mongodb.ObjectMeta)

				By("Checking Inserted Document")
				to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

				if usedInitScript {
					Expect(to.mongodb.Spec.Init).ShouldNot(BeNil())
					Expect(kmapi.HasCondition(to.mongodb.Status.Conditions, api.DatabaseDataRestored)).To(BeFalse())
				}
			}
		}

		It("should resume DormantDatabase successfully", shouldResumeMultipleTimes)

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Init = &api.InitSpec{
					Script: &api.ScriptSourceSpec{
						VolumeSource: core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: configMap.Name,
								},
							},
						},
					},
				}
			})
			It("should take Snapshot successfully", shouldResumeMultipleTimes)
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Init = &api.InitSpec{
					Script: &api.ScriptSourceSpec{
						VolumeSource: core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: configMap.Name,
								},
							},
						},
					},
				}
				to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
			})
			It("should take Snapshot successfully", shouldResumeMultipleTimes)
		})
	})

})
