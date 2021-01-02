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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Termination Policy", func() {
	to := testOptions{}
	testName := framework.TerminationPolicy

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
		to.CleanWorkloadLeftOvers(api.Elasticsearch{}.ResourceFQN())
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	Context("Termination Policy", func() {

		// Deletes database pods, service but leave the PVCs and Secrets
		Context("Halted", func() {
			It("Standalone cluster", func() {
				to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					return in
				})
				to.createAndHaltElasticsearchAndWaitForBeingReady()
				to.wipeOutElasticsearch()
			})

			It("Dedicated cluster", func() {
				to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					return in
				})
				to.createAndHaltElasticsearchAndWaitForBeingReady()
				to.wipeOutElasticsearch()
			})
		})

		// Deletes database pods, service, pvcs and secrets.
		Context("WipeOut", func() {
			var shouldRunWithTerminationWipeOut = func() {
				to.createElasticsearchAndWaitForBeingReady()

				By("Delete elasticsearch")
				err := to.DeleteElasticsearch(to.db.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("wait until elasticsearch is deleted")
				to.EventuallyElasticsearch(to.db.ObjectMeta).Should(BeFalse())

				By("Wait for elasticsearch services to be deleted")
				to.EventuallyServices(to.db.ObjectMeta, api.Elasticsearch{}.ResourceFQN()).Should(Succeed())

				By("Check for deleted PVCs")
				to.EventuallyPVCCount(to.db.ObjectMeta, api.Elasticsearch{}.ResourceFQN()).Should(Equal(0))

				By("Check for deleted Secrets")
				to.EventuallyDBSecretCount(to.db.ObjectMeta, api.Elasticsearch{}.ResourceFQN()).Should(Equal(0))
			}

			It("Standalone Cluster", func() {
				to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					return in
				})
				shouldRunWithTerminationWipeOut()
			})

			It("Dedicated Cluster", func() {
				to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					return in
				})
				shouldRunWithTerminationWipeOut()
			})
		})

		// Rejects attempt to delete database using ValidationWebhook.
		Context("DoNotTerminate", func() {

			It("should work successfully", func() {
				to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
					return in
				})

				// create and wait for running Elasticsearch
				to.createElasticsearchAndWaitForBeingReady()

				By("Delete elasticsearch")
				err := to.DeleteElasticsearch(to.db.ObjectMeta)
				Expect(err).Should(HaveOccurred())

				By("Elasticsearch is not deleted. Check for elasticsearch")
				to.EventuallyElasticsearch(to.db.ObjectMeta).Should(BeTrue())

				By("Check for Running elasticsearch")
				to.EventuallyElasticsearchReady(to.db.ObjectMeta).Should(BeTrue())

				By("Halt Elasticsearch: Update elasticsearch to set spec.halted = true")
				_, err = to.PatchElasticsearch(to.db.ObjectMeta, func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.Halted = true
					return in
				})
				Expect(err).To(HaveOccurred())

				By("Elasticsearch is not paused. Check for elasticsearch")
				to.EventuallyElasticsearch(to.db.ObjectMeta).Should(BeTrue())

				By("Check for Running elasticsearch")
				to.EventuallyElasticsearchReady(to.db.ObjectMeta).Should(BeTrue())

				By("Update elasticsearch to set spec.terminationPolicy = halt")
				_, err = to.PatchElasticsearch(to.db.ObjectMeta, func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					return in
				})
				Expect(err).NotTo(HaveOccurred())

				By("Halt Elasticsearch: Update elasticsearch to set spec.halted = true")
				_, err = to.PatchElasticsearch(to.db.ObjectMeta, func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.Halted = true
					return in
				})
				Expect(err).NotTo(HaveOccurred())

				By("Wait for halted/paused elasticsearch")
				to.EventuallyElasticsearchPhase(to.db.ObjectMeta).Should(Equal(api.DatabasePhaseHalted))

				By("Resume Elasticsearch: Update elasticsearch to set spec.halted = false")
				_, err = to.PatchElasticsearch(to.db.ObjectMeta, func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.Halted = false
					return in
				})
				Expect(err).NotTo(HaveOccurred())

				By("Wait for Running elasticsearch")
				to.EventuallyElasticsearchReady(to.db.ObjectMeta).Should(BeTrue())

				to.wipeOutElasticsearch()
			})
		})

		// Deletes database pods, service, pvcs but leave the secrets
		Context("Delete", func() {

			It("Should work successfully", func() {
				to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.TerminationPolicy = api.TerminationPolicyDelete
					return in
				})

				to.createElasticsearchAndWaitForBeingReady()

				By("Delete elasticsearch")
				err := to.DeleteElasticsearch(to.db.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Wait until elasticsearch is deleted")
				to.EventuallyElasticsearch(to.db.ObjectMeta).Should(BeFalse())

				By("Wait for elasticsearch services to be deleted")
				to.EventuallyServices(to.db.ObjectMeta, api.Elasticsearch{}.ResourceFQN()).Should(Succeed())
				Expect(err).NotTo(HaveOccurred())

				By("Check for deleted PVCs")
				to.EventuallyPVCCount(to.db.ObjectMeta, api.Elasticsearch{}.ResourceFQN()).Should(Equal(0))

				By("Check for intact Secrets")
				to.EventuallyDBSecretCount(to.db.ObjectMeta, api.Elasticsearch{}.ResourceFQN()).ShouldNot(Equal(0))
			})
		})
	})
})
