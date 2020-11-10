package elasticsearch

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Version Upgrade", func() {
	to := testOptions{}
	testName := framework.Upgrade

	BeforeEach(func() {
		f := framework.NewInvocation()
		to = testOptions{
			Invocation: f,
			db:         f.StandaloneElasticsearch(),
		}
		if to.StorageClass == "" {
			Skip("Missing StorageClassName. Provide as flag to test this.")
		}

		if framework.DBType != api.ResourceKindElasticsearch {
			Skip(fmt.Sprintf("Skipping Elasticsearch: %s tests...", testName))
		}

		if !framework.RunTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` to test this.", testName))
		}
		if framework.SSLEnabled {
			Skip("Skipping test with SSL enabled...")
		}
	})

	AfterEach(func() {
		// Cleanup
		By("Cleanup Left Overs")
		By("Delete left over Elasticsearch objects")
		to.CleanElasticsearch()
		By("Delete left over workloads if exists any")
		to.CleanWorkloadLeftOvers(api.ResourceKindElasticsearch)
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	Context("Standalone Cluster", func() {
		BeforeEach(func() {
			to.db = to.StandaloneElasticsearch()
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestUpgrade(to.db.ObjectMeta, framework.DBUpdatedVersion)
		})

		It("Should be successfully upgraded", func() {
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			to.wipeOutElasticsearch()
		})
	})
})
