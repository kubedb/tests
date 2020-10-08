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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Vertical Scaling", func() {
	to := testOptions{}
	testName := framework.VerticalScaling
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

	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !runTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", testName))
		}
	})

	AfterEach(func() {
		err := to.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())
		By("Delete mongodb")
		err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Delete mongodb ops request")
		err = to.DeleteMongoDBOpsRequest(to.mongoOpsReq.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb resources to be wipedOut")
		to.EventuallyWipedOut(to.mongodb.ObjectMeta).Should(Succeed())
	})
	Context("Without Custom Config", func() {
		Context("Scaling StandAlone Mongodb Resources", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBStandalone()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, resource, nil, nil, nil, nil, nil)
			})

			It("Should Scale StandAlone Mongodb Resources", func() {
				to.shouldTestOpsRequest()
			})

		})
		Context("Scaling ReplicaSet Resources", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, resource, nil, nil, nil, nil)
			})

			It("Should Scale ReplicaSet Resources", func() {
				to.shouldTestOpsRequest()

			})

		})
		Context("Scaling Mongos Resources", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, resource, nil, nil, nil)
			})

			It("Should Scale Mongos Resources", func() {
				to.shouldTestOpsRequest()
			})

		})
		Context("Scaling ConfigServer Resources", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, nil, resource, nil, nil)
			})

			It("Should Scale ConfigServer Resources", func() {
				to.shouldTestOpsRequest()
			})

		})
		Context("Scaling Shard Resources", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, nil, nil, resource, nil)
			})

			It("Should Scale Shard Resources", func() {
				to.shouldTestOpsRequest()
			})

		})
		Context("Scaling All Mongos,ConfigServer and Shard Resources", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, resource, resource, resource, nil)
			})

			It("Should Scale All Mongos,ConfigServer and Shard Resources", func() {
				to.shouldTestOpsRequest()
			})

		})
	})

	Context("With Custom Config", func() {
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
			var newCustomConfig *dbaapi.MongoDBCustomConfiguration
			var configSource *v1.VolumeSource
			BeforeEach(func() {
				to.skipMessage = ""
				configName := to.App() + "-previous-config"
				userConfig = to.GetCustomConfig(customConfigs, configName)
				newCustomConfig = &dbaapi.MongoDBCustomConfiguration{
					Data: data,
				}
				configSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
			})

			AfterEach(func() {
				By("Deleting configMap: " + userConfig.Name)
				err := to.DeleteConfigMap(userConfig.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("Standalone MongoDB", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBStandalone()
					to.mongodb.Spec.ConfigSource = configSource
					to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, resource, nil, nil, nil, nil, nil)
					to.mongoOpsReq.Spec.Configuration = &dbaapi.MongoDBCustomConfigurationSpec{
						Standalone: newCustomConfig,
					}
				})

				It("should run successfully", func() {
					to.runWithUserProvidedConfig(userConfig, nil)
				})
			})

			Context("With ReplicaSet", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBRS()
					to.mongodb.Spec.ConfigSource = configSource
					to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, resource, nil, nil, nil, nil)
					to.mongoOpsReq.Spec.Configuration = &dbaapi.MongoDBCustomConfigurationSpec{
						ReplicaSet: newCustomConfig,
					}
				})

				It("should run successfully", func() {
					to.runWithUserProvidedConfig(userConfig, nil)
				})
			})

			Context("With Sharding", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBShard()
					to.mongodb.Spec.ShardTopology.Shard.ConfigSource = configSource
					to.mongodb.Spec.ShardTopology.ConfigServer.ConfigSource = configSource
					to.mongodb.Spec.ShardTopology.Mongos.ConfigSource = configSource
					to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, resource, resource, resource, nil)
					to.mongoOpsReq.Spec.Configuration = &dbaapi.MongoDBCustomConfigurationSpec{
						Mongos:       newCustomConfig,
						ConfigServer: newCustomConfig,
						Shard:        newCustomConfig,
					}
				})

				It("should run successfully", func() {
					to.runWithUserProvidedConfig(userConfig, nil)
				})
			})
		})

		Context("From New ConfigMap", func() {
			var userConfig *v1.ConfigMap
			var newUserConfig *v1.ConfigMap
			var newCustomConfig *dbaapi.MongoDBCustomConfiguration
			var configSource *v1.VolumeSource

			BeforeEach(func() {
				prevConfigName := to.App() + "-previous-config"
				newConfigName := to.App() + "-new-config"
				userConfig = to.GetCustomConfig(customConfigs, prevConfigName)
				newUserConfig = to.GetCustomConfig(newCustomConfigs, newConfigName)
				newCustomConfig = &dbaapi.MongoDBCustomConfiguration{
					ConfigMap: &v1.LocalObjectReference{
						Name: newUserConfig.Name,
					},
				}
				configSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
			})

			AfterEach(func() {
				By("Deleting configMap: " + userConfig.Name)
				err := to.DeleteConfigMap(userConfig.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("Standalone MongoDB", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBStandalone()
					to.mongodb.Spec.ConfigSource = configSource
					to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, resource, nil, nil, nil, nil, nil)
					to.mongoOpsReq.Spec.Configuration = &dbaapi.MongoDBCustomConfigurationSpec{
						Standalone: newCustomConfig,
					}
				})

				It("should run successfully", func() {
					to.runWithUserProvidedConfig(userConfig, newUserConfig)
				})
			})

			Context("With Replica Set", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBRS()
					to.mongodb.Spec.ConfigSource = configSource
					to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, resource, nil, nil, nil, nil)
					to.mongoOpsReq.Spec.Configuration = &dbaapi.MongoDBCustomConfigurationSpec{
						ReplicaSet: newCustomConfig,
					}
				})

				It("should run successfully", func() {
					to.runWithUserProvidedConfig(userConfig, newUserConfig)
				})
			})

			Context("With Sharding", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBShard()
					to.mongodb.Spec.ShardTopology.Shard.ConfigSource = configSource
					to.mongodb.Spec.ShardTopology.ConfigServer.ConfigSource = configSource
					to.mongodb.Spec.ShardTopology.Mongos.ConfigSource = configSource
					to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, resource, resource, resource, nil)
					to.mongoOpsReq.Spec.Configuration = &dbaapi.MongoDBCustomConfigurationSpec{
						Mongos:       newCustomConfig,
						ConfigServer: newCustomConfig,
						Shard:        newCustomConfig,
					}
				})

				It("should run successfully", func() {
					to.runWithUserProvidedConfig(userConfig, newUserConfig)
				})
			})
		})
	})
})
