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
	"k8s.io/utils/pointer"
)

var _ = Describe("Horizontal Scaling Redis", func() {
	to := testOptions{}
	testName := framework.HorizontalScaling
	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !runTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", testName))
		}
		if framework.SSLEnabled && !strings.HasPrefix(framework.DBVersion, "6.") {
			Skip(fmt.Sprintf("TLS is not supported for version `%s` in redis", framework.DBVersion))
		}
	})

	AfterEach(func() {
		_, err := to.Invocation.TestConfig().FlushDBForCluster(to.redis)
		Expect(err).NotTo(HaveOccurred())

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
		to.EventuallyWipedOut(to.redis.ObjectMeta, api.Redis{}.ResourceFQN()).Should(Succeed())
	})

	Context("Scale up cluster master", func() {
		BeforeEach(func() {
			to.redis = to.RedisCluster(pointer.Int32Ptr(3), pointer.Int32Ptr(1))
			scalingSpec := &dbaapi.RedisHorizontalScalingSpec{
				Master:   pointer.Int32Ptr(5),
				Replicas: nil,
			}
			to.redisOpsReq = to.RedisOpsRequestHorizontalScale(to.redis.Name, to.redis.Namespace, scalingSpec)
		})

		It("Should Scale up the master of Redis Cluster", func() {
			to.shouldTestOpsReq()
		})
	})
	Context("Scale down cluster master", func() {
		BeforeEach(func() {
			to.redis = to.RedisCluster(pointer.Int32Ptr(4), pointer.Int32Ptr(1))
			scalingSpec := &dbaapi.RedisHorizontalScalingSpec{
				Master:   pointer.Int32Ptr(3),
				Replicas: nil,
			}
			to.redisOpsReq = to.RedisOpsRequestHorizontalScale(to.redis.Name, to.redis.Namespace, scalingSpec)
		})

		It("Should Scale down the master of Redis Cluster", func() {
			to.shouldTestOpsReq()
		})
	})

	Context("Scale up cluster replicas", func() {
		BeforeEach(func() {
			to.redis = to.RedisCluster(pointer.Int32Ptr(3), pointer.Int32Ptr(1))
			scalingSpec := &dbaapi.RedisHorizontalScalingSpec{
				Master:   nil,
				Replicas: pointer.Int32Ptr(3),
			}
			to.redisOpsReq = to.RedisOpsRequestHorizontalScale(to.redis.Name, to.redis.Namespace, scalingSpec)
		})

		It("Should Scale up the replicas of Redis Cluster", func() {
			to.shouldTestOpsReq()
		})
	})
	Context("Scale down cluster replicas", func() {
		BeforeEach(func() {
			to.redis = to.RedisCluster(pointer.Int32Ptr(4), pointer.Int32Ptr(2))
			scalingSpec := &dbaapi.RedisHorizontalScalingSpec{
				Master:   nil,
				Replicas: pointer.Int32Ptr(1),
			}
			to.redisOpsReq = to.RedisOpsRequestHorizontalScale(to.redis.Name, to.redis.Namespace, scalingSpec)
		})

		It("Should Scale down the replicas of Redis Cluster", func() {
			to.shouldTestOpsReq()
		})
	})

	Context("Scale up both cluster master & replicas", func() {
		BeforeEach(func() {
			to.redis = to.RedisCluster(pointer.Int32Ptr(3), pointer.Int32Ptr(1))
			scalingSpec := &dbaapi.RedisHorizontalScalingSpec{
				Master:   pointer.Int32Ptr(5),
				Replicas: pointer.Int32Ptr(1),
			}
			to.redisOpsReq = to.RedisOpsRequestHorizontalScale(to.redis.Name, to.redis.Namespace, scalingSpec)
		})

		It("Should Scale up the master & replicas of Redis Cluster", func() {
			to.shouldTestOpsReq()
		})
	})
	Context("Scale down both cluster master & replicas", func() {
		BeforeEach(func() {
			to.redis = to.RedisCluster(pointer.Int32Ptr(4), pointer.Int32Ptr(3))
			scalingSpec := &dbaapi.RedisHorizontalScalingSpec{
				Master:   pointer.Int32Ptr(3),
				Replicas: pointer.Int32Ptr(1),
			}
			to.redisOpsReq = to.RedisOpsRequestHorizontalScale(to.redis.Name, to.redis.Namespace, scalingSpec)
		})

		It("Should Scale down the master & replicas of Redis Cluster", func() {
			to.shouldTestOpsReq()
		})
	})

	Context("Scale up cluster master & Scale down cluster replicas", func() {
		BeforeEach(func() {
			to.redis = to.RedisCluster(pointer.Int32Ptr(3), pointer.Int32Ptr(3))
			scalingSpec := &dbaapi.RedisHorizontalScalingSpec{
				Master:   pointer.Int32Ptr(4),
				Replicas: pointer.Int32Ptr(1),
			}
			to.redisOpsReq = to.RedisOpsRequestHorizontalScale(to.redis.Name, to.redis.Namespace, scalingSpec)
		})

		It("Should Scale up cluster master & Scale down cluster replicas", func() {
			to.shouldTestOpsReq()
		})
	})
	Context("Scale down cluster master & Scale up cluster replicas", func() {
		BeforeEach(func() {
			to.redis = to.RedisCluster(pointer.Int32Ptr(4), pointer.Int32Ptr(2))
			scalingSpec := &dbaapi.RedisHorizontalScalingSpec{
				Master:   pointer.Int32Ptr(3),
				Replicas: pointer.Int32Ptr(3),
			}
			to.redisOpsReq = to.RedisOpsRequestHorizontalScale(to.redis.Name, to.redis.Namespace, scalingSpec)
		})

		It("Should Scale down cluster master & Scale up cluster replicas", func() {
			to.shouldTestOpsReq()
		})
	})
})
