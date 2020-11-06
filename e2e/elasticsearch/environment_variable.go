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
	"context"
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha2/util"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	exec_util "kmodules.xyz/client-go/tools/exec"
)

var _ = Describe("Environment Variable", func() {
	to := testOptions{}
	testName := framework.EnvironmentVariable

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
			Skip("Skipping test...")
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

	Describe("Environment Variable", func() {
		allowedEnvList := []core.EnvVar{
			{
				Name:  "cluster.name",
				Value: "kubedb-es-e2e-cluster",
			},
			{
				Name:  "ES_JAVA_OPTS",
				Value: "-Xms256m -Xmx256m",
			},
		}

		forbiddenEnvList := []core.EnvVar{
			{
				Name:  "node.name",
				Value: "kubedb-es-e2e-node",
			},
			{
				Name:  "node.master",
				Value: "true",
			},
			{
				Name:  "node.data",
				Value: "true",
			},
		}

		Context("With Allowed Env", func() {
			It("Should Run Successfully with Given Env", func() {
				to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.PodTemplate.Spec.Env = allowedEnvList
					return in
				})
				to.createElasticsearchAndWaitForBeingReady()

				podName := to.GetElasticsearchIngestPodName(to.db)

				By("Checking pod started with given envs")
				pod, err := to.KubeClient().CoreV1().Pods(to.db.Namespace).Get(context.TODO(), podName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				out, err := exec_util.ExecIntoPod(to.RestConfig(), pod, exec_util.Command("env"))
				Expect(err).NotTo(HaveOccurred())
				for _, env := range allowedEnvList {
					Expect(out).Should(ContainSubstring(env.Name + "=" + env.Value))
				}

			})
		})

		Context("With Forbidden Envs", func() {

			It("should reject to create Elasticsearch CRD", func() {
				for _, env := range forbiddenEnvList {
					to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
						in.Spec.PodTemplate.Spec.Env = []core.EnvVar{env}
						return in
					})

					By("Creating Elasticsearch with " + env.Name + " env var.")
					err := to.CreateElasticsearch(to.db)
					Expect(err).To(HaveOccurred())
				}
			})
		})

		Context("Update Envs", func() {
			It("should not reject to update Envs", func() {
				to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.PodTemplate.Spec.Env = allowedEnvList
					return in
				})

				to.createElasticsearchAndWaitForBeingReady()

				By("Updating Envs...")
				_, _, err := util.PatchElasticsearch(context.TODO(), to.DBClient().KubedbV1alpha2(), to.db, func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.PodTemplate.Spec.Env = []core.EnvVar{
						{
							Name:  "cluster.name",
							Value: "kubedb-es-e2e-cluster-patched",
						},
					}
					return in
				}, metav1.PatchOptions{})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

})
