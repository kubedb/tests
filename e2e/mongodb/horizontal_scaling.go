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

package e2e_test

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

var _ = Describe("Horizontal Scaling", func() {
	to := testOptions{}
	testName := framework.HorizontalScaling
	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !framework.RunTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", testName))
		}
	})

	AfterEach(func() {
		err := to.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())
		//Delete MongoDB
		By("Delete mongodb")
		err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Delete mongodb ops request")
		err = to.DeleteMongoDBOpsRequest(to.mongoOpsReq.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb resources to be wipedOut")
		to.EventuallyWipedOut(to.mongodb.ObjectMeta, api.MongoDB{}.ResourceFQN()).Should(Succeed())
	})
	Context("Scale Up Shard Replica", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			shard := dbaapi.MongoDBShardNode{
				Shards:   0,
				Replicas: 4,
			}
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, &shard, nil, nil, nil)
		})

		It("Should Scale Up Shard Replica", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scale Down Shard Replica", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			shard := dbaapi.MongoDBShardNode{
				Shards:   0,
				Replicas: 2,
			}
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, &shard, nil, nil, nil)
		})

		It("Should Scale Down Shard Replica", func() {
			to.shouldTestOpsRequest()
		})

	})

	Context("Scale Up Shard", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			shard := dbaapi.MongoDBShardNode{
				Shards:   3,
				Replicas: 0,
			}
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, &shard, nil, nil, nil)
		})

		It("Should Scale Up Shard", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scale Down Shard", func() {
		Context("Without Database Primary Shard", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.ShardTopology.Shard.Shards = 3
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				shard := dbaapi.MongoDBShardNode{
					Shards:   2,
					Replicas: 0,
				}
				to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, &shard, nil, nil, nil)
			})

			It("Should Scale Down Shard", func() {
				to.shouldTestOpsRequest()
			})

		})

		Context("With Database Primary Shard", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.ShardTopology.Shard.Shards = 3
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				shard := dbaapi.MongoDBShardNode{
					Shards:   2,
					Replicas: 0,
				}
				to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, &shard, nil, nil, nil)
			})

			It("Should Scale Down Shard", func() {
				to.shouldTestOpsRequest()
			})

		})
	})

	Context("Scale Up Shard & Replica", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			shard := dbaapi.MongoDBShardNode{
				Shards:   3,
				Replicas: 4,
			}
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, &shard, nil, nil, nil)
		})

		It("Should Scale Up Shard and Replica", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scale Down Shard & Replica", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.ShardTopology.Shard.Shards = 3
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			shard := dbaapi.MongoDBShardNode{
				Shards:   2,
				Replicas: 2,
			}
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, &shard, nil, nil, nil)
		})

		It("Should Scale Down Shard and Replica", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scale Down Shard & Scale Up Replica", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.ShardTopology.Shard.Shards = 3
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			shard := dbaapi.MongoDBShardNode{
				Shards:   2,
				Replicas: 4,
			}
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, &shard, nil, nil, nil)
		})

		It("Should Scale Down Shard & Scale Up Replica", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scale Up Shard & Scale Down Replica", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			shard := dbaapi.MongoDBShardNode{
				Shards:   3,
				Replicas: 2,
			}
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, &shard, nil, nil, nil)
		})

		It("Should Scale Up Shard & Scale Down Replica", func() {
			to.shouldTestOpsRequest()
		})

	})

	Context("Scale Up Shard, Shard Replicas, ConfigServer and Mongos", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			shard := dbaapi.MongoDBShardNode{
				Shards:   3,
				Replicas: 4,
			}
			confgSrvr := dbaapi.ConfigNode{
				Replicas: 4,
			}
			mongos := dbaapi.MongosNode{
				Replicas: 3,
			}
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, &shard, &confgSrvr, &mongos, nil)
		})

		It("Should Scale Up Shard, Shard Replicas, ConfigServer and Mongos", func() {
			to.shouldTestOpsRequest()
		})
	})
	Context("Scale Down Shard, Shard Replicas, ConfigServer and Mongos", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			to.mongodb.Spec.ShardTopology.Shard.Shards = 3
			to.mongodb.Spec.ShardTopology.Shard.Replicas = 3
			to.mongodb.Spec.ShardTopology.Mongos.Replicas = 3
			to.mongodb.Spec.ShardTopology.ConfigServer.Replicas = 3
			shard := dbaapi.MongoDBShardNode{
				Shards:   2,
				Replicas: 2,
			}
			confgSrvr := dbaapi.ConfigNode{
				Replicas: 2,
			}
			mongos := dbaapi.MongosNode{
				Replicas: 2,
			}
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, &shard, &confgSrvr, &mongos, nil)
		})

		It("Scale Up Shard, Shard Replicas, ConfigServer and Mongos", func() {
			to.shouldTestOpsRequest()
		})
	})

	Context("Scale Up ConfigServer Replica", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			confgSrvr := dbaapi.ConfigNode{
				Replicas: 4,
			}
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, nil, &confgSrvr, nil, nil)
		})

		It("Should Scale Up ConfigServer Replica", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scale Down ConfigServer Replica", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.ShardTopology.ConfigServer.Replicas = 3
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			confgSrvr := dbaapi.ConfigNode{
				Replicas: 2,
			}
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, nil, &confgSrvr, nil, nil)
		})

		It("Should Scale Down ConfigServer Replica", func() {
			to.shouldTestOpsRequest()
		})

	})

	Context("Scale Up Mongos Replica", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			mongos := dbaapi.MongosNode{
				Replicas: 3,
			}
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, &mongos, nil)
		})

		It("Should Scale Up Mongos Replica", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scale Down Mongos Replica", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.ShardTopology.Mongos.Replicas = 3
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			mongos := dbaapi.MongosNode{
				Replicas: 2,
			}
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, &mongos, nil)
		})

		It("Should Scale Down Mongos Replica", func() {
			to.shouldTestOpsRequest()
		})

	})

	Context("Scale Up Mongodb ReplicaSet", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBRS()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, nil, pointer.Int32Ptr(4))
		})

		It("Should Scale Up Mongodb ReplicaSet", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scale Down Mongodb ReplicaSet", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBRS()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.Replicas = types.Int32P(3)
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			to.mongoOpsReq = to.MongoDBOpsRequestHorizontalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, nil, pointer.Int32Ptr(2))
		})

		It("Should Scale Down Mongodb ReplicaSet", func() {
			to.shouldTestOpsRequest()
		})

	})
})
