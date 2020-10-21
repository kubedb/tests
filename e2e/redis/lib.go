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
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"
	"kubedb.dev/tests/e2e/redis/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testOptions struct {
	*framework.Invocation
	redis       *api.Redis
	redisOpsReq *dbaapi.RedisOpsRequest

	cluster *testing.ClusterNodes
	nodes   [][]*testing.RedisNode

	skipMessage string
}

func (to *testOptions) getConfiguredClusterInfo() {
	var err error

	By("Wait until redis cluster be configured")
	Expect(to.WaitUntilRedisClusterConfigured(to.redis)).NotTo(HaveOccurred())

	By("Get configured cluster info")
	to.nodes, err = to.Invocation.TestConfig().GetClusterNodesForCluster(to.redis)
	Expect(err).NotTo(HaveOccurred())
	to.cluster = &testing.ClusterNodes{
		Nodes: to.nodes,
	}
}

func (to *testOptions) setValue() {
	res, err := to.Invocation.TestConfig().GetItem(to.redis, "A")
	Expect(err).NotTo(HaveOccurred())
	Expect(res).To(Equal(""))
	_, err = to.Invocation.TestConfig().SetItem(to.redis, "A", "VALUE")
	Expect(err).NotTo(HaveOccurred())
}

func (to *testOptions) getValue() {
	res, err := to.Invocation.TestConfig().GetItem(to.redis, "A")
	Expect(err).NotTo(HaveOccurred())
	Expect(res).To(Equal("VALUE"))
}

// spin up the expected redis for testing
func (to *testOptions) createRedis() {
	By("Create Redis: " + to.redis.Name)
	err := to.CreateRedis(to.redis)
	Expect(err).NotTo(HaveOccurred())

	By("Wait for Running redis")
	to.EventuallyRedisRunning(to.redis.ObjectMeta).Should(BeTrue())

	By("Wait for AppBinding to create")
	to.EventuallyAppBinding(to.redis.ObjectMeta).Should(BeTrue())

	By("Check valid AppBinding Specs")
	err = to.CheckRedisAppBindingSpec(to.redis.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())
}

func (to *testOptions) shouldTestClusterOpsReq() {
	to.createRedis()
	time.Sleep(1 * time.Minute)

	to.getConfiguredClusterInfo()

	By("Set Item Inside DB")
	to.setValue()

	By("Checking key value")
	to.getValue()

	By("Applying the OpsRequest")
	_, err := to.CreateRedisOpsRequest(to.redisOpsReq)
	Expect(err).NotTo(HaveOccurred())

	to.EventuallyRedisOpsRequestPhase(to.redisOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

	to.redis, err = to.GetRedis(to.redis.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())

	to.getConfiguredClusterInfo()

	By("Checking key value after update")
	to.getValue()
}

func runTestCommunity(testProfile string) bool {
	return strings.Contains(framework.TestProfiles.String(), testProfile) ||
		framework.TestProfiles.String() == framework.RedisAll ||
		framework.TestProfiles.String() == framework.RedisCommunity
}

func runTestEnterprise(testProfile string) bool {
	return strings.Contains(framework.TestProfiles.String(), testProfile) ||
		framework.TestProfiles.String() == framework.RedisAll ||
		framework.TestProfiles.String() == framework.RedisEnterprise
}
