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

package elasticsearch

import (
	"fmt"
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Volume Expansion", func() {
	to := testOptions{}
	testName := framework.VolumeExpansion

	BeforeEach(func() {
		f := framework.NewInvocation()
		to = testOptions{
			Invocation: f,
			db:         f.StandaloneElasticsearch(),
		}
		if to.StorageClass == "" {
			Skip("Missing StorageClassName. Provide as flag to test this.")
		}

		if strings.ToLower(framework.DBType) != api.ResourceSingularElasticsearch {
			Skip(fmt.Sprintf("Skipping Elasticsearch: %s tests...", testName))
		}

		if !framework.RunTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` to test this.", testName))
		}

	})

	AfterEach(func() {
		// Cleanup
		By("Cleanup Left Overs")
		By("Delete left over Elasticsearch objects")
		to.CleanElasticsearch()
		By("Delete left over Elasticsearch Ops Request objects")
		to.CleanElasticsearchOpsRequests()
		By("Delete left over workloads if exists any")
		to.CleanWorkloadLeftOvers(api.ResourceKindElasticsearch)
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	Context("Combined Cluster", func() {
		AfterEach(func() {
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			valid := to.verifyStorage()
			Expect(valid).Should(BeTrue())
			to.wipeOutElasticsearch()
			err := to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should run with default resources", func() {
			to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			storage := resource.MustParse("2Gi")
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestVolumeExpansion(to.db.ObjectMeta, &dbaapi.ElasticsearchVolumeExpansionSpec{
				Node: &storage,
			})

		})
	})

	Context("Dedicated Cluster", func() {
		AfterEach(func() {
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			valid := to.verifyStorage()
			Expect(valid).Should(BeTrue())
			to.wipeOutElasticsearch()
			err := to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("All nodes", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			mStorage := resource.MustParse("2Gi")
			dStorage := resource.MustParse("3Gi")
			iStorage := resource.MustParse("2.5Gi")
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestVolumeExpansion(to.db.ObjectMeta, &dbaapi.ElasticsearchVolumeExpansionSpec{
				Topology: &dbaapi.ElasticsearchVolumeExpansionTopologySpec{
					Master: &mStorage,
					Ingest: &iStorage,
					Data:   &dStorage,
				},
			})

		})

		It("Only Master Nodes", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			mStorage := resource.MustParse("2Gi")
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestVolumeExpansion(to.db.ObjectMeta, &dbaapi.ElasticsearchVolumeExpansionSpec{
				Topology: &dbaapi.ElasticsearchVolumeExpansionTopologySpec{
					Master: &mStorage,
				},
			})

		})

		It("Only Data Nodes", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			dStorage := resource.MustParse("3Gi")
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestVolumeExpansion(to.db.ObjectMeta, &dbaapi.ElasticsearchVolumeExpansionSpec{
				Topology: &dbaapi.ElasticsearchVolumeExpansionTopologySpec{
					Data: &dStorage,
				},
			})
		})

		It("Only Ingest Nodes", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			iStorage := resource.MustParse("2.5Gi")
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestVolumeExpansion(to.db.ObjectMeta, &dbaapi.ElasticsearchVolumeExpansionSpec{
				Topology: &dbaapi.ElasticsearchVolumeExpansionTopologySpec{
					Ingest: &iStorage,
				},
			})
		})
	})

})
