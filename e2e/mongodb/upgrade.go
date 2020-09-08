package e2e_test

import (
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Upgrade Database Version", func() {
	to := testOptions{}
	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
	})

	AfterEach(func() {
		err := to.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		//Delete MongoDB
		By("Delete mongodb")
		err := to.DeleteMongoDB(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Delete mongodb ops request")
		err = to.DeleteMongoDBOpsRequest(to.mongoOpsReq.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb resources to be wipedOut")
		to.EventuallyWipedOut(to.mongodb.ObjectMeta).Should(Succeed())
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
