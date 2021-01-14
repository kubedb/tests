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
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Custom Configuration", func() {
	to := testOptions{}
	testName := framework.CustomConfig

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

		if !framework.RunTestCommunity(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` to test this.", testName))
		}
	})

	AfterEach(func() {
		// Cleanup
		By("Cleanup Left Overs")
		By("Delete left over Elasticsearch objects")
		to.CleanElasticsearch()
		By("Delete left over workloads if exists any")
		to.CleanWorkloadLeftOvers(api.Elasticsearch{}.ResourceFQN())
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	Describe("Custom Configuration", func() {
		It("Dedicated Cluster", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = framework.SSLEnabled
				return in
			})
			to.createElasticsearchWithCustomConfigAndWaitForBeingReady()
			to.wipeOutElasticsearch()
			err := to.DeleteSecret(to.configSecret.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
