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

package mariadb

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

var _ = Describe("MariaDB", func() {
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

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !runTestEnterprise(framework.Reconfigure) {
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

	Context("Reconfigure Database", func() {
		Context("MariaDB Standalone", func() {
			Context("Remove custom configuration", func() {
				It("Should remove custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("md-custom-config")
					cm, err := fi.CustomConfigForMariaDB(customConfigs, customConfigName)
					By("Creating custom Config for MariaDB " + cm.Namespace + "/" + cm.Name)
					Expect(err).NotTo(HaveOccurred())

					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking mariadb configured from provided custom configuration")
					for _, cfg := range customConfigs {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Removing custom configuration and waiting for the success
					_ = fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MariaDBCustomConfigurationSpec{
							RemoveCustomConfig: true,
						}
					})
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking mysql custom configuration have removed")
					for _, cfg := range customConfigs {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).ShouldNot(matcher.UseCustomConfig(cfg))
					}
				})
			})

			Context("Reconfigure custom configuration", func() {
				It("Should reconfigure custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("md-custom-config")
					cm, err := fi.CustomConfigForMariaDB(customConfigs, customConfigName)
					By("Creating custom Config for MariaDB " + cm.Namespace + "/" + cm.Name)
					Expect(err).NotTo(HaveOccurred())

					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking mariadb configured from provided custom configuration")
					for _, cfg := range customConfigs {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Reconfigure custom configuration and waiting for the success
					reconfigureCustomConfigName := rand.WithUniqSuffix("reconfigure-md")
					rcm, err := fi.CustomConfigForMariaDB(reconfigureCustomConfigs, reconfigureCustomConfigName)
					Expect(err).NotTo(HaveOccurred())
					_ = fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MariaDBCustomConfigurationSpec{
							ConfigSecret: &core.LocalObjectReference{
								Name: rcm.Name,
							},
						}
					})
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking mariadb custom configuration have been reconfigured")
					for _, cfg := range reconfigureCustomConfigs {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}
				})
			})

			Context("Inline Reconfigure custom configuration", func() {
				It("Should inline reconfigure custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("md-custom-config")
					cm, err := fi.CustomConfigForMariaDB(customConfigs, customConfigName)
					By("Creating custom Config for MariaDB " + cm.Namespace + "/" + cm.Name)
					Expect(err).NotTo(HaveOccurred())

					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking mariadb configured from provided custom configuration")
					for _, cfg := range customConfigs {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Inline reconfigure custom configuration and waiting for the success

					_ = fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MariaDBCustomConfigurationSpec{
							InlineConfig: "max_connections=200\n" +
								"read_buffer_size=1048576",
						}
					})
					fi.EventuallyDBReadyMD(md, dbInfo)

					inlineData := []string{
						"max_connections=200",
						"read_buffer_size=1048576", // 1MB
					}
					By("Checking mysql custom configuration have been reconfigured with inline configuration")
					for _, cfg := range inlineData {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}
				})
			})
		})

		Context("MariaDB Cluster", func() {
			Context("Remove custom configuration", func() {
				It("Should remove custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("md-custom-config")
					cm, err := fi.CustomConfigForMariaDB(customConfigs, customConfigName)
					By("Creating custom Config for MariaDB " + cm.Namespace + "/" + cm.Name)
					Expect(err).NotTo(HaveOccurred())

					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Spec.Replicas = types.Int32P(3)
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking mariadb configured from provided custom configuration")
					for _, cfg := range customConfigs {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Removing custom configuration and waiting for the success
					_ = fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MariaDBCustomConfigurationSpec{
							RemoveCustomConfig: true,
						}
					})
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking mysql custom configuration have removed")
					for _, cfg := range customConfigs {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).ShouldNot(matcher.UseCustomConfig(cfg))
					}
				})
			})

			Context("Reconfigure custom configuration", func() {
				It("Should reconfigure custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("md-custom-config")
					cm, err := fi.CustomConfigForMariaDB(customConfigs, customConfigName)
					By("Creating custom Config for MariaDB " + cm.Namespace + "/" + cm.Name)
					Expect(err).NotTo(HaveOccurred())

					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Spec.Replicas = types.Int32P(3)
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking mariadb configured from provided custom configuration")
					for _, cfg := range customConfigs {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Reconfigure custom configuration and waiting for the success
					reconfigureCustomConfigName := rand.WithUniqSuffix("reconfigure-md")
					rcm, err := fi.CustomConfigForMariaDB(reconfigureCustomConfigs, reconfigureCustomConfigName)
					Expect(err).NotTo(HaveOccurred())
					_ = fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MariaDBCustomConfigurationSpec{
							ConfigSecret: &core.LocalObjectReference{
								Name: rcm.Name,
							},
						}
					})
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking mariadb custom configuration have been reconfigured")
					for _, cfg := range reconfigureCustomConfigs {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}
				})
			})

			Context("Inline Reconfigure custom configuration", func() {
				It("Should inline reconfigure custom configuration", func() {
					customConfigName := rand.WithUniqSuffix("md-custom-config")
					cm, err := fi.CustomConfigForMariaDB(customConfigs, customConfigName)
					By("Creating custom Config for MariaDB " + cm.Namespace + "/" + cm.Name)
					Expect(err).NotTo(HaveOccurred())

					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Spec.Replicas = types.Int32P(3)
						in.Spec.ConfigSecret = &core.LocalObjectReference{
							Name: cm.Name,
						}
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking mariadb configured from provided custom configuration")
					for _, cfg := range customConfigs {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}

					// Inline reconfigure custom configuration and waiting for the success

					_ = fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigure
						in.Spec.Configuration = &opsapi.MariaDBCustomConfigurationSpec{
							InlineConfig: "max_connections=200\n" +
								"read_buffer_size=1048576",
						}
					})
					fi.EventuallyDBReadyMD(md, dbInfo)

					inlineData := []string{
						"max_connections=200",
						"read_buffer_size=1048576", // 1MB
					}
					By("Checking mysql custom configuration have been reconfigured with inline configuration")
					for _, cfg := range inlineData {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}
				})
			})
		})
	})
})
