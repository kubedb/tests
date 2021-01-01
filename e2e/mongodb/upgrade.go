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
)

var _ = Describe("Upgrade Database Version", func() {
	to := testOptions{}
	testName := framework.Upgrade
	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !framework.RunTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", testName))
		}
	})

	AfterEach(func() {
		err := to.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())
		//Delete MongoDB
		By("Delete mongodb")
		err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Delete mongodb ops request")
		err = to.DeleteMongoDBOpsRequest(to.mongoOpsReq.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb resources to be wipedOut")
		to.EventuallyWipedOut(to.mongodb.ObjectMeta, api.MongoDB{}.ResourceFQN()).Should(Succeed())
	})

	Context("Update Standalone DB", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBStandalone()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			to.mongoOpsReq = to.MongoDBOpsRequestUpgrade(to.mongodb.Name, to.mongodb.Namespace, framework.DBUpdatedVersion)
		})

		It("Should Update MongoDB version", func() {
			to.shouldTestOpsRequest()
		})

	})

	Context("Update Non-Sharded Cluster", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBRS()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			to.mongoOpsReq = to.MongoDBOpsRequestUpgrade(to.mongodb.Name, to.mongodb.Namespace, framework.DBUpdatedVersion)
		})

		It("Should Update MongoDB version", func() {
			to.shouldTestOpsRequest()
		})

	})

	Context("Update Sharded Cluster", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			to.mongoOpsReq = to.MongoDBOpsRequestUpgrade(to.mongodb.Name, to.mongodb.Namespace, framework.DBUpdatedVersion)
		})

		It("Should Update MongoDB version", func() {
			to.shouldTestOpsRequest()
		})

	})
})
