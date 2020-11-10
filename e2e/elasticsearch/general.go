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
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("General", func() {
	to := testOptions{}
	testName := framework.General

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

		if !framework.RunTestCommunity(testName) {
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

	Context("With PVC - Halted", func() {
		It("Standalone Cluster With Default Resource", func() {
			to.db = to.StandaloneElasticsearch()
			to.createAndHaltElasticsearchAndWaitForBeingReady()
			to.wipeOutElasticsearch()
		})

		It("Standalone Cluster With Security Disabled", func() {
			to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.DisableSecurity = true
				return in
			})
			to.createAndHaltElasticsearchAndWaitForBeingReady()
			to.wipeOutElasticsearch()
		})

		It("Dedicated Elasticsearch Cluster With Default Resource", func() {
			to.db = to.ClusterElasticsearch()
			to.createAndHaltElasticsearchAndWaitForBeingReady()
			to.wipeOutElasticsearch()
		})

		It("Dedicated Elasticsearch Cluster With Security Disabled", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.DisableSecurity = true
				return in
			})
			to.createAndHaltElasticsearchAndWaitForBeingReady()
			to.wipeOutElasticsearch()
		})
	})

	Context("Custom AuthSecret", func() {

		It("Standalone cluster with Custom AuthSecret", func() {
			secret := to.GetAuthSecretForElasticsearch(to.StandaloneElasticsearch(), false)
			secret, err := to.CreateSecret(secret)
			Expect(err).NotTo(HaveOccurred())

			to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.AuthSecret = &core.LocalObjectReference{
					Name: secret.Name,
				}
				return in
			})
			to.createElasticsearchAndWaitForBeingReady()
			_ = to.insertData()
			to.wipeOutElasticsearch()
		})

		It("Dedicated cluster with Custom AuthSecret", func() {
			secret := to.GetAuthSecretForElasticsearch(to.ClusterElasticsearch(), false)
			secret, err := to.CreateSecret(secret)
			Expect(err).NotTo(HaveOccurred())

			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.AuthSecret = &core.LocalObjectReference{
					Name: secret.Name,
				}
				return in
			})
			to.createElasticsearchAndWaitForBeingReady()
			_ = to.insertData()
			to.wipeOutElasticsearch()
		})
	})

	Context("PDB", func() {
		It("Combined cluster should stand pod eviction", func() {
			to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.Replicas = types.Int32P(3)
				in.Spec.MaxUnavailable = &intstr.IntOrString{IntVal: 1}
				return in
			})

			to.createElasticsearchAndWaitForBeingReady()

			By("Evicting pods from statefulSet...")
			err := to.EvictPodsFromStatefulSet(to.db.ObjectMeta, api.ResourceKindElasticsearch)
			Expect(err).NotTo(HaveOccurred())
			to.wipeOutElasticsearch()
		})

		It("Dedicated cluster should stand pod eviction", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.Topology.Master.Replicas = types.Int32P(3)
				in.Spec.Topology.Master.MaxUnavailable = &intstr.IntOrString{IntVal: 1}
				in.Spec.Topology.Data.Replicas = types.Int32P(3)
				in.Spec.Topology.Data.MaxUnavailable = &intstr.IntOrString{IntVal: 1}
				in.Spec.Topology.Ingest.Replicas = types.Int32P(3)
				in.Spec.Topology.Ingest.MaxUnavailable = &intstr.IntOrString{IntVal: 1}
				return in
			})

			to.createElasticsearchAndWaitForBeingReady()

			By("Evicting pods from statefulSet...")
			err := to.EvictPodsFromStatefulSet(to.db.ObjectMeta, api.ResourceKindElasticsearch)
			Expect(err).NotTo(HaveOccurred())
			to.wipeOutElasticsearch()
		})
	})
})
