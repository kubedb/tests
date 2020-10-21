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
)

var _ = Describe("Reconfigure", func() {
	to := testOptions{}
	testName := framework.Reconfigure
	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !runTestEnterprise(testName) {
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
		to.EventuallyWipedOut(to.mongodb.ObjectMeta).Should(Succeed())
	})

	Context("From Data", func() {
		var userConfig *v1.Secret
		var newCustomConfig *dbaapi.MongoDBCustomConfiguration
		var configSecret *v1.LocalObjectReference
		BeforeEach(func() {
			to.skipMessage = ""
			configName := to.App() + "-previous-config"
			userConfig = to.GetCustomConfig(customConfigs, configName)
			newCustomConfig = &dbaapi.MongoDBCustomConfiguration{
				InlineConfig: inlineConfig,
			}
			configSecret = &v1.LocalObjectReference{
				Name: userConfig.Name,
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
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ConfigSecret = configSecret
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, newCustomConfig, nil, nil, nil, nil)
			})

			It("should run successfully", func() {
				to.runWithUserProvidedConfig(userConfig, nil, false)
			})
		})

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ConfigSecret = configSecret
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, nil, newCustomConfig, nil, nil, nil)
			})

			It("should run successfully", func() {
				to.runWithUserProvidedConfig(userConfig, nil, false)
			})
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ShardTopology.Shard.ConfigSecret = configSecret
				to.mongodb.Spec.ShardTopology.ConfigServer.ConfigSecret = configSecret
				to.mongodb.Spec.ShardTopology.Mongos.ConfigSecret = configSecret
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, nil, nil, newCustomConfig, newCustomConfig, newCustomConfig)

			})

			It("should run successfully", func() {
				to.runWithUserProvidedConfig(userConfig, nil, false)
			})
		})
	})

	Context("From New ConfigMap", func() {
		var userConfig *v1.Secret
		var newUserConfig *v1.Secret
		var newCustomConfig *dbaapi.MongoDBCustomConfiguration
		var configSecret *v1.LocalObjectReference

		BeforeEach(func() {
			prevConfigName := to.App() + "-previous-config"
			newConfigName := to.App() + "-new-config"
			userConfig = to.GetCustomConfig(customConfigs, prevConfigName)
			newUserConfig = to.GetCustomConfig(newCustomConfigs, newConfigName)
			newCustomConfig = &dbaapi.MongoDBCustomConfiguration{
				ConfigSecret: &v1.LocalObjectReference{
					Name: newUserConfig.Name,
				},
			}
			configSecret = &v1.LocalObjectReference{
				Name: userConfig.Name,
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
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ConfigSecret = configSecret
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, newCustomConfig, nil, nil, nil, nil)
			})

			It("should run successfully", func() {
				to.runWithUserProvidedConfig(userConfig, newUserConfig, false)
			})
		})

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.ConfigSecret = configSecret
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, nil, newCustomConfig, nil, nil, nil)
			})

			It("should run successfully", func() {
				to.runWithUserProvidedConfig(userConfig, newUserConfig, false)
			})
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ShardTopology.Shard.ConfigSecret = configSecret
				to.mongodb.Spec.ShardTopology.ConfigServer.ConfigSecret = configSecret
				to.mongodb.Spec.ShardTopology.Mongos.ConfigSecret = configSecret
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, nil, nil, newCustomConfig, newCustomConfig, newCustomConfig)
			})

			It("should run successfully", func() {
				to.runWithUserProvidedConfig(userConfig, newUserConfig, false)
			})
		})
	})

	FContext("Remove Config", func() {
		var userConfig *v1.Secret
		var newCustomConfig *dbaapi.MongoDBCustomConfiguration
		var configSecret *v1.LocalObjectReference
		BeforeEach(func() {
			to.skipMessage = ""
			configName := to.App() + "-previous-config"
			userConfig = to.GetCustomConfig(customConfigs, configName)
			newCustomConfig = &dbaapi.MongoDBCustomConfiguration{
				RemoveCustomConfig: true,
			}
			configSecret = &v1.LocalObjectReference{
				Name: userConfig.Name,
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
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ConfigSecret = configSecret
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, newCustomConfig, nil, nil, nil, nil)
			})

			It("should run successfully", func() {
				to.runWithUserProvidedConfig(userConfig, nil, true)
			})
		})

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ConfigSecret = configSecret
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, nil, newCustomConfig, nil, nil, nil)
			})

			It("should run successfully", func() {
				to.runWithUserProvidedConfig(userConfig, nil, true)
			})
		})

		FContext("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ShardTopology.Shard.ConfigSecret = configSecret
				to.mongodb.Spec.ShardTopology.ConfigServer.ConfigSecret = configSecret
				to.mongodb.Spec.ShardTopology.Mongos.ConfigSecret = configSecret
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, nil, nil, newCustomConfig, newCustomConfig, newCustomConfig)

			})

			It("should run successfully", func() {
				to.runWithUserProvidedConfig(userConfig, nil, true)
			})
		})
	})
})
