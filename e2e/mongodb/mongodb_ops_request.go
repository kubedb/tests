/*
Copyright The KubeDB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package e2e_test

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/test/e2e/framework"

	"github.com/appscode/go/types"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
	kmapi "kmodules.xyz/client-go/api/v1"
)

var _ = Describe("MongoDB", func() {
	var (
		f           *framework.Invocation
		mongodb     *api.MongoDB
		mongoOpsReq *dbaapi.MongoDBOpsRequest
		skipMessage string
	)

	dbName := "kubedbtest"

	var createMongodb = func(mongodb *api.MongoDB) {
		if skipMessage != "" {
			Skip(skipMessage)
		}

		if framework.SSLEnabled {
			issuer, err := f.InsureIssuer(mongodb.ObjectMeta, api.ResourceKindMongoDB)
			Expect(err).NotTo(HaveOccurred())
			mongodb.Spec.SSLMode = api.SSLModeRequireSSL
			mongodb.Spec.TLS = &kmapi.TLSConfig{
				IssuerRef: &v1.TypedLocalObjectReference{
					Name:     issuer.Name,
					Kind:     "Issuer",
					APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group),
				},
			}
		}

		By("Create MongoDB: " + mongodb.Name)
		err := f.CreateMongoDB(mongodb)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running mongodb")
		f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

		By("Wait for AppBinding to create")
		f.EventuallyAppBinding(mongodb.ObjectMeta).Should(BeTrue())

		By("Check valid AppBinding Specs")
		err = f.CheckMongoDBAppBindingSpec(mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Ping mongodb database")
		f.EventuallyPingMongo(mongodb.ObjectMeta)
	}

	var shouldTestOpsRequest = func() {
		// Create MongoDB
		createMongodb(mongodb)

		// Insert Data
		By("Insert Document Inside DB")
		f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

		By("Checking Inserted Document")
		f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

		// Update Database
		By("Updating MongoDB")
		err := f.CreateMongoDBOpsRequest(mongoOpsReq)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for MongoDB Ops Request Phase to be Successful")
		f.EventuallyMongoDBOpsRequestPhase(mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

		// Retrieve Inserted Data
		By("Checking Inserted Document after update")
		f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())
	}

	BeforeEach(func() {
		f = framework.NewInvocation()
	})

	AfterEach(func() {
		err := f.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		//Delete MongoDB
		By("Delete mongodb")
		err := f.DeleteMongoDB(mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Delete mongodb ops request")
		err = f.DeleteMongoDBOpsRequest(mongoOpsReq.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb resources to be wipedOut")
		f.EventuallyWipedOut(mongodb.ObjectMeta).Should(Succeed())
	})

	Context("Update Database Version", func() {
		Context("Update Standalone DB", func() {
			BeforeEach(func() {
				mongodb = f.MongoDBStandalone()
				mongodb.Spec.Version = framework.MongoDBCatalogName
				mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				mongoOpsReq = f.MongoDBOpsRequestUpgrade(mongodb.Name, mongodb.Namespace, framework.MongoDBUpdatedCatalogName)
			})

			It("Should Update MongoDB version", func() {
				shouldTestOpsRequest()
			})

		})

		Context("Update Non-Sharded Cluster", func() {
			BeforeEach(func() {
				mongodb = f.MongoDBRS()
				mongodb.Spec.Version = framework.MongoDBCatalogName
				mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				mongoOpsReq = f.MongoDBOpsRequestUpgrade(mongodb.Name, mongodb.Namespace, framework.MongoDBUpdatedCatalogName)
			})

			It("Should Update MongoDB version", func() {
				shouldTestOpsRequest()
			})

		})

		Context("Update Sharded Cluster", func() {
			BeforeEach(func() {
				mongodb = f.MongoDBShard()
				mongodb.Spec.Version = framework.MongoDBCatalogName
				mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				mongoOpsReq = f.MongoDBOpsRequestUpgrade(mongodb.Name, mongodb.Namespace, framework.MongoDBUpdatedCatalogName)
			})

			It("Should Update MongoDB version", func() {
				shouldTestOpsRequest()
			})

		})
	})

	Context("Scale Database", func() {
		Context("Horizontal Scaling", func() {
			Context("Scale Up Shard Replica", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					shard := dbaapi.MongoDBShardNode{
						Shards:   0,
						Replicas: 3,
					}
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, &shard, nil, nil, nil)
				})

				It("Should Scale Up Shard Replica", func() {
					shouldTestOpsRequest()
				})

			})
			Context("Scale Down Shard Replica", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					shard := dbaapi.MongoDBShardNode{
						Shards:   0,
						Replicas: 1,
					}
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, &shard, nil, nil, nil)
				})

				It("Should Scale Down Shard Replica", func() {
					shouldTestOpsRequest()
				})

			})

			Context("Scale Up Shard", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					shard := dbaapi.MongoDBShardNode{
						Shards:   3,
						Replicas: 0,
					}
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, &shard, nil, nil, nil)
				})

				It("Should Scale Up Shard", func() {
					shouldTestOpsRequest()
				})

			})
			Context("Scale Down Shard", func() {
				Context("Without Database Primary Shard", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.Version = framework.MongoDBCatalogName
						mongodb.Spec.ShardTopology.Shard.Shards = 3
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
						shard := dbaapi.MongoDBShardNode{
							Shards:   2,
							Replicas: 0,
						}
						mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, &shard, nil, nil, nil)
					})

					It("Should Scale Down Shard", func() {
						shouldTestOpsRequest()
					})

				})

				Context("With Database Primary Shard", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.Version = framework.MongoDBCatalogName
						mongodb.Spec.ShardTopology.Shard.Shards = 3
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
						shard := dbaapi.MongoDBShardNode{
							Shards:   2,
							Replicas: 0,
						}
						mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, &shard, nil, nil, nil)
					})

					It("Should Scale Down Shard", func() {
						// Create MongoDB
						createMongodb(mongodb)

						// Insert Data
						By("Insert Document Inside DB")
						f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

						By("Checking Inserted Document")
						f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

						By("Moving Primary")
						err := f.MovePrimary(mongodb.ObjectMeta, dbName)
						Expect(err).NotTo(HaveOccurred())

						// Update Database
						By("Updating MongoDB")
						err = f.CreateMongoDBOpsRequest(mongoOpsReq)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for MongoDB Ops Request Phase to be Successful")
						f.EventuallyMongoDBOpsRequestPhase(mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

						// Retrieve Inserted Data
						By("Checking Inserted Document after update")
						f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

					})

				})
			})

			Context("Scale Up Shard & Replica", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					shard := dbaapi.MongoDBShardNode{
						Shards:   3,
						Replicas: 3,
					}
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, &shard, nil, nil, nil)
				})

				It("Should Scale Up Shard and Replica", func() {
					shouldTestOpsRequest()
				})

			})
			Context("Scale Down Shard & Replica", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.ShardTopology.Shard.Shards = 3
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					shard := dbaapi.MongoDBShardNode{
						Shards:   2,
						Replicas: 1,
					}
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, &shard, nil, nil, nil)
				})

				It("Should Scale Down Shard and Replica", func() {
					shouldTestOpsRequest()
				})

			})
			Context("Scale Down Shard & Scale Up Replica", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.ShardTopology.Shard.Shards = 3
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					shard := dbaapi.MongoDBShardNode{
						Shards:   2,
						Replicas: 3,
					}
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, &shard, nil, nil, nil)
				})

				It("Should Scale Down Shard & Scale Up Replica", func() {
					shouldTestOpsRequest()

				})

			})
			Context("Scale Up Shard & Scale Down Replica", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					shard := dbaapi.MongoDBShardNode{
						Shards:   3,
						Replicas: 1,
					}
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, &shard, nil, nil, nil)
				})

				It("Should Scale Up Shard & Scale Down Replica", func() {
					shouldTestOpsRequest()

				})

			})

			Context("Scale Up ConfigServer Replica", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					confgSrvr := dbaapi.ConfigNode{
						Replicas: 3,
					}
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, nil, &confgSrvr, nil, nil)
				})

				It("Should Scale Up ConfigServer Replica", func() {
					shouldTestOpsRequest()

				})

			})
			Context("Scale Down ConfigServer Replica", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.ShardTopology.ConfigServer.Replicas = 3
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					confgSrvr := dbaapi.ConfigNode{
						Replicas: 2,
					}
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, nil, &confgSrvr, nil, nil)
				})

				It("Should Scale Down ConfigServer Replica", func() {
					shouldTestOpsRequest()

				})

			})

			Context("Scale Up Mongos Replica", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					mongos := dbaapi.MongosNode{
						Replicas: 3,
					}
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, nil, nil, &mongos, nil)
				})

				It("Should Scale Up Mongos Replica", func() {
					shouldTestOpsRequest()

				})

			})
			Context("Scale Down Mongos Replica", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.ShardTopology.Mongos.Replicas = 3
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					mongos := dbaapi.MongosNode{
						Replicas: 2,
					}
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, nil, nil, &mongos, nil)
				})

				It("Should Scale Down Mongos Replica", func() {
					shouldTestOpsRequest()

				})

			})

			Context("Scale Up Mongodb ReplicaSet", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBRS()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, nil, nil, nil, pointer.Int32Ptr(3))
				})

				It("Should Scale Up Mongodb ReplicaSet", func() {
					shouldTestOpsRequest()

				})

			})
			Context("Scale Down Mongodb ReplicaSet", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBRS()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.Replicas = types.Int32P(3)
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					mongoOpsReq = f.MongoDBOpsRequestHorizontalScale(mongodb.Name, mongodb.Namespace, nil, nil, nil, pointer.Int32Ptr(2))
				})

				It("Should Scale Down Mongodb ReplicaSet", func() {
					shouldTestOpsRequest()

				})

			})
		})

		Context("Vertical Scaling", func() {
			Context("Scaling StandAlone Mongodb Resources", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBStandalone()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					standalone := &v1.ResourceRequirements{
						Limits: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("300Mi"),
							v1.ResourceCPU:    resource.MustParse(".2"),
						},
						Requests: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("200Mi"),
							v1.ResourceCPU:    resource.MustParse(".1"),
						},
					}
					mongoOpsReq = f.MongoDBOpsRequestVerticalScale(mongodb.Name, mongodb.Namespace, standalone, nil, nil, nil, nil, nil)
				})

				It("Should Scale StandAlone Mongodb Resources", func() {
					shouldTestOpsRequest()

				})

			})
			Context("Scaling ReplicaSet Resources", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBRS()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					replicaset := &v1.ResourceRequirements{
						Limits: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("300Mi"),
							v1.ResourceCPU:    resource.MustParse(".2"),
						},
						Requests: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("200Mi"),
							v1.ResourceCPU:    resource.MustParse(".1"),
						},
					}
					mongoOpsReq = f.MongoDBOpsRequestVerticalScale(mongodb.Name, mongodb.Namespace, nil, replicaset, nil, nil, nil, nil)
				})

				It("Should Scale ReplicaSet Resources", func() {
					shouldTestOpsRequest()

				})

			})
			Context("Scaling Mongos Resources", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					mongos := &v1.ResourceRequirements{
						Limits: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("300Mi"),
							v1.ResourceCPU:    resource.MustParse(".2"),
						},
						Requests: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("200Mi"),
							v1.ResourceCPU:    resource.MustParse(".1"),
						},
					}
					mongoOpsReq = f.MongoDBOpsRequestVerticalScale(mongodb.Name, mongodb.Namespace, nil, nil, mongos, nil, nil, nil)
				})

				It("Should Scale Mongos Resources", func() {
					shouldTestOpsRequest()

				})

			})
			Context("Scaling ConfigServer Resources", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					configServer := &v1.ResourceRequirements{
						Limits: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("300Mi"),
							v1.ResourceCPU:    resource.MustParse(".2"),
						},
						Requests: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("200Mi"),
							v1.ResourceCPU:    resource.MustParse(".1"),
						},
					}
					mongoOpsReq = f.MongoDBOpsRequestVerticalScale(mongodb.Name, mongodb.Namespace, nil, nil, nil, configServer, nil, nil)
				})

				It("Should Scale ConfigServer Resources", func() {
					shouldTestOpsRequest()

				})

			})
			Context("Scaling Shard Resources", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					shard := &v1.ResourceRequirements{
						Limits: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("300Mi"),
							v1.ResourceCPU:    resource.MustParse(".2"),
						},
						Requests: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("200Mi"),
							v1.ResourceCPU:    resource.MustParse(".1"),
						},
					}
					mongoOpsReq = f.MongoDBOpsRequestVerticalScale(mongodb.Name, mongodb.Namespace, nil, nil, nil, nil, shard, nil)
				})

				It("Should Scale Shard Resources", func() {
					shouldTestOpsRequest()

				})

			})
			Context("Scaling All Mongos,ConfigServer and Shard Resources", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					resource := &v1.ResourceRequirements{
						Limits: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("300Mi"),
							v1.ResourceCPU:    resource.MustParse(".2"),
						},
						Requests: map[v1.ResourceName]resource.Quantity{
							v1.ResourceMemory: resource.MustParse("200Mi"),
							v1.ResourceCPU:    resource.MustParse(".1"),
						},
					}
					mongoOpsReq = f.MongoDBOpsRequestVerticalScale(mongodb.Name, mongodb.Namespace, nil, nil, resource, resource, resource, nil)
				})

				It("Should Scale All Mongos,ConfigServer and Shard Resources", func() {
					shouldTestOpsRequest()

				})

			})
		})
	})

	Context("Volume Expansion", func() {
		BeforeEach(func() {
			if !f.IsGKE() {
				skipMessage = "volume expansion testing is only supported in GKE"
			}
		})

		Context("Standalone Instance", func() {
			BeforeEach(func() {
				mongodb = f.MongoDBStandalone()
				mongodb.Spec.Version = framework.MongoDBCatalogName
				mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				standalone := resource.MustParse("2Gi")
				mongoOpsReq = f.MongoDBOpsRequestVolumeExpansion(mongodb.Name, mongodb.Namespace, &standalone, nil, nil, nil)
			})

			It("Should Scale StandAlone Mongodb Resources", func() {
				shouldTestOpsRequest()
			})

		})
		Context("ReplicaSet Cluster", func() {
			BeforeEach(func() {
				mongodb = f.MongoDBRS()
				mongodb.Spec.Version = framework.MongoDBCatalogName
				mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				replicaset := resource.MustParse("2Gi")
				mongoOpsReq = f.MongoDBOpsRequestVolumeExpansion(mongodb.Name, mongodb.Namespace, nil, &replicaset, nil, nil)
			})

			It("Should Scale ReplicaSet Resources", func() {
				shouldTestOpsRequest()
			})

		})
		Context("Scaling ConfigServer Resources", func() {
			BeforeEach(func() {
				mongodb = f.MongoDBShard()
				mongodb.Spec.Version = framework.MongoDBCatalogName
				mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				configServer := resource.MustParse("2Gi")
				mongoOpsReq = f.MongoDBOpsRequestVolumeExpansion(mongodb.Name, mongodb.Namespace, nil, nil, nil, &configServer)
			})

			It("Should Scale ConfigServer Resources", func() {
				shouldTestOpsRequest()
			})

		})
		Context("Scaling Shard Resources", func() {
			BeforeEach(func() {
				mongodb = f.MongoDBShard()
				mongodb.Spec.Version = framework.MongoDBCatalogName
				mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				shard := resource.MustParse("2Gi")
				mongoOpsReq = f.MongoDBOpsRequestVolumeExpansion(mongodb.Name, mongodb.Namespace, nil, nil, &shard, nil)
			})

			It("Should Scale Shard Resources", func() {
				shouldTestOpsRequest()
			})

		})
	})

	Context("Reconfigure", func() {
		var prevMaxIncomingConnections = int32(10000)
		customConfigs := []string{
			fmt.Sprintf(`   maxIncomingConnections: %v`, prevMaxIncomingConnections),
		}

		var newMaxIncomingConnections = int32(20000)
		newCustomConfigs := []string{
			fmt.Sprintf(`   maxIncomingConnections: %v`, newMaxIncomingConnections),
		}
		data := map[string]string{
			"mongod.conf": fmt.Sprintf(`net:
   maxIncomingConnections: %v`, newMaxIncomingConnections),
		}

		Context("From Data", func() {
			var userConfig *v1.ConfigMap
			var newCustomConfig *dbaapi.MongoDBCustomConfig
			BeforeEach(func() {
				skipMessage = ""
				configName := f.App() + "-previous-config"
				userConfig = f.GetCustomConfig(customConfigs, configName)
				newCustomConfig = &dbaapi.MongoDBCustomConfig{
					Data: data,
				}
			})

			AfterEach(func() {
				By("Deleting configMap: " + userConfig.Name)
				err := f.DeleteConfigMap(userConfig.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())
			})

			runWithUserProvidedConfig := func() {
				if skipMessage != "" {
					Skip(skipMessage)
				}

				By("Creating configMap: " + userConfig.Name)
				err := f.CreateConfigMap(userConfig)
				Expect(err).NotTo(HaveOccurred())

				createMongodb(mongodb)

				By("Checking maxIncomingConnections from mongodb config")
				f.EventuallyMaxIncomingConnections(mongodb.ObjectMeta).Should(Equal(prevMaxIncomingConnections))

				// Insert Data
				By("Insert Document Inside DB")
				f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

				By("Checking Inserted Document")
				f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

				// Update Database
				By("Updating MongoDB")
				err = f.CreateMongoDBOpsRequest(mongoOpsReq)
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for MongoDB Ops Request Phase to be Successful")
				f.EventuallyMongoDBOpsRequestPhase(mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

				// Retrieve Inserted Data
				By("Checking Inserted Document after update")
				f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

				By("Checking updated maxIncomingConnections from mongodb config")
				f.EventuallyMaxIncomingConnections(mongodb.ObjectMeta).Should(Equal(newMaxIncomingConnections))
			}

			Context("Standalone MongoDB", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBStandalone()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					mongodb.Spec.ConfigSource = &v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}
					mongoOpsReq = f.MongoDBOpsRequestReconfigure(mongodb.Name, mongodb.Namespace, newCustomConfig, nil, nil, nil, nil)
				})

				It("should run successfully", runWithUserProvidedConfig)
			})

			Context("With Replica Set", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBRS()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					mongodb.Spec.ConfigSource = &v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}
					mongoOpsReq = f.MongoDBOpsRequestReconfigure(mongodb.Name, mongodb.Namespace, nil, newCustomConfig, nil, nil, nil)
				})

				It("should run successfully", runWithUserProvidedConfig)
			})

			Context("With Sharding", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					mongodb.Spec.ShardTopology.Shard.ConfigSource = &v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}
					mongodb.Spec.ShardTopology.ConfigServer.ConfigSource = &v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}
					mongodb.Spec.ShardTopology.Mongos.ConfigSource = &v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}

					mongoOpsReq = f.MongoDBOpsRequestReconfigure(mongodb.Name, mongodb.Namespace, nil, nil, newCustomConfig, newCustomConfig, newCustomConfig)

				})

				It("should run successfully", runWithUserProvidedConfig)
			})
		})

		Context("From New ConfigMap", func() {
			var userConfig *v1.ConfigMap
			var newUserConfig *v1.ConfigMap
			var newCustomConfig *dbaapi.MongoDBCustomConfig

			BeforeEach(func() {
				prevConfigName := f.App() + "-previous-config"
				newConfigName := f.App() + "-new-config"
				userConfig = f.GetCustomConfig(customConfigs, prevConfigName)
				newUserConfig = f.GetCustomConfig(newCustomConfigs, newConfigName)
				newCustomConfig = &dbaapi.MongoDBCustomConfig{
					ConfigMap: &v1.LocalObjectReference{
						Name: newUserConfig.Name,
					},
				}
			})

			AfterEach(func() {
				By("Deleting configMap: " + userConfig.Name)
				err := f.DeleteConfigMap(userConfig.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())
			})

			runWithUserProvidedConfig := func() {
				if skipMessage != "" {
					Skip(skipMessage)
				}

				By("Creating configMap: " + userConfig.Name)
				err := f.CreateConfigMap(userConfig)
				Expect(err).NotTo(HaveOccurred())

				By("Creating configMap: " + newUserConfig.Name)
				err = f.CreateConfigMap(newUserConfig)
				Expect(err).NotTo(HaveOccurred())

				createMongodb(mongodb)

				By("Checking maxIncomingConnections from mongodb config")
				f.EventuallyMaxIncomingConnections(mongodb.ObjectMeta).Should(Equal(prevMaxIncomingConnections))

				By("Insert Document Inside DB")
				f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

				By("Checking Inserted Document")
				f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

				By("Updating MongoDB")
				err = f.CreateMongoDBOpsRequest(mongoOpsReq)
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for MongoDB Ops Request Phase to be Successful")
				f.EventuallyMongoDBOpsRequestPhase(mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

				// Retrieve Inserted Data
				By("Checking Inserted Document after update")
				f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

				By("Checking updated maxIncomingConnections from mongodb config")
				f.EventuallyMaxIncomingConnections(mongodb.ObjectMeta).Should(Equal(newMaxIncomingConnections))
			}

			Context("Standalone MongoDB", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBStandalone()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					mongodb.Spec.ConfigSource = &v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}
					mongoOpsReq = f.MongoDBOpsRequestReconfigure(mongodb.Name, mongodb.Namespace, newCustomConfig, nil, nil, nil, nil)
				})

				It("should run successfully", runWithUserProvidedConfig)
			})

			Context("With Replica Set", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBRS()
					mongodb.Spec.ConfigSource = &v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}
					mongoOpsReq = f.MongoDBOpsRequestReconfigure(mongodb.Name, mongodb.Namespace, nil, newCustomConfig, nil, nil, nil)
				})

				It("should run successfully", runWithUserProvidedConfig)
			})

			Context("With Sharding", func() {
				BeforeEach(func() {
					mongodb = f.MongoDBShard()
					mongodb.Spec.Version = framework.MongoDBCatalogName
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					mongodb.Spec.ShardTopology.Shard.ConfigSource = &v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}
					mongodb.Spec.ShardTopology.ConfigServer.ConfigSource = &v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}
					mongodb.Spec.ShardTopology.Mongos.ConfigSource = &v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}

					mongoOpsReq = f.MongoDBOpsRequestReconfigure(mongodb.Name, mongodb.Namespace, nil, nil, newCustomConfig, newCustomConfig, newCustomConfig)
				})

				It("should run successfully", runWithUserProvidedConfig)
			})
		})
	})
})
