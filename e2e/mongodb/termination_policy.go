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
)

var _ = Describe("Termination Policy", func() {
	var err error
	to := testOptions{}
	testName := framework.TerminationPolicy

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
	Context("with TerminationDoNotTerminate", func() {
		BeforeEach(func() {
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
		})

		var shouldWorkDoNotTerminate = func() {
			// Create and wait for running MongoDB
			to.createAndWaitForRunning()

			By("Delete mongodb")
			err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
			Expect(err).Should(HaveOccurred())

			By("MongoDB is not paused. Check for mongodb")
			to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeTrue())

			By("Check for Running mongodb")
			to.EventuallyMongoDBRunning(to.mongodb.ObjectMeta).Should(BeTrue())

			By("Update mongodb to set spec.terminationPolicy = Pause")
			_, err := to.PatchMongoDB(to.mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
				in.Spec.TerminationPolicy = api.TerminationPolicyHalt
				return in
			})
			Expect(err).NotTo(HaveOccurred())
		}

		It("should work successfully", shouldWorkDoNotTerminate)

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
			})
			It("should run successfully", shouldWorkDoNotTerminate)
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
				//to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
			})
			It("should run successfully", shouldWorkDoNotTerminate)
		})

	})

	Context("with TerminationPolicyHalt", func() {
		var shouldRunWithTerminationHalt = func() {
			to.createAndInsertData()

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

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

			By("Delete mongodb")
			err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			By("wait until mongodb is deleted")
			to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

			// create mongodb object again to resume it
			By("Create (pause) MongoDB: " + to.mongodb.Name)
			err = to.CreateMongoDB(to.mongodb)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for Running mongodb")
			to.EventuallyMongoDBRunning(to.mongodb.ObjectMeta).Should(BeTrue())

			By("Ping mongodb database")
			to.EventuallyPingMongo(to.mongodb.ObjectMeta)

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
		}

		It("should create dormantdatabase successfully", shouldRunWithTerminationHalt)

		Context("with Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
			})

			It("should create dormantdatabase successfully", shouldRunWithTerminationHalt)
		})

		Context("with Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				//to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
			})

			It("should create dormantdatabase successfully", shouldRunWithTerminationHalt)
		})
	})

	Context("with TerminationPolicyDelete", func() {
		BeforeEach(func() {
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyDelete
		})

		var shouldRunWithTerminationDelete = func() {
			to.createAndInsertData()

			By("Delete mongodb")
			err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			By("wait until mongodb is deleted")
			to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

			By("Check for deleted PVCs")
			to.EventuallyPVCCount(to.mongodb.ObjectMeta, api.ResourceKindMongoDB).Should(Equal(0))

			By("Check for intact Secrets")
			to.EventuallyDBSecretCount(to.mongodb.ObjectMeta, api.ResourceKindMongoDB).ShouldNot(Equal(0))
		}

		It("should run with TerminationPolicyDelete", shouldRunWithTerminationDelete)

		Context("with Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyDelete
			})
			It("should initialize database successfully", shouldRunWithTerminationDelete)
		})

		Context("with Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyDelete
			})
			It("should initialize database successfully", shouldRunWithTerminationDelete)
		})
	})

	Context("with TerminationPolicyWipeOut", func() {
		BeforeEach(func() {
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
		})

		var shouldRunWithTerminationWipeOut = func() {
			to.createAndInsertData()

			By("Delete mongodb")
			err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			By("wait until mongodb is deleted")
			to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

			By("Check for deleted PVCs")
			to.EventuallyPVCCount(to.mongodb.ObjectMeta, api.ResourceKindMongoDB).Should(Equal(0))

			By("Check for deleted Secrets")
			to.EventuallyDBSecretCount(to.mongodb.ObjectMeta, api.ResourceKindMongoDB).Should(Equal(0))
		}

		It("should run with TerminationPolicyWipeOut", shouldRunWithTerminationWipeOut)

		Context("with Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			})
			It("should initialize database successfully", shouldRunWithTerminationWipeOut)
		})

		Context("with Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

				//to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
			})
			It("should initialize database successfully", shouldRunWithTerminationWipeOut)
		})
	})
})
