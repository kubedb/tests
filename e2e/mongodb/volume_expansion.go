package e2e_test

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Volume Expansion", func() {
	to := testOptions{}
	testName := framework.VolumeExpansion
	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !to.IsGKE() {
			to.skipMessage = "volume expansion testing is only supported in GKE"
		}
		if !runTestEnterprise(testName) {
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
		to.EventuallyWipedOut(to.mongodb.ObjectMeta).Should(Succeed())
	})

	Context("Standalone Instance", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBStandalone()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			standalone := resource.MustParse("2Gi")
			to.mongoOpsReq = to.MongoDBOpsRequestVolumeExpansion(to.mongodb.Name, to.mongodb.Namespace, &standalone, nil, nil, nil)
		})

		It("Should Scale StandAlone Mongodb Resources", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("ReplicaSet Cluster", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBRS()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			replicaset := resource.MustParse("2Gi")
			to.mongoOpsReq = to.MongoDBOpsRequestVolumeExpansion(to.mongodb.Name, to.mongodb.Namespace, nil, &replicaset, nil, nil)
		})

		It("Should Scale ReplicaSet Resources", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scaling ConfigServer Resources", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			configServer := resource.MustParse("2Gi")
			to.mongoOpsReq = to.MongoDBOpsRequestVolumeExpansion(to.mongodb.Name, to.mongodb.Namespace, nil, nil, nil, &configServer)
		})

		It("Should Scale ConfigServer Resources", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scaling Shard Resources", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			shard := resource.MustParse("2Gi")
			to.mongoOpsReq = to.MongoDBOpsRequestVolumeExpansion(to.mongodb.Name, to.mongodb.Namespace, nil, nil, &shard, nil)
		})

		It("Should Scale Shard Resources", func() {
			to.shouldTestOpsRequest()
		})
	})
})
