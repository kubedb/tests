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
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"
	"kubedb.dev/tests/e2e/redis/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
)

var (
	prevMaxClient = int32(500)
	newMaxClient  = int32(1000)
	customConfigs = []string{
		fmt.Sprintf(`maxclients %v`, prevMaxClient),
	}
	newCustomConfig = []string{
		fmt.Sprintf(`maxclients %v`, newMaxClient),
	}
	inlineConfig = fmt.Sprintf(`maxclients %v`, newMaxClient)
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
	if to.redis.Spec.Mode == api.RedisModeStandalone {
		to.EventuallySetItem(to.redis, "A", "VALUE").Should(BeTrue())
	} else {
		res, err := to.Invocation.TestConfig().GetItem(to.redis, "A")
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(Equal(""))
		_, err = to.Invocation.TestConfig().SetItem(to.redis, "A", "VALUE")
		Expect(err).NotTo(HaveOccurred())
	}
}

func (to *testOptions) getValue() {
	if to.redis.Spec.Mode == api.RedisModeStandalone {
		to.EventuallyGetItem(to.redis, "A").Should(BeEquivalentTo("VALUE"))
	} else {
		res, err := to.Invocation.TestConfig().GetItem(to.redis, "A")
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(Equal("VALUE"))
	}
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

func (to *testOptions) shouldTestOpsReq() {
	to.createRedis()
	if to.redis.Spec.Mode == api.RedisModeCluster {
		time.Sleep(30 * time.Second)
		By("Wait until redis cluster be configured")
		Expect(to.WaitUntilRedisClusterConfigured(to.redis)).NotTo(HaveOccurred())
	}

	By("Inserting item into database")
	to.setValue()

	By("Retrieving item from database")
	to.getValue()

	By("Applying the OpsRequest")
	_, err := to.CreateRedisOpsRequest(to.redisOpsReq)
	Expect(err).NotTo(HaveOccurred())

	to.EventuallyRedisOpsRequestPhase(to.redisOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

	to.redis, err = to.GetRedis(to.redis.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())

	if to.redis.Spec.Mode == api.RedisModeCluster {
		By("Wait until redis cluster be configured")
		Expect(to.WaitUntilRedisClusterConfigured(to.redis)).NotTo(HaveOccurred())
	}

	By("Checking key value after update")
	to.getValue()
}

func (to *testOptions) shouldTestConfigurationOpsReq(userConfig, newUserConfig *core.Secret, prevConfig, newConfig []string) {
	if to.skipMessage != "" {
		Skip(to.skipMessage)
	}

	By("Creating secret: " + userConfig.Name)
	_, err := to.CreateSecret(userConfig)
	Expect(err).NotTo(HaveOccurred())

	if newUserConfig != nil {
		By("Creating secret: " + newUserConfig.Name)
		_, err = to.CreateSecret(newUserConfig)
		Expect(err).NotTo(HaveOccurred())
	}

	to.createRedis()
	if to.redis.Spec.Mode == api.RedisModeCluster {
		time.Sleep(30 * time.Second)
		By("Wait until redis cluster be configured")
		Expect(to.WaitUntilRedisClusterConfigured(to.redis)).NotTo(HaveOccurred())
	}
	By("Checking initial redis configuration")
	for _, cfg := range prevConfig {
		to.EventuallyRedisConfig(to.redis, cfg).Should(Equal(cfg))
	}

	By("Inserting item into database")
	to.setValue()

	By("Retrieving item from database")
	to.getValue()

	By("Applying the OpsRequest")
	_, err = to.CreateRedisOpsRequest(to.redisOpsReq)
	Expect(err).NotTo(HaveOccurred())

	to.EventuallyRedisOpsRequestPhase(to.redisOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

	to.redis, err = to.GetRedis(to.redis.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())

	if to.redis.Spec.Mode == api.RedisModeCluster {
		By("Wait until redis cluster be configured")
		Expect(to.WaitUntilRedisClusterConfigured(to.redis)).NotTo(HaveOccurred())
	}

	By("Checking key value after update")
	to.getValue()

	By("Checking new redis configuration")
	for _, cfg := range newConfig {
		to.EventuallyRedisConfig(to.redis, cfg).Should(Equal(cfg))
	}
}

func runTestCommunity(testProfile string) bool {
	return runTestDatabaseType() && (strings.Contains(framework.TestProfiles.String(), testProfile) ||
		framework.TestProfiles.String() == framework.RedisAll ||
		framework.TestProfiles.String() == framework.RedisCommunity)
}

func runTestEnterprise(testProfile string) bool {
	return runTestDatabaseType() && (strings.Contains(framework.TestProfiles.String(), testProfile) ||
		framework.TestProfiles.String() == framework.RedisAll ||
		framework.TestProfiles.String() == framework.RedisEnterprise)
}

func runTestDatabaseType() bool {
	return strings.Compare(framework.DBType, api.ResourceKindRedis) == 0
}
