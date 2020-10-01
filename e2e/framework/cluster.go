package framework

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	"github.com/appscode/go/sets"
	rd "github.com/go-redis/redis"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	core_util "kmodules.xyz/client-go/core/v1"
	"kmodules.xyz/client-go/tools/portforward"
)

func (f *Framework) RedisClusterOptions() *rd.ClusterOptions {
	return &rd.ClusterOptions{
		DialTimeout:        10 * time.Second,
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		PoolSize:           10,
		PoolTimeout:        30 * time.Second,
		IdleTimeout:        500 * time.Millisecond,
		IdleCheckFrequency: 500 * time.Millisecond,
	}
}

type RedisNode struct {
	SlotStart []int
	SlotEnd   []int
	SlotsCnt  int

	ID       string
	IP       string
	Port     string
	Role     string
	Down     bool
	MasterID string

	Master *RedisNode
	Slaves []*RedisNode
}

type ClusterScenario struct {
	Nodes   [][]RedisNode
	Clients [][]*rd.Client
}

func (s *ClusterScenario) Addrs() []string {
	var addrs []string
	for i := 0; i < len(s.Nodes); i++ {
		addrs = append(addrs, "127.0.0.1:"+s.Nodes[i][0].Port)
	}
	for i := 0; i < len(s.Nodes); i++ {
		for j := 1; j < len(s.Nodes[i]); j++ {
			addrs = append(addrs, "127.0.0.1:"+s.Nodes[i][j].Port)
		}
	}

	return addrs
}

func (s *ClusterScenario) ClusterNodes(slotStart, slotEnd int) []rd.ClusterNode {
	for i := 0; i < len(s.Nodes); i++ {
		for k := 0; k < len(s.Nodes[i][0].SlotStart); k++ {
			if s.Nodes[i][0].SlotStart[k] == slotStart && s.Nodes[i][0].SlotEnd[k] == slotEnd {
				nodes := make([]rd.ClusterNode, len(s.Nodes[i]))
				for j := 0; j < len(s.Nodes[i]); j++ {
					nodes[j] = rd.ClusterNode{
						Id:   "",
						Addr: net.JoinHostPort(s.Nodes[i][j].IP, "6379"),
					}
				}

				return nodes
			}
		}
	}

	return nil
}

