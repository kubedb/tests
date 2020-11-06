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
	"strconv"
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"
	"kubedb.dev/tests/e2e/redis/testing"

	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("Redis Cluster", func() {
	var (
		err                  error
		failover             bool
		expectedClusterSlots []testing.ClusterSlot
	)

	to := testOptions{}
	testName := framework.RedisCluster

	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !runTestCommunity(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `community` to test this.", testName))
		}
		if framework.SSLEnabled && !strings.HasPrefix(framework.DBVersion, "6.") {
			Skip(fmt.Sprintf("TLS is not supported for version `%s` in redis", framework.DBVersion))
		}

		to.redis = to.RedisCluster(framework.DBVersion, nil, nil)
		to.skipMessage = ""
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

		By("Delete redis")
		err = to.DeleteRedis(to.redis.ObjectMeta)
		if err != nil {
			if kerr.IsNotFound(err) {
				// Redis was not created. Hence, rest of cleanup is not necessary.
				return
			}
			Expect(err).NotTo(HaveOccurred())
		}

		By("Wait for redis to be deleted")
		to.EventuallyRedis(to.redis.ObjectMeta).Should(BeFalse())

		By("Wait for redis resources to be wipedOut cluster")
		to.EventuallyWipedOut(to.redis.ObjectMeta, api.ResourceKindRedis).Should(Succeed())
	})

	var assertSimple = func() {
		It("should GET/SET/DEL", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			if !failover {
				res, err := to.TestConfig().GetItem(to.redis, "A")
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(Equal(""))
				_, err = to.TestConfig().SetItem(to.redis, "A", "VALUE")
				Expect(err).NotTo(HaveOccurred())
			}

			res, err := to.TestConfig().GetItem(to.redis, "A")
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal("VALUE"))
		})
	}

	Context("Cluster Commands", func() {
		BeforeEach(func() {
			to.createRedis()
			to.getConfiguredClusterInfo()
		})

		AfterEach(func() {
			_, err := to.TestConfig().FlushDBForCluster(to.redis)
			Expect(err).NotTo(HaveOccurred())
			err = to.CleanupTestResources()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should CLUSTER INFO", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			res, err := to.TestConfig().GetClusterInfoForRedis(to.redis)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(ContainSubstring(fmt.Sprintf("cluster_known_nodes:%d",
				(*to.redis.Spec.Cluster.Master)*((*to.redis.Spec.Cluster.Replicas)+1))))

			for i := 0; i < 10; i++ {
				_, err := to.TestConfig().SetItem(to.redis, fmt.Sprintf("%d", i), "")
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("calls fn for every master node", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			for i := 0; i < 10; i++ {
				res, err := to.TestConfig().SetItem(to.redis, strconv.Itoa(i), "")
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(Equal("OK"))
			}

			_, err := to.TestConfig().FlushDBForCluster(to.redis)
			Expect(err).NotTo(HaveOccurred())

			size, err := to.TestConfig().GetDbSizeForCluster(to.redis)
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(int64(0)))
		})

		It("should CLUSTER SLOTS", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			slots, err := to.TestConfig().GetClusterSlots(to.redis)
			Expect(err).NotTo(HaveOccurred())
			Expect(slots).To(HaveLen(3))

			wanted := []testing.ClusterSlot{
				{
					Start: 0,
					End:   5460,
					Nodes: to.cluster.ClusterNodes(0, 5460),
				}, {
					Start: 5461,
					End:   10922,
					Nodes: to.cluster.ClusterNodes(5461, 10922),
				}, {
					Start: 10923,
					End:   16383,
					Nodes: to.cluster.ClusterNodes(10923, 16383),
				},
			}

			Expect(to.TestConfig().AssertSlotsEqual(slots, wanted)).NotTo(HaveOccurred())
		})

		It("should CLUSTER NODES", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			res, err := to.TestConfig().GetClusterNodes(to.redis, 0, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(res)).To(BeNumerically(">", 400))
		})

		It("should CLUSTER COUNT-FAILURE-REPORTS", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			n, err := to.TestConfig().GetClusterCountFailureReports(to.redis, to.cluster.Nodes[0][0].ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(int64(0)))
		})

		It("should CLUSTER COUNTKEYSINSLOT", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			res, err := to.TestConfig().GetClusterCountKeysInSlot(to.redis, 10)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(int64(0)))
		})

		It("should CLUSTER SAVECONFIG", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			res, err := to.TestConfig().GetClusterSaveConfig(to.redis)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal("OK"))
		})

		It("should CLUSTER SLAVES", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			for i := 0; i < len(to.nodes); i++ {
				if to.nodes[i][0].Role == "master" {
					slaveList, err := to.TestConfig().GetClusterSlaves(to.redis, to.cluster.Nodes[i][0].ID)
					Expect(err).NotTo(HaveOccurred())
					Expect(slaveList).Should(ContainElement(ContainSubstring("slave")))
					Expect(slaveList).Should(HaveLen(1))
					break
				}
			}
		})

		It("should RANDOMKEY", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			const nkeys = 100

			for i := 0; i < nkeys; i++ {
				_, err := to.TestConfig().SetItem(to.redis, fmt.Sprintf("key%d", i), "value")
				Expect(err).NotTo(HaveOccurred())
			}

			var keys []string
			addKey := func(key string) {
				for _, k := range keys {
					if k == key {
						return
					}
				}
				keys = append(keys, key)
			}

			for i := 0; i < nkeys*10; i++ {
				key, err := to.TestConfig().GetRandomKey(to.redis)
				Expect(err).NotTo(HaveOccurred())
				addKey(key)
			}

			Expect(len(keys)).To(BeNumerically("~", nkeys, nkeys/10))
		})

		assertSimple()
		//assertPubSub()
	})

	Context("Cluster failover", func() {
		BeforeEach(func() {
			to.createRedis()
			failover = true

			to.getConfiguredClusterInfo()

			for i := 0; i < int(*to.redis.Spec.Cluster.Master); i++ {
				for j := 0; j <= int(*to.redis.Spec.Cluster.Replicas); j++ {
					if to.nodes[i][j].Role == "slave" {
						res, err := to.TestConfig().GetDbSizeForIndividualNodeInCluster(to.redis, i, j)
						Expect(err).NotTo(HaveOccurred())
						Expect(res).Should(Equal(int64(0)))

					}
				}
			}

			_, err = to.TestConfig().SetItem(to.redis, "A", "VALUE")
			Expect(err).NotTo(HaveOccurred())

			slots, err := to.TestConfig().GetClusterSlots(to.redis)
			Expect(err).NotTo(HaveOccurred())
			totalSlots := testing.AvailableClusterSlots(slots)
			Expect(totalSlots).To(Equal(16384))

			for i := 0; i < int(*to.redis.Spec.Cluster.Master); i++ {
				for j := 0; j <= int(*to.redis.Spec.Cluster.Replicas); j++ {
					if to.nodes[i][j].Role == "slave" {
						res, err := to.TestConfig().ClusterFailOver(to.redis, i, j)
						Expect(err).NotTo(HaveOccurred())
						Expect(res).To(Equal("OK"))
						time.Sleep(time.Second * 7)

						slots, err := to.TestConfig().GetClusterSlots(to.redis)
						Expect(err).NotTo(HaveOccurred())

						totalSlots := testing.AvailableClusterSlots(slots)
						Expect(totalSlots).To(Equal(16384))

						break
					}
				}
			}

		})

		AfterEach(func() {
			failover = false

			err = to.CleanupTestResources()
			Expect(err).NotTo(HaveOccurred())

		})

		assertSimple()
	})

	Context("Modify cluster", func() {
		It("should configure according to modified redis crd", func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}
			to.createRedis()

			By("Add replica")
			to.redis, err = to.PatchRedis(to.redis.ObjectMeta, func(in *api.Redis) *api.Redis {
				in.Spec.Cluster.Replicas = types.Int32P((*to.redis.Spec.Cluster.Replicas) + 1)
				return in
			})
			Expect(err).NotTo(HaveOccurred())

			By("Wait until statefulsets are ready")
			Expect(to.WaitUntilStatefulSetReady(to.redis)).NotTo(HaveOccurred())

			to.getConfiguredClusterInfo()
			time.Sleep(time.Minute)
			to.cluster.Nodes, err = to.TestConfig().GetClusterNodesForCluster(to.redis)
			Expect(err).NotTo(HaveOccurred())

			By("cluster slots should be configured as expected")
			expectedClusterSlots = []testing.ClusterSlot{
				{
					Start: 0,
					End:   5460,
					Nodes: to.cluster.ClusterNodes(0, 5460),
				}, {
					Start: 5461,
					End:   10922,
					Nodes: to.cluster.ClusterNodes(5461, 10922),
				}, {
					Start: 10923,
					End:   16383,
					Nodes: to.cluster.ClusterNodes(10923, 16383),
				},
			}

			res, err := to.TestConfig().GetClusterSlots(to.redis)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(HaveLen(3))
			err = to.TestConfig().AssertSlotsEqual(res, expectedClusterSlots)

			Expect(err).ShouldNot(HaveOccurred())

			// =======================================

			By("Remove replica")
			to.redis, err = to.PatchRedis(to.redis.ObjectMeta, func(in *api.Redis) *api.Redis {
				in.Spec.Cluster.Replicas = types.Int32P((*to.redis.Spec.Cluster.Replicas) - 1)
				return in
			})
			Expect(err).NotTo(HaveOccurred())

			By("Wait until statefulsets are ready")
			Expect(to.WaitUntilStatefulSetReady(to.redis)).NotTo(HaveOccurred())
			to.getConfiguredClusterInfo()

			time.Sleep(time.Minute)
			to.cluster.Nodes, err = to.TestConfig().GetClusterNodesForCluster(to.redis)
			Expect(err).NotTo(HaveOccurred())

			By("cluster slots should be configured as expected")
			expectedClusterSlots = []testing.ClusterSlot{
				{
					Start: 0,
					End:   5460,
					Nodes: to.cluster.ClusterNodes(0, 5460),
				}, {
					Start: 5461,
					End:   10922,
					Nodes: to.cluster.ClusterNodes(5461, 10922),
				}, {
					Start: 10923,
					End:   16383,
					Nodes: to.cluster.ClusterNodes(10923, 16383),
				},
			}
			res, err = to.TestConfig().GetClusterSlots(to.redis)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(HaveLen(3))
			err = to.TestConfig().AssertSlotsEqual(res, expectedClusterSlots)
			Expect(err).ShouldNot(HaveOccurred())

			// =======================================

			By("Add master")
			to.redis, err = to.PatchRedis(to.redis.ObjectMeta, func(in *api.Redis) *api.Redis {
				in.Spec.Cluster.Master = types.Int32P((*to.redis.Spec.Cluster.Master) + 1)
				return in
			})
			Expect(err).NotTo(HaveOccurred())

			By("Wait until statefulsets are ready")
			Expect(to.WaitUntilStatefulSetReady(to.redis)).NotTo(HaveOccurred())
			to.getConfiguredClusterInfo()

			time.Sleep(time.Minute)
			to.cluster.Nodes, err = to.TestConfig().GetClusterNodesForCluster(to.redis)
			Expect(err).NotTo(HaveOccurred())

			By("cluster slots should be configured as expected")
			expectedClusterSlots = []testing.ClusterSlot{
				{
					Start: 1365,
					End:   5460,
					Nodes: to.cluster.ClusterNodes(1365, 5460),
				}, {
					Start: 6827,
					End:   10922,
					Nodes: to.cluster.ClusterNodes(6827, 10922),
				}, {
					Start: 12288,
					End:   16383,
					Nodes: to.cluster.ClusterNodes(12288, 16383),
				}, {
					Start: 0,
					End:   1364,
					Nodes: to.cluster.ClusterNodes(0, 1364),
				}, {
					Start: 5461,
					End:   6826,
					Nodes: to.cluster.ClusterNodes(5461, 6826),
				}, {
					Start: 10923,
					End:   12287,
					Nodes: to.cluster.ClusterNodes(10923, 12287),
				},
			}
			res, err = to.TestConfig().GetClusterSlots(to.redis)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(HaveLen(6))
			err = to.TestConfig().AssertSlotsEqual(res, expectedClusterSlots)
			Expect(err).ShouldNot(HaveOccurred())

			// =======================================

			By("Remove master")
			to.redis, err = to.PatchRedis(to.redis.ObjectMeta, func(in *api.Redis) *api.Redis {
				in.Spec.Cluster.Master = types.Int32P((*to.redis.Spec.Cluster.Master) - 1)
				return in
			})
			Expect(err).NotTo(HaveOccurred())

			By("Wait until statefulsets are ready")
			Expect(to.WaitUntilStatefulSetReady(to.redis)).NotTo(HaveOccurred())

			to.getConfiguredClusterInfo()
			time.Sleep(time.Minute)
			to.cluster.Nodes, err = to.TestConfig().GetClusterNodesForCluster(to.redis)
			Expect(err).NotTo(HaveOccurred())

			By("cluster slots should be configured as expected")
			expectedClusterSlots = []testing.ClusterSlot{
				{
					Start: 0,
					End:   5460,
					Nodes: to.cluster.ClusterNodes(0, 5460),
				}, {
					Start: 5461,
					End:   6825,
					Nodes: to.cluster.ClusterNodes(5461, 6825),
				}, {
					Start: 6827,
					End:   10922,
					Nodes: to.cluster.ClusterNodes(6827, 10922),
				}, {
					Start: 6826,
					End:   6826,
					Nodes: to.cluster.ClusterNodes(6826, 6826),
				}, {
					Start: 10923,
					End:   16383,
					Nodes: to.cluster.ClusterNodes(10923, 16383),
				},
			}
			res, err = to.TestConfig().GetClusterSlots(to.redis)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(HaveLen(5))
			err = to.TestConfig().AssertSlotsEqual(res, expectedClusterSlots)
			Expect(err).ShouldNot(HaveOccurred())

		})
	})
})
