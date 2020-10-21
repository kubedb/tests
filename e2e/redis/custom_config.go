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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("Custom config Redis", func() {
	var customConfigs []string
	to := testOptions{}
	testName := framework.RedisCustomConfig

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
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	AfterEach(func() {
		err := to.CleanupTestResources()
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

	customConfigs = []string{
		"databases 10",
		"maxclients 500",
	}

	Context("from secret", func() {
		var userConfig *core.Secret

		BeforeEach(func() {
			userConfig = to.GetCustomConfigRedis(customConfigs)
		})

		AfterEach(func() {
			By("Deleting secret: " + userConfig.Name)
			err := to.DeleteSecret(userConfig.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set configuration provided in secret", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			By("Creating secret: " + userConfig.Name)
			_, err := to.CreateSecret(userConfig)
			Expect(err).NotTo(HaveOccurred())

			to.redis.Spec.ConfigSecret = &core.LocalObjectReference{
				Name: userConfig.Name,
			}

			to.createRedis()

			By("Checking redis configured from provided custom configuration")
			for _, cfg := range customConfigs {
				to.EventuallyRedisConfig(to.redis, cfg).Should(Equal(cfg))
			}
		})
	})

})
