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

package framework

import (
	"context"
	"fmt"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	"github.com/appscode/go/sets"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	core_util "kmodules.xyz/client-go/core/v1"
)

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

func (f *Framework) WaitUntilRedisClusterConfigured(redis *api.Redis) error {
	return wait.PollImmediate(time.Second*5, time.Minute*5, func() (bool, error) {
		slots, err := f.testConfig.GetClusterSlots(redis)
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
