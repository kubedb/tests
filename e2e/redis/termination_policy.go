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

package redis

import (
	"fmt"
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("Termination Policy Redis", func() {
	var (
		err   error
		key   string
		value string
	)
	to := testOptions{}
	testName := framework.RedisTerminationPolicy

	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !runTestCommunity(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `community` to test this.", testName))
		}
		if framework.SSLEnabled && !strings.HasPrefix(framework.DBVersion, "6.") {
			Skip(fmt.Sprintf("TLS is not supported for version `%s` in redis", framework.DBVersion))
		}

		to.redis = to.RedisStandalone(framework.DBVersion)
		to.skipMessage = ""
		key = rand.WithUniqSuffix("kubed-e2e")
		value = rand.GenerateTokenWithLength(10)
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	AfterEach(func() {
		err = to.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())

		if to.redis == nil {
			// No redis. So, no cleanup
			return
		}

		By("Check if Redis " + to.redis.Name + " exists.")
		rd, err := to.GetRedis(to.redis.ObjectMeta)
		if err != nil {
			if kerr.IsNotFound(err) {
				// Redis was not created. Hence, rest of cleanup is not necessary.
				return
			}
			Expect(err).NotTo(HaveOccurred())
		}

		By("Update redis to set spec.terminationPolicy = WipeOut")
		_, err = to.PatchRedis(rd.ObjectMeta, func(in *api.Redis) *api.Redis {
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		By("Delete redis")
		err = to.DeleteRedis(to.redis.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for redis to be deleted")
		to.EventuallyRedis(to.redis.ObjectMeta).Should(BeFalse())

		By("Wait for redis resources to be wipedOut")
		to.EventuallyWipedOut(to.redis.ObjectMeta).Should(Succeed())
	})

	var shouldBeSuccessfullyRunning = func() {
		if to.skipMessage != "" {
			Skip(to.skipMessage)
		}

		// Create Redis
		to.createRedis()

		By("Inserting item into database")
		to.EventuallySetItem(to.redis, key, value).Should(BeTrue())

		By("Retrieving item from database")
		to.EventuallyGetItem(to.redis, key).Should(BeEquivalentTo(value))
	}

	Context("with TerminationPolicyDoNotTerminate", func() {
		BeforeEach(func() {
			to.redis.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
		})

		It("should work successfully", func() {
			// Create and wait for running Redis
			to.createRedis()

			By("Delete redis")
			err = to.DeleteRedis(to.redis.ObjectMeta)
			Expect(err).Should(HaveOccurred())

			By("Redis is not halted. Check for redis")
			to.EventuallyRedis(to.redis.ObjectMeta).Should(BeTrue())

			By("Check for Running redis")
			to.EventuallyRedisRunning(to.redis.ObjectMeta).Should(BeTrue())

			By("Update redis to set spec.terminationPolicy = Halt")
			_, err := to.PatchRedis(to.redis.ObjectMeta, func(in *api.Redis) *api.Redis {
				in.Spec.TerminationPolicy = api.TerminationPolicyHalt
				return in
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with TerminationPolicyHalt", func() {
		var shouldRunWithTerminationHalt = func() {

			shouldBeSuccessfullyRunning()

			By("Halt Redis: Update redis to set spec.halted = true")
			_, err := to.PatchRedis(to.redis.ObjectMeta, func(in *api.Redis) *api.Redis {
				in.Spec.Halted = true
				return in
			})
			Expect(err).NotTo(HaveOccurred())

			By("Wait for halted redis")
			to.EventuallyRedisPhase(to.redis.ObjectMeta).Should(Equal(api.DatabasePhaseHalted))

			By("Resume Redis: Update redis to set spec.halted = false")
			_, err = to.PatchRedis(to.redis.ObjectMeta, func(in *api.Redis) *api.Redis {
				in.Spec.Halted = false
				return in
			})
			Expect(err).NotTo(HaveOccurred())

			By("Wait for Running redis")
			to.EventuallyRedisRunning(to.redis.ObjectMeta).Should(BeTrue())

			By("Delete redis")
			err = to.DeleteRedis(to.redis.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for redis to be deleted")
			to.EventuallyRedis(to.redis.ObjectMeta).Should(BeFalse())

			// Create Redis object again to resume it
			By("Create (halt) Redis: " + to.redis.Name)
			err = to.CreateRedis(to.redis)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for Running redis")
			to.EventuallyRedisRunning(to.redis.ObjectMeta).Should(BeTrue())

			By("Retrieving item from database")
			to.EventuallyGetItem(to.redis, key).Should(BeEquivalentTo(value))

		}

		It("should halt and resume successfully", shouldRunWithTerminationHalt)
	})

	Context("with TerminationPolicyDelete", func() {
		BeforeEach(func() {
			to.redis.Spec.TerminationPolicy = api.TerminationPolicyDelete
		})

		var shouldRunWithTerminationDelete = func() {

			shouldBeSuccessfullyRunning()

			By("Delete redis")
			err = to.DeleteRedis(to.redis.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			By("wait until redis is deleted")
			to.EventuallyRedis(to.redis.ObjectMeta).Should(BeFalse())

			By("Check for deleted PVCs")
			to.EventuallyPVCCount(to.redis.ObjectMeta, api.ResourceKindRedis).Should(Equal(0))
		}

		It("should run with TerminationPolicyDelete", shouldRunWithTerminationDelete)
	})

	Context("with TerminationPolicyWipeOut", func() {
		BeforeEach(func() {
			to.redis.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
		})

		var shouldRunWithTerminationWipeOut = func() {

			shouldBeSuccessfullyRunning()

			By("Delete redis")
			err = to.DeleteRedis(to.redis.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			By("wait until redis is deleted")
			to.EventuallyRedis(to.redis.ObjectMeta).Should(BeFalse())

			By("Check for deleted PVCs")
			to.EventuallyPVCCount(to.redis.ObjectMeta, api.ResourceKindRedis).Should(Equal(0))
		}

		It("should run with TerminationPolicyDelete", shouldRunWithTerminationWipeOut)
	})
})
