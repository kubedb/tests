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

	"github.com/appscode/go/log"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	ofst "kmodules.xyz/offshoot-api/api/v1"
)

var _ = Describe("General MongoDB", func() {
	var err error
	to := testOptions{}
	testName := framework.General

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

	Context("With PVC - Halt", func() {
		var shouldRunWithPVC = func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}
			// Create MongoDB
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
			to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

			By("Halt MongoDB: Update mongodb to set spec.halted = true")
			_, err := to.PatchMongoDB(to.mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
				in.Spec.Halted = true
				return in
			})
			Expect(err).NotTo(HaveOccurred())

			By("Wait for halted/paused mongodb")
			to.EventuallyMongoDBPhase(to.mongodb.ObjectMeta).Should(Equal(api.DatabasePhaseHalted))

			By("Resume MongoDB: Update mongodb to set spec.halted = false")
			_, err = to.PatchMongoDB(to.mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
				in.Spec.Halted = false
				return in
			})
			Expect(err).NotTo(HaveOccurred())

			By("Wait for Running mongodb")
			to.EventuallyMongoDBRunning(to.mongodb.ObjectMeta).Should(BeTrue())

			By("Ping mongodb database")
			to.EventuallyPingMongo(to.mongodb.ObjectMeta)

			if to.verifySharding {
				By("Check if db " + dbName + " is set to partitioned")
				to.EventuallyCollectionPartitioned(to.mongodb.ObjectMeta, dbName).Should(Equal(to.enableSharding))
			}

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())
		}

		It("should run successfully", shouldRunWithPVC)

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Replicas = types.Int32P(3)
			})
			It("should run successfully", shouldRunWithPVC)
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.verifySharding = true
				to.mongodb = to.MongoDBShard()
			})

			Context("-", func() {
				BeforeEach(func() {
					to.enableSharding = false
				})
				It("should run successfully", shouldRunWithPVC)
			})

			Context("With Sharding Enabled database", func() {
				BeforeEach(func() {
					to.enableSharding = true
				})
				It("should run successfully", shouldRunWithPVC)
			})
		})

	})

	Context("PDB", func() {
		It("should run evictions on MongoDB successfully", func() {
			to.mongodb = to.MongoDBRS()
			to.mongodb.Spec.Replicas = types.Int32P(3)
			// Create MongoDB
			to.createAndWaitForRunning()
			//Evict a MongoDB pod
			By("Try to evict pods")
			err = to.EvictPodsFromStatefulSet(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should run evictions on Sharded MongoDB successfully", func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
			to.mongodb.Spec.ShardTopology.Shard.Shards = int32(1)
			to.mongodb.Spec.ShardTopology.ConfigServer.Replicas = int32(3)
			to.mongodb.Spec.ShardTopology.Mongos.Replicas = int32(3)
			to.mongodb.Spec.ShardTopology.Shard.Replicas = int32(3)
			// Create MongoDB
			to.createAndWaitForRunning()
			//Evict a MongoDB pod from each sts
			By("Try to evict pods from each statefulset")
			err := to.EvictPodsFromStatefulSet(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with custom SA Name", func() {
		BeforeEach(func() {
			authSecret := to.SecretForDatabaseAuthentication(to.mongodb.ObjectMeta, false)
			to.mongodb.Spec.AuthSecret = &core.LocalObjectReference{
				Name: authSecret.Name,
			}
			_, err = to.CreateSecret(authSecret)
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyHalt
		})

		It("should start and resume without shard successfully", func() {
			//shouldTakeSnapshot()
			Expect(err).NotTo(HaveOccurred())
			to.mongodb.Spec.PodTemplate = &ofst.PodTemplateSpec{
				Spec: ofst.PodSpec{
					ServiceAccountName: "my-custom-sa",
				},
			}
			to.createAndWaitForRunning()
			if to.mongodb == nil {
				Skip("Skipping")
			}

			By("Check if MongoDB " + to.mongodb.Name + " exists.")
			_, err = to.GetMongoDB(to.mongodb.ObjectMeta)
			if err != nil {
				if kerr.IsNotFound(err) {
					// MongoDB was not created. Hence, rest of cleanup is not necessary.
					return
				}
				Expect(err).NotTo(HaveOccurred())
			}

			By("Delete mongodb: " + to.mongodb.Name)
			err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
			if err != nil {
				if kerr.IsNotFound(err) {
					// MongoDB was not created. Hence, rest of cleanup is not necessary.
					log.Infof("Skipping rest of cleanup. Reason: MongoDB %s is not found.", to.mongodb.Name)
					return
				}
				Expect(err).NotTo(HaveOccurred())
			}

			By("Wait for mongodb to be deleted")
			to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

			By("Resume DB")
			to.createAndWaitForRunning()
		})

		It("should start and resume with shard successfully", func() {
			to.mongodb = to.MongoDBShard()
			//to.mongodb = to.MongoDBWithFlexibleProbeTimeout(mongodb)
			to.mongodb.Spec.ShardTopology.Shard.Shards = int32(1)
			to.mongodb.Spec.ShardTopology.Shard.MongoDBNode.Replicas = int32(1)
			to.mongodb.Spec.ShardTopology.ConfigServer.MongoDBNode.Replicas = int32(1)
			to.mongodb.Spec.ShardTopology.Mongos.MongoDBNode.Replicas = int32(1)

			to.mongodb.Spec.ShardTopology.ConfigServer.PodTemplate = ofst.PodTemplateSpec{
				Spec: ofst.PodSpec{
					ServiceAccountName: "my-custom-sa-configserver",
				},
			}
			to.mongodb.Spec.ShardTopology.Mongos.PodTemplate = ofst.PodTemplateSpec{
				Spec: ofst.PodSpec{
					ServiceAccountName: "my-custom-sa-mongos",
				},
			}
			to.mongodb.Spec.ShardTopology.Shard.PodTemplate = ofst.PodTemplateSpec{
				Spec: ofst.PodSpec{
					ServiceAccountName: "my-custom-sa-shard",
				},
			}

			to.createAndWaitForRunning()
			if to.mongodb == nil {
				Skip("Skipping")
			}

			By("Check if MongoDB " + to.mongodb.Name + " exists.")
			_, err := to.GetMongoDB(to.mongodb.ObjectMeta)
			if err != nil {
				if kerr.IsNotFound(err) {
					// MongoDB was not created. Hence, rest of cleanup is not necessary.
					return
				}
				Expect(err).NotTo(HaveOccurred())
			}

			By("Delete mongodb: " + to.mongodb.Name)
			err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
			if err != nil {
				if kerr.IsNotFound(err) {
					// MongoDB was not created. Hence, rest of cleanup is not necessary.
					log.Infof("Skipping rest of cleanup. Reason: MongoDB %s is not found.", to.mongodb.Name)
					return
				}
				Expect(err).NotTo(HaveOccurred())
			}

			By("Wait for mongodb to be deleted")
			to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

			By("Resume DB")
			to.createAndWaitForRunning()
		})
	})
})
