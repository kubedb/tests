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
	"context"
	"fmt"
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha2/util"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kmodules.xyz/client-go/tools/exec"
)

var _ = Describe("Environment Variables", func() {
	var envList []core.EnvVar
	to := testOptions{}
	testName := framework.EnvironmentVariable

	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !runTestCommunity(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `community` to test this.", testName))
		}
		if framework.SSLEnabled && !strings.HasPrefix(framework.DBVersion, "6.") {
			Skip(fmt.Sprintf("TLS is not supported for version `%s` in redis", framework.DBVersion))
		}

		to.redis = to.RedisStandalone()
		envList = []core.EnvVar{
			{
				Name:  "TEST_ENV",
				Value: "kubedb-redis-e2e",
			},
		}
		to.redis.Spec.PodTemplate.Spec.Env = envList
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
		to.EventuallyWipedOut(to.redis.ObjectMeta, api.Redis{}.ResourceFQN()).Should(Succeed())
	})

	Context("Allowed Envs", func() {
		It("should run successfully with given Env", func() {
			to.createRedis()

			By("Checking pod started with given envs")
			pod, err := to.GetPod(to.redis.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			out, err := exec.ExecIntoPod(to.Invocation.RestConfig(), pod, exec.Command("env"))
			Expect(err).NotTo(HaveOccurred())
			for _, env := range envList {
				Expect(out).Should(ContainSubstring(env.Name + "=" + env.Value))
			}

		})
	})

	Context("Update Envs", func() {
		It("should not reject to update Env", func() {
			to.createRedis()

			By("Updating Envs")
			_, _, err := util.PatchRedis(context.TODO(), to.DBClient().KubedbV1alpha2(), to.redis, func(in *api.Redis) *api.Redis {
				in.Spec.PodTemplate.Spec.Env = []core.EnvVar{
					{
						Name:  "TEST_ENV",
						Value: "patched",
					},
				}
				return in
			}, metav1.PatchOptions{})

			Expect(err).NotTo(HaveOccurred())
		})
	})
})
