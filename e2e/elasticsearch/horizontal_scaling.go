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
	"gomodules.xyz/pointer"
)

var _ = Describe("Horizontal Scaling", func() {
	to := testOptions{}
	testName := framework.HorizontalScaling

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
			to.wipeOutElasticsearch()
			err := to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Scale Up", func() {
			to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestHorizontalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchHorizontalScalingSpec{
				Node: pointer.Int32P(3),
			})
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
		})

		It("Scale Down", func() {
			to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.Replicas = pointer.Int32P(4)
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestHorizontalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchHorizontalScalingSpec{
				Node: pointer.Int32P(2),
			})
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
		})

	})

	Context("Dedicated Cluster", func() {
		AfterEach(func() {
			to.wipeOutElasticsearch()
			err := to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Scale Up", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.Topology.Master.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Data.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Ingest.Replicas = pointer.Int32P(1)
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestHorizontalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchHorizontalScalingSpec{
				Topology: &dbaapi.ElasticsearchHorizontalScalingTopologySpec{
					Master: pointer.Int32P(2),
					Data:   pointer.Int32P(2),
					Ingest: pointer.Int32P(3),
				},
			})

			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
		})

		It("Scale Down", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.Topology.Data.Replicas = pointer.Int32P(3)
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestHorizontalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchHorizontalScalingSpec{
				Topology: &dbaapi.ElasticsearchHorizontalScalingTopologySpec{
					Master: pointer.Int32P(1),
					Data:   pointer.Int32P(2),
					Ingest: pointer.Int32P(1),
				},
			})

			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
		})

		It("Scale Up and Down", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.Topology.Master.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Data.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Ingest.Replicas = pointer.Int32P(5)
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestHorizontalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchHorizontalScalingSpec{
				Topology: &dbaapi.ElasticsearchHorizontalScalingTopologySpec{
					Master: pointer.Int32P(2), // scale up
					Data:   pointer.Int32P(3), // scale up
					Ingest: pointer.Int32P(2), // scale down
				},
			})

			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
		})
	})

	Context("Multiple HorizontalScaling Ops Requests", func() {
		AfterEach(func() {
			to.wipeOutElasticsearch()
			err := to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Combined Cluster: Scale Up -> Scale Down -> Scale Up", func() {
			to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.Replicas = pointer.Int32P(1)
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			to.createElasticsearchAndWaitForBeingReady()

			// Scale Up
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestHorizontalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchHorizontalScalingSpec{
				Node: pointer.Int32P(4),
			})
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			err := to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())

			// Scale Down
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestHorizontalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchHorizontalScalingSpec{
				Node: pointer.Int32P(2),
			})
			indicesCount = to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			err = to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())

			// Scale Up
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestHorizontalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchHorizontalScalingSpec{
				Node: pointer.Int32P(4),
			})
			indicesCount = to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
		})

		It("Dedicated Cluster: Scale Up -> Scale Down -> Scale Up", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.Topology.Master.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Data.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Ingest.Replicas = pointer.Int32P(1)
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			to.createElasticsearchAndWaitForBeingReady()

			// Scale Up
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestHorizontalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchHorizontalScalingSpec{
				Topology: &dbaapi.ElasticsearchHorizontalScalingTopologySpec{
					Master: pointer.Int32P(2),
					Data:   pointer.Int32P(3),
					Ingest: pointer.Int32P(2),
				},
			})
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			err := to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())

			// Scale Down
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestHorizontalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchHorizontalScalingSpec{
				Topology: &dbaapi.ElasticsearchHorizontalScalingTopologySpec{
					Master: pointer.Int32P(1),
					Data:   pointer.Int32P(2),
					Ingest: pointer.Int32P(1),
				},
			})
			indicesCount = to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			err = to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())

			// Scale Up
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestHorizontalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchHorizontalScalingSpec{
				Topology: &dbaapi.ElasticsearchHorizontalScalingTopologySpec{
					Master: pointer.Int32P(2),
					Data:   pointer.Int32P(3),
					Ingest: pointer.Int32P(2),
				},
			})
			indicesCount = to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
		})
	})
})
