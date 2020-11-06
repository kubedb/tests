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
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Redis Volume Expansion", func() {
	to := testOptions{}
	testName := framework.RedisVolumeExpansion
	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !to.IsGKE() {
			to.skipMessage = "volume expansion testing is only supported in GKE"
		}
		if !runTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", testName))
		}
		if framework.SSLEnabled && !strings.HasPrefix(framework.DBVersion, "6.") {
			Skip(fmt.Sprintf("TLS is not supported for version `%s` in redis", framework.DBVersion))
		}
	})

	AfterEach(func() {
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

		//Delete Redis
		By("Delete redis")
		err = to.DeleteRedis(to.redis.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Delete RedisOpsRequest")
		err = to.DeleteRedisOpsRequest(to.redisOpsReq.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for redis resources to be wipedOut")
		to.EventuallyWipedOut(to.redis.ObjectMeta, api.ResourceKindRedis).Should(Succeed())
	})

	Context("Volume Expansion in StandAlone Redis", func() {
		BeforeEach(func() {
			to.redis = to.RedisStandalone(framework.DBVersion)
			storageReq := resource.MustParse("2Gi")
			to.redisOpsReq = to.RedisOpsRequestVolumeExpansion(to.redis.Name, to.redis.Namespace, &storageReq)
		})

		It("Should Expand StandAlone Redis", func() {
			var err error
			// Create Redis
			to.createRedis()

			By("Inserting item into database")
			to.EventuallySetItem(to.redis, "A", "VALUE").Should(BeTrue())

			By("Retrieving item from database")
			to.EventuallyGetItem(to.redis, "A").Should(BeEquivalentTo("VALUE"))

			// Scaling Database
			By("Expanding Volume")
			to.redisOpsReq, err = to.CreateRedisOpsRequest(to.redisOpsReq)
			Expect(err).NotTo(HaveOccurred())

			to.EventuallyRedisOpsRequestPhase(to.redisOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

			// Retrieve Inserted Data
			By("Checking key value after update")
			to.EventuallyGetItem(to.redis, "A").Should(BeEquivalentTo("VALUE"))
		})
	})

	Context("Volume Expansion in Redis Cluster", func() {
		BeforeEach(func() {
			to.redis = to.RedisCluster(framework.DBVersion, nil, nil)
			storageReq := resource.MustParse("2Gi")
			to.redisOpsReq = to.RedisOpsRequestVolumeExpansion(to.redis.Name, to.redis.Namespace, &storageReq)
		})

		AfterEach(func() {
			_, err := to.Invocation.TestConfig().FlushDBForCluster(to.redis)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should Scale Resources of Redis Cluster", func() {
			to.shouldTestClusterOpsReq()
		})
	})
})
