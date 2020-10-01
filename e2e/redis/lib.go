package redis

import (
	"fmt"
	rd "github.com/go-redis/redis"
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

	opt       *rd.ClusterOptions
	client    *rd.ClusterClient
	cluster   *framework.ClusterScenario
	ports     [][]string
	tunnels   [][]*portforward.Tunnel
	nodes     [][]framework.RedisNode
	rdClients [][]*rd.Client
}

func (to *testOptions) getConfiguredClusterInfo() {
	var err error
	By("Forward ports")
	to.ports, to.tunnels, err = to.GetPodsIPWithTunnel(to.redis)
	Expect(err).NotTo(HaveOccurred())

	By("Wait until redis cluster be configured")
	Expect(to.WaitUntilRedisClusterConfigured(to.redis, to.ports[0][0])).NotTo(HaveOccurred())

	By("Get configured cluster info")
	//cluster = framework.Sync(ports, cl.redis)
	to.nodes, to.rdClients = framework.Sync(to.ports, to.redis)
	to.cluster = &framework.ClusterScenario{
		Nodes:   to.nodes,
		Clients: to.rdClients,
	}
}

func(to *testOptions) clusterSlots() ([]rd.ClusterSlot, error) {
	var slots []rd.ClusterSlot
	for i := range to.nodes {
		for k := range to.nodes[i][0].SlotStart {
			slot := rd.ClusterSlot{
				Start: to.nodes[i][0].SlotStart[k],
				End:   to.nodes[i][0].SlotEnd[k],
				Nodes: make([]rd.ClusterNode, len(to.nodes[i])),
			}
			for j := 0; j < len(to.nodes[i]); j++ {
				slot.Nodes[j] = rd.ClusterNode{
					Addr: ":" + to.nodes[i][j].Port,
				}
			}

			slots = append(slots, slot)
		}
	}

	return slots, nil
}

func(to *testOptions) initializeCLusterClient() {
	By("Initializing cluster client")
	err := to.client.ForEachMaster(func(master *rd.Client) error {
		return master.FlushDB().Err()
	})
	Expect(err).NotTo(HaveOccurred())
}

func(to *testOptions) createClusterClient() {
	By(fmt.Sprintf("Creating cluster client using ports %v", to.ports))
	to.opt = &rd.ClusterOptions{
		ClusterSlots:  to.clusterSlots,
		RouteRandomly: true,
	}
	to.client = to.cluster.ClusterClient(to.opt)
	Expect(to.client.ReloadState()).NotTo(HaveOccurred())
}

func (to *testOptions) createAndInitializeClusterClient() {
	to.createClusterClient()
	to.initializeCLusterClient()
}

func (to *testOptions) closeExistingTunnels() {
	By("closing tunnels")
	for i := range to.tunnels {
		for j := range to.tunnels[i] {
			to.tunnels[i][j].Close()
		}
	}
}

func (to *testOptions) setValue() {
	var err error
	res := to.client.Get("A").Val()
	Expect(res).To(Equal(""))
	err = to.client.Set("A", "VALUE", 0).Err()
	Expect(err).NotTo(HaveOccurred())
}

func (to *testOptions) getValue() {
	Eventually(func() string {
		return to.client.Get("A").Val()
	}, 30*time.Second).Should(Equal("VALUE"))
}

func (to *testOptions) createRedis () {
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

func (to *testOptions) shouldTestHorizontalOpsReq() {
	// Create Redis
	to.createRedis()

	to.getConfiguredClusterInfo()
	to.createAndInitializeClusterClient()

	// Insert Data
	By("Set Item Inside DB")
	to.setValue()

	// Retrieve Inserted Data
	By("Checking key value")
	to.getValue()

	// Scaling Database
	By("Horizontal Scaling Redis")
	_, err := to.CreateRedisOpsRequest(to.redisOpsReq)
	Expect(err).NotTo(HaveOccurred())

	to.EventuallyRedisOpsRequestPhase(to.redisOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

	to.redis, err = to.GetRedis(to.redis.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())

	Expect(to.client.Close()).NotTo(HaveOccurred())
	to.closeExistingTunnels()

	to.getConfiguredClusterInfo()
	to.createClusterClient()

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