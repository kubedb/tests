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
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("Reconfigure Redis", func() {
	to := testOptions{}
	testName := framework.RedisReconfigure
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
		to.EventuallyWipedOut(to.redis.ObjectMeta).Should(Succeed())
	})

	Context("Reconfigure From Data", func() {
		var userConfig *core.Secret
		var newConfigSpec *dbaapi.RedisCustomConfigurationSpec
		var configSecret *core.LocalObjectReference
		BeforeEach(func() {
			to.skipMessage = ""
			prevConfigName := to.App() + "-previous-config"
			userConfig = to.GetCustomConfigRedis(customConfigs, prevConfigName)
			newConfigSpec = &dbaapi.RedisCustomConfigurationSpec{
				InlineConfig: inlineConfig,
			}
			configSecret = &core.LocalObjectReference{
				Name: userConfig.Name,
			}
		})
		AfterEach(func() {
			By("Deleting secret: " + userConfig.Name)
			err := to.DeleteSecret(userConfig.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("Standalone Redis", func() {
			BeforeEach(func() {
				to.redis = to.RedisStandalone(framework.DBVersion)
				to.redis.Spec.ConfigSecret = configSecret
				to.redisOpsReq = to.RedisOpsRequestReconfiguration(to.redis.Name, to.redis.Namespace, newConfigSpec)
			})

			It("Should configure with inline data", func() {
				to.shouldTestConfigurationOpsReq(userConfig, nil, customConfigs, newCustomConfig)
			})
		})

		Context("Cluster Redis", func() {
			BeforeEach(func() {
				to.redis = to.RedisCluster(framework.DBVersion, nil, nil)
				to.redis.Spec.ConfigSecret = configSecret
				to.redisOpsReq = to.RedisOpsRequestReconfiguration(to.redis.Name, to.redis.Namespace, newConfigSpec)
			})
			AfterEach(func() {
				_, err := to.Invocation.TestConfig().FlushDBForCluster(to.redis)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Should configure with inline data", func() {
				to.shouldTestConfigurationOpsReq(userConfig, nil, customConfigs, newCustomConfig)
			})
		})
	})

	Context("Reconfigure From Secret", func() {
		var userConfig *core.Secret
		var newUserConfig *core.Secret
		var newConfigSpec *dbaapi.RedisCustomConfigurationSpec
		var configSecret *core.LocalObjectReference
		BeforeEach(func() {
			to.skipMessage = ""
			prevConfigName := to.App() + "-previous-config"
			newConfigName := to.App() + "-new-config"
			userConfig = to.GetCustomConfigRedis(customConfigs, prevConfigName)
			newUserConfig = to.GetCustomConfigRedis(newCustomConfig, newConfigName)
			newConfigSpec = &dbaapi.RedisCustomConfigurationSpec{
				ConfigSecret: &core.LocalObjectReference{
					Name: newUserConfig.Name,
				},
			}
			configSecret = &core.LocalObjectReference{
				Name: userConfig.Name,
			}
		})
		AfterEach(func() {
			By("Deleting secret: " + userConfig.Name)
			err := to.DeleteSecret(userConfig.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
			By("Deleting secret: " + newUserConfig.Name)
			err = to.DeleteSecret(newUserConfig.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("Standalone Redis", func() {
			BeforeEach(func() {
				to.redis = to.RedisStandalone(framework.DBVersion)
				to.redis.Spec.ConfigSecret = configSecret
				to.redisOpsReq = to.RedisOpsRequestReconfiguration(to.redis.Name, to.redis.Namespace, newConfigSpec)
			})

			It("Should configure with new secret", func() {
				to.shouldTestConfigurationOpsReq(userConfig, newUserConfig, customConfigs, newCustomConfig)
			})
		})

		Context("Cluster Redis", func() {
			BeforeEach(func() {
				to.redis = to.RedisCluster(framework.DBVersion, nil, nil)
				to.redis.Spec.ConfigSecret = configSecret
				to.redisOpsReq = to.RedisOpsRequestReconfiguration(to.redis.Name, to.redis.Namespace, newConfigSpec)
			})
			AfterEach(func() {
				_, err := to.Invocation.TestConfig().FlushDBForCluster(to.redis)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Should configure with new secret", func() {
				to.shouldTestConfigurationOpsReq(userConfig, newUserConfig, customConfigs, newCustomConfig)
			})
		})
	})

	Context("Removing Config", func() {
		var userConfig *core.Secret
		var newConfigSpec *dbaapi.RedisCustomConfigurationSpec
		var configSecret *core.LocalObjectReference
		var removedConfig = []string{
			"maxclients 10000",
		}
		BeforeEach(func() {
			to.skipMessage = ""
			prevConfigName := to.App() + "-previous-config"
			userConfig = to.GetCustomConfigRedis(customConfigs, prevConfigName)
			newConfigSpec = &dbaapi.RedisCustomConfigurationSpec{
				RemoveCustomConfig: true,
			}
			configSecret = &core.LocalObjectReference{
				Name: userConfig.Name,
			}
		})
		AfterEach(func() {
			By("Deleting secret: " + userConfig.Name)
			err := to.DeleteSecret(userConfig.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("Standalone Redis", func() {
			BeforeEach(func() {
				to.redis = to.RedisStandalone(framework.DBVersion)
				to.redis.Spec.ConfigSecret = configSecret
				to.redisOpsReq = to.RedisOpsRequestReconfiguration(to.redis.Name, to.redis.Namespace, newConfigSpec)
			})

			It("Should configure by removing all data", func() {
				to.shouldTestConfigurationOpsReq(userConfig, nil, customConfigs, removedConfig)
			})
		})

		Context("Cluster Redis", func() {
			BeforeEach(func() {
				to.redis = to.RedisCluster(framework.DBVersion, nil, nil)
				to.redis.Spec.ConfigSecret = configSecret
				to.redisOpsReq = to.RedisOpsRequestReconfiguration(to.redis.Name, to.redis.Namespace, newConfigSpec)
			})
			AfterEach(func() {
				_, err := to.Invocation.TestConfig().FlushDBForCluster(to.redis)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Should configure by removing all data", func() {
				to.shouldTestConfigurationOpsReq(userConfig, nil, customConfigs, removedConfig)
			})
		})
	})
})
