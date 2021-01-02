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

var _ = Describe("StorageType Redis", func() {
	var (
		err   error
		key   string
		value string
	)
	to := testOptions{}
	testName := framework.RedisStorageType

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
		to.EventuallyWipedOut(to.redis.ObjectMeta, api.Redis{}.ResourceFQN()).Should(Succeed())
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

	Context("Ephemeral", func() {

		BeforeEach(func() {
			to.redis.Spec.StorageType = api.StorageTypeEphemeral
			to.redis.Spec.Storage = nil
		})

		Context("General Behaviour", func() {

			BeforeEach(func() {
				to.redis.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			})

			It("should run successfully", shouldBeSuccessfullyRunning)
		})

		Context("With TerminationPolicyHalt", func() {

			BeforeEach(func() {
				to.redis.Spec.TerminationPolicy = api.TerminationPolicyHalt
			})

			It("should reject to create Redis object", func() {

				By("Creating Redis: " + to.redis.Name)
				err := to.CreateRedis(to.redis)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
