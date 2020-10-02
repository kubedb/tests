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
	"kubedb.dev/tests/e2e/redis/testing"
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"kmodules.xyz/client-go/tools/portforward"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"
	"strings"
	"time"
)

type testOptions struct {
	*framework.Invocation
	redis       *api.Redis
	redisOpsReq *dbaapi.RedisOpsRequest

	//opt       *rd.ClusterOptions
	//client    *rd.ClusterClient
	cluster   *testing.ClusterNodes
	//ports     [][]string
	//tunnels   [][]*portforward.Tunnel
	//nodes     [][]framework.RedisNode
	//rdClients [][]*rd.Client
}

func (to *testOptions) getConfiguredClusterInfo() {
	//var err error
	//By("Forward ports")
	//to.ports, to.tunnels, err = to.GetPodsIPWithTunnel(to.redis)
	//Expect(err).NotTo(HaveOccurred())

	By("Wait until redis cluster be configured")
	//Expect(to.WaitUntilRedisClusterConfigured(to.redis, to.ports[0][0])).NotTo(HaveOccurred())
	Expect(to.WaitUntilRedisClusterConfigured(to.redis)).NotTo(HaveOccurred())

	By("Get configured cluster info")
	//cluster = framework.Sync(ports, cl.redis)
	//to.nodes, to.rdClients = framework.Sync(to.ports, to.redis)
	//to.cluster = &framework.ClusterScenario{
	//	Nodes:   to.nodes,
	//	Clients: to.rdClients,
	//}
	nodes, err := to.Invocation.TestConfig().GetClusterNodesForCluster(to.redis)
	Expect(err).NotTo(HaveOccurred())
	to.cluster = &testing.ClusterNodes{
		Nodes: nodes,
	}
}

//func (to *testOptions) clusterSlots() ([]rd.ClusterSlot, error) {
//	var slots []rd.ClusterSlot
//	for i := range to.nodes {
//		for k := range to.nodes[i][0].SlotStart {
//			slot := rd.ClusterSlot{
//				Start: to.nodes[i][0].SlotStart[k],
//				End:   to.nodes[i][0].SlotEnd[k],
//				Nodes: make([]rd.ClusterNode, len(to.nodes[i])),
//			}
//			for j := 0; j < len(to.nodes[i]); j++ {
//				slot.Nodes[j] = rd.ClusterNode{
//					Addr: ":" + to.nodes[i][j].Port,
//				}
//			}
//
//			slots = append(slots, slot)
//		}
//	}
//
//	return slots, nil
//}

//func (to *testOptions) initializeCLusterClient() {
//	By("Initializing cluster client")
//	err := to.client.ForEachMaster(func(master *rd.Client) error {
//		return master.FlushDB().Err()
//	})
//	Expect(err).NotTo(HaveOccurred())
//}

//func (to *testOptions) createClusterClient() {
//	By(fmt.Sprintf("Creating cluster client using ports %v", to.ports))
//	to.opt = &rd.ClusterOptions{
//		ClusterSlots:  to.clusterSlots,
//		RouteRandomly: true,
//	}
//	to.client = to.cluster.ClusterClient(to.opt)
//	Expect(to.client.ReloadState()).NotTo(HaveOccurred())
//}

//func (to *testOptions) createAndInitializeClusterClient() {
//	to.createClusterClient()
//	to.initializeCLusterClient()
//}

//func (to *testOptions) closeExistingTunnels() {
//	By("closing tunnels")
//	for i := range to.tunnels {
//		for j := range to.tunnels[i] {
//			to.tunnels[i][j].Close()
//		}
//	}
//}

func (to *testOptions) setValue() {
	//var err error
	//res := to.client.Get("A").Val()
	//Expect(res).To(Equal(""))
	//err = to.client.Set("A", "VALUE", 0).Err()
	//Expect(err).NotTo(HaveOccurred())

	res, err := to.Invocation.TestConfig().GetItem(to.redis, "A")
	Expect(err).NotTo(HaveOccurred())
	Expect(res).To(Equal(""))
	_, err = to.Invocation.TestConfig().SetItem(to.redis, "A", "VALUE")
	Expect(err).NotTo(HaveOccurred())
}

func (to *testOptions) getValue() {
	//Eventually(func() string {
	//	return to.client.Get("A").Val()
	//}, 30*time.Second).Should(Equal("VALUE"))

	res, err := to.Invocation.TestConfig().GetItem(to.redis, "A")
	Expect(err).NotTo(HaveOccurred())
	Expect(res).To(Equal("VALUE"))
}

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
	// Create Redis
	to.createRedis()
	time.Sleep(1 * time.Minute)

	to.getConfiguredClusterInfo()
	//to.createAndInitializeClusterClient()

	// Insert Data
	By("Set Item Inside DB")
	to.setValue()

	// Retrieve Inserted Data
	By("Checking key value")
	to.getValue()

	// Scaling Database
	By("Applying the OpsRequest")
	_, err := to.CreateRedisOpsRequest(to.redisOpsReq)
	Expect(err).NotTo(HaveOccurred())

	to.EventuallyRedisOpsRequestPhase(to.redisOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

	to.redis, err = to.GetRedis(to.redis.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())

	//Expect(to.client.Close()).NotTo(HaveOccurred())
	//to.closeExistingTunnels()

	to.getConfiguredClusterInfo()
	//to.createClusterClient()

	// Retrieve Inserted Data
	By("Checking key value after update")
	to.getValue()
}
func runTestCommunity(testProfile string) bool {
	return strings.Contains(framework.TestProfiles.String(), testProfile) ||
		framework.TestProfiles.String() == framework.All ||
		framework.TestProfiles.String() == framework.Community
}

func runTestEnterprise(testProfile string) bool {
	return strings.Contains(framework.TestProfiles.String(), testProfile) ||
		framework.TestProfiles.String() == framework.All ||
		framework.TestProfiles.String() == framework.Enterprise
}
