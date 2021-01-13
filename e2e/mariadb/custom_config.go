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
	"kubedb.dev/tests/e2e/framework"
	"kubedb.dev/tests/e2e/matcher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
)

var _ = Describe("MariaDB", func() {

	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMariaDB))
		}
		if !runTestCommunity(framework.CustomConfig) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.CustomConfig))
		}
	})

	JustAfterEach(func() {
		fi.PrintDebugInfoOnFailure()
	})

	AfterEach(func() {
		err := fi.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())

	})

	Describe("Test", func() {

		Context("Custom config", func() {

			var customConfigs = []string{
				"max_connections=200",
				"read_buffer_size=1048576", // 1MB
			}
			var customConfigForMariaDB = func() (*core.Secret, error) {
				cm := fi.GetCustomConfigForMariaDB(customConfigs)
				By("Creating custom Config for MariaDB " + cm.Namespace + "/" + cm.Name)
				cm, err := fi.CreateSecret(cm)
				if err != nil {
					return nil, err
				}
				fi.AppendToCleanupList(cm)
				return cm, err
			}

			Context("from secret", func() {
				It("should set configuration provided in secret", func() {
					cm, err := customConfigForMariaDB()
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
					dbInfo := framework.DatabaseConnectionInfo{
						StatefulSetOrdinal: 0,
						ClientPodIndex:     0,
						DatabaseName:       framework.DBMySQL,
						User:               framework.MySQLRootUser,
						Param:              "",
					}
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking mariadb configured from provided custom configuration")
					for _, cfg := range customConfigs {
						fi.EventuallyMariaDBVariable(md.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))
					}
				})
			})
		})

	})
})
