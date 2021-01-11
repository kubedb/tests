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
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("StorageType", func() {
	to := testOptions{}
	testName := framework.StorageType

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
		if strings.ToLower(framework.DBType) != api.ResourceSingularMongoDB {
			Skip(fmt.Sprintf("Skipping MongoDB: %s tests...", testName))
		}
		if !framework.RunTestCommunity(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` to test this.", testName))
		}
	})

	AfterEach(func() {
		// Cleanup
		By("Cleanup Left Overs")
		By("Delete left over MongoDB objects")
		to.CleanMongoDB()
		By("Delete left over workloads if exists any")
		to.CleanWorkloadLeftOvers(api.MongoDB{}.ResourceFQN())
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

	Context("Ephemeral", func() {

		Context("Standalone MongoDB", func() {

			BeforeEach(func() {
				to.mongodb.Spec.StorageType = api.StorageTypeEphemeral
				to.mongodb.Spec.Storage = nil
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			})

			It("should run successfully", to.createAndInsertData)
		})

		Context("With Replica Set", func() {

			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Replicas = types.Int32P(3)
				to.mongodb.Spec.StorageType = api.StorageTypeEphemeral
				to.mongodb.Spec.Storage = nil
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			})

			It("should run successfully", to.createAndInsertData)
		})

		Context("With Sharding", func() {

			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.StorageType = api.StorageTypeEphemeral
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

				//to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
			})

			It("should run successfully", to.createAndInsertData)
		})

		Context("With TerminationPolicyHalt", func() {

			BeforeEach(func() {
				to.mongodb.Spec.StorageType = api.StorageTypeEphemeral
				to.mongodb.Spec.Storage = nil
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyHalt
			})

			It("should reject to create MongoDB object", func() {

				By("Creating MongoDB: " + to.mongodb.Name)
				err := to.CreateMongoDB(to.mongodb)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
