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

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gomodules.xyz/pointer"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
)

var _ = Describe("Vertical Scaling", func() {
	to := testOptions{}
	testName := framework.VerticalScaling

	BeforeEach(func() {
		f := framework.NewInvocation()
		to = testOptions{
			Invocation: f,
			db:         f.StandaloneElasticsearch(),
		}
		if to.StorageClass == "" {
			Skip("Missing StorageClassName. Provide as flag to test this.")
		}

		if framework.DBType != api.ResourceSingularElasticsearch {
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

		It("should run with default resources", func() {
			to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestVerticalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchVerticalScalingSpec{
				Node: &core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceCPU:    resource.MustParse(".250"),
						core.ResourceMemory: resource.MustParse("300Mi"),
					},
					Limits: core.ResourceList{
						core.ResourceCPU:    resource.MustParse(".650"),
						core.ResourceMemory: resource.MustParse("500Mi"),
					},
				},
			})
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			ok := to.verifyResources()
			Expect(ok).Should(BeTrue())
		})
	})

	Context("Dedicated Cluster", func() {
		AfterEach(func() {
			to.wipeOutElasticsearch()
			err := to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("All nodes", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				in.Spec.Topology.Master.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Data.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Ingest.Replicas = pointer.Int32P(1)
				return in
			})
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestVerticalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchVerticalScalingSpec{
				Topology: &dbaapi.ElasticsearchVerticalScalingTopologySpec{
					Master: &core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".250"),
							core.ResourceMemory: resource.MustParse("300Mi"),
						},
						Limits: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".650"),
							core.ResourceMemory: resource.MustParse("500Mi"),
						},
					},
					Ingest: &core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".260"),
							core.ResourceMemory: resource.MustParse("310Mi"),
						},
						Limits: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".660"),
							core.ResourceMemory: resource.MustParse("510Mi"),
						},
					},
					Data: &core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".270"),
							core.ResourceMemory: resource.MustParse("320Mi"),
						},
						Limits: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".670"),
							core.ResourceMemory: resource.MustParse("520Mi"),
						},
					},
				},
			})
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			ok := to.verifyResources()
			Expect(ok).Should(BeTrue())
		})

		It("Only Master Nodes", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				in.Spec.Topology.Master.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Data.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Ingest.Replicas = pointer.Int32P(1)
				return in
			})
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestVerticalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchVerticalScalingSpec{
				Topology: &dbaapi.ElasticsearchVerticalScalingTopologySpec{
					Master: &core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".250"),
							core.ResourceMemory: resource.MustParse("300Mi"),
						},
						Limits: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".650"),
							core.ResourceMemory: resource.MustParse("500Mi"),
						},
					},
				},
			})
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			ok := to.verifyResources()
			Expect(ok).Should(BeTrue())
		})

		It("Only Data Nodes", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				in.Spec.Topology.Master.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Data.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Ingest.Replicas = pointer.Int32P(1)
				return in
			})
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestVerticalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchVerticalScalingSpec{
				Topology: &dbaapi.ElasticsearchVerticalScalingTopologySpec{
					Data: &core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".250"),
							core.ResourceMemory: resource.MustParse("300Mi"),
						},
						Limits: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".650"),
							core.ResourceMemory: resource.MustParse("500Mi"),
						},
					},
				},
			})
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			ok := to.verifyResources()
			Expect(ok).Should(BeTrue())
		})

		It("Only Ingest Nodes", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				in.Spec.Topology.Master.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Data.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Ingest.Replicas = pointer.Int32P(1)
				return in
			})
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestVerticalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchVerticalScalingSpec{
				Topology: &dbaapi.ElasticsearchVerticalScalingTopologySpec{
					Ingest: &core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".250"),
							core.ResourceMemory: resource.MustParse("300Mi"),
						},
						Limits: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(".650"),
							core.ResourceMemory: resource.MustParse("500Mi"),
						},
					},
				},
			})
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			ok := to.verifyResources()
			Expect(ok).Should(BeTrue())
		})
	})

	Context("Dedicated Cluster with metrics exporter", func() {
		AfterEach(func() {
			to.wipeOutElasticsearch()
			err := to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should run successfully", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.Monitor = &mona.AgentSpec{
					Agent: mona.AgentPrometheus,
				}

				in.Spec.EnableSSL = framework.SSLEnabled
				in.Spec.Topology.Master.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Data.Replicas = pointer.Int32P(1)
				in.Spec.Topology.Ingest.Replicas = pointer.Int32P(1)
				return in
			})
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestVerticalScale(to.db.ObjectMeta, &dbaapi.ElasticsearchVerticalScalingSpec{
				Exporter: &core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceMemory: resource.MustParse("100Mi"),
						core.ResourceCPU:    resource.MustParse("100m"),
					},
					Limits: core.ResourceList{
						core.ResourceMemory: resource.MustParse("200Mi"),
						core.ResourceCPU:    resource.MustParse("250m"),
					},
				},
			})
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.verifyData(indicesCount)
			ok := to.verifyResources()
			Expect(ok).Should(BeTrue())
		})
	})
})