func (s *ClusterScenario) ClusterClient(opt *rd.ClusterOptions) *rd.ClusterClient {
	var errBadState = fmt.Errorf("cluster state is not consistent")
	opt.Addrs = s.Addrs()
	client := rd.NewClusterClient(opt)

	Eventually(func() error {
		if opt.ClusterSlots != nil {
			return nil
		}

		err := client.ForEachMaster(func(master *rd.Client) error {
			_, errp := master.Ping().Result()
			if errp != nil {
				return fmt.Errorf("%v: master(%s) ping error <-> %v", errBadState, master.String(), errp)
			}
			s := master.Info("replication").Val()
			if !strings.Contains(s, "role:master") {
				return fmt.Errorf("%v: %s is not master in role", errBadState, master.String())
			}
			return nil
		})
		if err != nil {
			return err
		}

		err = client.ForEachSlave(func(slave *rd.Client) error {
			_, errp := slave.Ping().Result()
			if errp != nil {
				return fmt.Errorf("%v: slave(%s) ping error <-> %v", errBadState, slave.String(), errp)
			}
			s := slave.Info("replication").Val()
			if !strings.Contains(s, "role:slave") {
				return fmt.Errorf("%v: %s is not slave in role", errBadState, slave.String())
			}
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	}, 5*time.Minute, 5*time.Second).Should(BeNil())

	return client
}

func (f *Framework) GetPodsIPWithTunnel(redis *api.Redis) ([][]string, [][]*portforward.Tunnel, error) {
	return FowardedPodsIPWithTunnel(f.kubeClient, f.restConfig, redis)
}

func Sync(addrs [][]string, redis *api.Redis) ([][]RedisNode, [][]*rd.Client) {
	var (
		nodes     = make([][]RedisNode, int(*redis.Spec.Cluster.Master))
		rdClients = make([][]*rd.Client, int(*redis.Spec.Cluster.Master))

		start, end int
		nodesConf  string
		slotRange  []string
		err        error
	)

	for i := 0; i < int(*redis.Spec.Cluster.Master); i++ {
		nodes[i] = make([]RedisNode, int(*redis.Spec.Cluster.Replicas)+1)
		rdClients[i] = make([]*rd.Client, int(*redis.Spec.Cluster.Replicas)+1)

		for j := 0; j <= int(*redis.Spec.Cluster.Replicas); j++ {
			rdClients[i][j] = rd.NewClient(&rd.Options{
				Addr: fmt.Sprintf(":%s", addrs[i][j]),
			})

			nodesConf, err = rdClients[i][j].ClusterNodes().Result()
			Expect(err).NotTo(HaveOccurred())

			nodesConf = strings.TrimSpace(nodesConf)
			for _, info := range strings.Split(nodesConf, "\n") {
				info = strings.TrimSpace(info)

				if strings.Contains(info, "myself") {
					parts := strings.Split(info, " ")

					node := RedisNode{
						ID:   parts[0],
						IP:   strings.Split(parts[1], ":")[0],
						Port: addrs[i][j],
					}

					if strings.Contains(parts[2], "slave") {
						node.Role = "slave"
						node.MasterID = parts[3]
					} else {
						node.Role = "master"
						node.SlotsCnt = 0

						for k := 8; k < len(parts); k++ {
							if parts[k][0] == '[' && parts[k][len(parts[k])-1] == ']' {
								continue
							}

							slotRange = strings.Split(parts[k], "-")

							// slotRange contains only int. So errors are ignored
							start, _ = strconv.Atoi(slotRange[0])
							if len(slotRange) == 1 {
								end = start
							} else {
								end, _ = strconv.Atoi(slotRange[1])
							}

							node.SlotStart = append(node.SlotStart, start)
							node.SlotEnd = append(node.SlotEnd, end)
							node.SlotsCnt += (end - start) + 1
						}
					}
					nodes[i][j] = node
					break
				}
			}
		}
	}

	return nodes, rdClients
}

func (f *Framework) WaitUntilRedisClusterConfigured(redis *api.Redis, port string) error {
	return wait.PollImmediate(time.Second*5, time.Minute*5, func() (bool, error) {
		rdClient := rd.NewClient(&rd.Options{
			Addr: fmt.Sprintf(":%s", port),
		})

		slots, err := rdClient.ClusterSlots().Result()
		if err != nil {
			return false, nil
		}

		total := 0
		masterIds := sets.NewString()
		checkReplcas := true
		for _, slot := range slots {
			total += slot.End - slot.Start + 1
			masterIds.Insert(slot.Nodes[0].Id)
			checkReplcas = checkReplcas && (len(slot.Nodes)-1 == int(*redis.Spec.Cluster.Replicas))
		}

		if total != 16384 || masterIds.Len() != int(*redis.Spec.Cluster.Master) || !checkReplcas {
			return false, nil
		}

		return true, nil
	})
}

func (f *Framework) WaitUntilStatefulSetReady(redis *api.Redis) error {
	for i := 0; i < int(*redis.Spec.Cluster.Master); i++ {
		for j := 0; j <= int(*redis.Spec.Cluster.Replicas); j++ {
			podName := fmt.Sprintf("%s-shard%d-%d", redis.Name, i, j)
			err := core_util.WaitUntilPodRunning(
				context.TODO(),
				f.kubeClient,
				metav1.ObjectMeta{
					Name:      podName,
					Namespace: redis.Namespace,
				},
			)
			if err != nil {
				return errors.Wrapf(err, "failed to ready pod '%s/%s'", redis.Namespace, podName)
			}
		}
	}

	return nil
}

func slotEqual(s1, s2 rd.ClusterSlot) bool {
	if s1.Start != s2.Start {
		return false
	}
	if s1.End != s2.End {
		return false
	}
	if len(s1.Nodes) != len(s2.Nodes) {
		return false
	}
	for i, n1 := range s1.Nodes {
		if n1.Addr != s2.Nodes[i].Addr {
			return false
		}
	}
	return true
}

func AssertSlotsEqual(slots, wanted []rd.ClusterSlot) error {
	for _, s2 := range wanted {
		ok := false
		for _, s1 := range slots {
			if slotEqual(s1, s2) {
				ok = true
				break
			}
		}
		if ok {
			continue
		}
		return fmt.Errorf("%v not found in %v", s2, slots)
	}
	return nil
}
