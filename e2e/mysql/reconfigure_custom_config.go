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

package mysql

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	opsapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"
	"kubedb.dev/tests/e2e/matcher"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
)

var _ = Describe("MySQL", func() {
	var fi *framework.Invocation

	var customConfigs = []string{
		"max_connections=200",
		"read_buffer_size=1048576", // 1MB
	}

	var reconfigureCustomConfigs = []string{
		"max_connections=400",
		"read_buffer_size=4702208", // 1MB
	}

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !RunTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !RunTestEnterprise(framework.Reconfigure) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Reconfigure))
		}
	})

	JustAfterEach(func() {
		fi.PrintDebugInfoOnFailure()
	})

	AfterEach(func() {
		err := fi.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())

	})

	FContext("Reconfigure Database", func() {
		Context("MySQL Standalone", func() {
			Context("Remove custom configuration", func() {
				It("Should remove custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("mysql")
					cm, err := fi.CustomConfigForMySQL(customConfigs, customConfigName)
					Expect(err).NotTo(HaveOccurred())

					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        "",
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking provided custom configuration have been configured")
					for _, cfg := range customConfigs {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Removing custom configuration and waiting for the success
					_ = fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MySQLCustomConfigurationSpec{
							RemoveCustomConfig: true,
						}
					})
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking mysql custom configuration have removed")
					for _, cfg := range customConfigs {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).ShouldNot(matcher.UseCustomConfig(cfg))
					}
				})
			})

			Context("Reconfigure custom configuration", func() {
				It("Should reconfigure custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("mysql")
					cm, err := fi.CustomConfigForMySQL(customConfigs, customConfigName)
					Expect(err).NotTo(HaveOccurred())

					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        "",
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking provided custom configuration have been configured")
					for _, cfg := range customConfigs {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Reconfigure custom configuration and waiting for the success
					reconfigureCustomConfigName := rand.WithUniqSuffix("reconfigure-mysql")
					rcm, err := fi.CustomConfigForMySQL(reconfigureCustomConfigs, reconfigureCustomConfigName)
					Expect(err).NotTo(HaveOccurred())
					_ = fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MySQLCustomConfigurationSpec{
							ConfigSecret: &core.LocalObjectReference{
								Name: rcm.Name,
							},
						}
					})
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking mysql custom configuration have been reconfigured")
					for _, cfg := range reconfigureCustomConfigs {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}
				})
			})

			Context("Inline Reconfigure custom configuration", func() {
				It("Should inline reconfigure custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("mysql")
					cm, err := fi.CustomConfigForMySQL(customConfigs, customConfigName)
					Expect(err).NotTo(HaveOccurred())

					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        "",
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking provided custom configuration have been configured")
					for _, cfg := range customConfigs {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Inline reconfigure custom configuration and waiting for the success
					Expect(err).NotTo(HaveOccurred())
					_ = fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MySQLCustomConfigurationSpec{
							InlineConfig: "max_connections=200\n" +
								"read_buffer_size=1048576",
						}
					})
					fi.EventuallyDBReady(my, dbInfo)

					inlineData := []string{
						"max_connections=200",
						"read_buffer_size=1048576", // 1MB
					}
					By("Checking mysql custom configuration have been reconfigured with inline configuration")
					for _, cfg := range inlineData {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}
				})
			})
		})

		Context("MySQL Group Replication", func() {
			Context("Remove custom configuration", func() {
				It("Should Remove custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("mysql")
					cm, err := fi.CustomConfigForMySQL(customConfigs, customConfigName)
					Expect(err).NotTo(HaveOccurred())
					// Create MySQL Group Replication and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
						clusterMode := api.MySQLClusterModeGroup
						in.Spec.Topology = &api.MySQLClusterTopology{
							Mode: &clusterMode,
							Group: &api.MySQLGroupSpec{
								Name:         "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
								BaseServerID: types.Int64P(api.MySQLDefaultBaseServerID),
							},
						}
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        "",
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking provided custom configuration have been configured")
					for _, cfg := range customConfigs {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Remove custom configuration and waiting for the success
					_ = fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MySQLCustomConfigurationSpec{
							RemoveCustomConfig: true,
						}
					})
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking mysql custom configuration have been removed")
					for _, cfg := range customConfigs {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).ShouldNot(matcher.UseCustomConfig(cfg))
					}
				})
			})

			Context("Reconfigure custom configuration", func() {
				It("Should reconfigure custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("mysql")
					cm, err := fi.CustomConfigForMySQL(customConfigs, customConfigName)
					Expect(err).NotTo(HaveOccurred())
					// Create MySQL Group Replication and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
						clusterMode := api.MySQLClusterModeGroup
						in.Spec.Topology = &api.MySQLClusterTopology{
							Mode: &clusterMode,
							Group: &api.MySQLGroupSpec{
								Name:         "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
								BaseServerID: types.Int64P(api.MySQLDefaultBaseServerID),
							},
						}
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        "",
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking provided custom configuration have been configured")
					for _, cfg := range customConfigs {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Reconfigure custom configuration and waiting for the success
					reconfigureCustomConfigName := rand.WithUniqSuffix("reconfigure-mysql")
					rcm, err := fi.CustomConfigForMySQL(reconfigureCustomConfigs, reconfigureCustomConfigName)
					_ = fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MySQLCustomConfigurationSpec{
							ConfigSecret: &core.LocalObjectReference{
								Name: rcm.Name,
							},
						}
					})
					Expect(err).NotTo(HaveOccurred())
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking mysql custom configuration have been reconfigured")
					for _, cfg := range reconfigureCustomConfigs {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}
				})
			})

			Context("Inline Reconfigure custom configuration", func() {
				It("Should inline Reconfigure custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("mysql")
					cm, err := fi.CustomConfigForMySQL(customConfigs, customConfigName)
					Expect(err).NotTo(HaveOccurred())
					// Create MySQL Group Replication and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
						clusterMode := api.MySQLClusterModeGroup
						in.Spec.Topology = &api.MySQLClusterTopology{
							Mode: &clusterMode,
							Group: &api.MySQLGroupSpec{
								Name:         "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
								BaseServerID: types.Int64P(api.MySQLDefaultBaseServerID),
							},
						}
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        "",
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking provided custom configuration have been configured")
					for _, cfg := range customConfigs {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Inline reconfigure custom configuration and waiting for the success
					_ = fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MySQLCustomConfigurationSpec{
							InlineConfig: "max_connections=200\n" +
								"read_buffer_size=1048576",
						}
					})
					fi.EventuallyDBReady(my, dbInfo)

					inlineData := []string{
						"max_connections=200",
						"read_buffer_size=1048576", // 1MB
					}
					By("Checking mysql custom configuration have been reconfigured with inline configuration")
					for _, cfg := range inlineData {
						fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}
				})
			})
		})
	})
})
