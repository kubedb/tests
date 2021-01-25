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
	"time"

	"kubedb.dev/apimachinery/apis/autoscaling/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Elasticsearch Autoscaling", func() {
	to := testOptions{}
	provisioner := "topolvm-provisioner"
	testName := framework.Autoscaling
	computeAutoscalerSpec := v1alpha1.ComputeAutoscalerSpec{
		Trigger:                v1alpha1.AutoscalerTriggerOn,
		ResourceDiffPercentage: 5,
		PodLifeTimeThreshold: v1.Duration{
			Duration: 3 * time.Minute,
		},
	}

	storageAutoscalerSpec := v1alpha1.StorageAutoscalerSpec{
		Trigger:          v1alpha1.AutoscalerTriggerOn,
		UsageThreshold:   50,
		ScalingThreshold: 100,
	}

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
		By("Delete left over Elasticsearch Autoscaler objects")
		to.CleanElasticsearchAutoscalers()
		By("Delete left over Elasticsearch Ops Request objects")
		to.CleanElasticsearchOpsRequests()
		By("Delete left over workloads if exists any")
		to.CleanWorkloadLeftOvers(api.ResourceKindElasticsearch)
	})

	Context("Computing", func() {
		Context("Auto Scaling Combined cluster Resources", func() {
			BeforeEach(func() {
				to.db = to.StandaloneElasticsearch()
				to.db.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				esComputeSpec := &v1alpha1.ElasticsearchComputeAutoscalerSpec{
					Node: &computeAutoscalerSpec,
				}
				to.esAutoscaler = to.ElasticsearchAutoscalerCompute(to.db.Name, to.db.Namespace, esComputeSpec)
			})

			It("Should Scale StandAlone Resources", func() {
				to.shouldTestComputeAutoscaler()
			})

		})
		Context("Auto Scaling Cluster Master Resources", func() {
			BeforeEach(func() {
				to.db = to.ClusterElasticsearch()
				to.db.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				esComputeSpec := &v1alpha1.ElasticsearchComputeAutoscalerSpec{
					Topology: &v1alpha1.ElasticsearchComputeTopologyAutoscalerSpec{
						Master: &computeAutoscalerSpec,
					},
				}
				to.esAutoscaler = to.ElasticsearchAutoscalerCompute(to.db.Name, to.db.Namespace, esComputeSpec)
			})

			It("Should Scale Master Resources", func() {
				to.shouldTestComputeAutoscaler()
			})
		})
		Context("Auto Scaling Cluster Ingest Resources", func() {
			BeforeEach(func() {
				to.db = to.ClusterElasticsearch()
				to.db.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				esComputeSpec := &v1alpha1.ElasticsearchComputeAutoscalerSpec{
					Topology: &v1alpha1.ElasticsearchComputeTopologyAutoscalerSpec{
						Ingest: &computeAutoscalerSpec,
					},
				}
				to.esAutoscaler = to.ElasticsearchAutoscalerCompute(to.db.Name, to.db.Namespace, esComputeSpec)
			})

			It("Should Scale Ingest Resources", func() {
				to.shouldTestComputeAutoscaler()
			})

		})
		Context("Auto Scaling Cluster Data Resources", func() {
			BeforeEach(func() {
				to.db = to.ClusterElasticsearch()
				to.db.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				esComputeSpec := &v1alpha1.ElasticsearchComputeAutoscalerSpec{
					Topology: &v1alpha1.ElasticsearchComputeTopologyAutoscalerSpec{
						Data: &computeAutoscalerSpec,
					},
				}
				to.esAutoscaler = to.ElasticsearchAutoscalerCompute(to.db.Name, to.db.Namespace, esComputeSpec)
			})

			It("Should Scale Data Resources", func() {
				to.shouldTestComputeAutoscaler()
			})
		})
	})

	Context("Storage", func() {
		Context("Auto Combined cluster Storage", func() {
			BeforeEach(func() {
				to.db = to.StandaloneElasticsearch()
				to.db.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.db.Spec.Storage.StorageClassName = &provisioner
				esStorageSpec := &v1alpha1.ElasticsearchStorageAutoscalerSpec{
					Node: &storageAutoscalerSpec,
				}
				to.esAutoscaler = to.ElasticsearchAutoscalerStorage(to.db.Name, to.db.Namespace, esStorageSpec)
			})

			It("Should Scale StandAlone Storage", func() {
				to.shouldTestStorageAutoscaler()
			})

		})
		Context("Auto Scaling Cluster Master Storage", func() {
			BeforeEach(func() {
				to.db = to.ClusterElasticsearch()
				to.db.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.db.Spec.Topology.Master.Storage.StorageClassName = &provisioner
				esStorageSpec := &v1alpha1.ElasticsearchStorageAutoscalerSpec{
					Topology: &v1alpha1.ElasticsearchStorageTopologyAutoscalerSpec{
						Master: &storageAutoscalerSpec,
					},
				}
				to.esAutoscaler = to.ElasticsearchAutoscalerStorage(to.db.Name, to.db.Namespace, esStorageSpec)
			})

			It("Should Scale Master Storage", func() {
				to.shouldTestStorageAutoscaler()
			})
		})
		Context("Auto Scaling Cluster Ingest Storage", func() {
			BeforeEach(func() {
				to.db = to.ClusterElasticsearch()
				to.db.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.db.Spec.Topology.Ingest.Storage.StorageClassName = &provisioner
				esStorageSpec := &v1alpha1.ElasticsearchStorageAutoscalerSpec{
					Topology: &v1alpha1.ElasticsearchStorageTopologyAutoscalerSpec{
						Ingest: &storageAutoscalerSpec,
					},
				}
				to.esAutoscaler = to.ElasticsearchAutoscalerStorage(to.db.Name, to.db.Namespace, esStorageSpec)
			})

			It("Should Scale Ingest Storage", func() {
				to.shouldTestStorageAutoscaler()
			})

		})
		Context("Auto Scaling Cluster Data Storage", func() {
			BeforeEach(func() {
				to.db = to.ClusterElasticsearch()
				to.db.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.db.Spec.Topology.Data.Storage.StorageClassName = &provisioner
				esStorageSpec := &v1alpha1.ElasticsearchStorageAutoscalerSpec{
					Topology: &v1alpha1.ElasticsearchStorageTopologyAutoscalerSpec{
						Data: &storageAutoscalerSpec,
					},
				}
				to.esAutoscaler = to.ElasticsearchAutoscalerStorage(to.db.Name, to.db.Namespace, esStorageSpec)
			})

			It("Should Scale Data Storage", func() {
				to.shouldTestStorageAutoscaler()
			})
		})
	})
})
