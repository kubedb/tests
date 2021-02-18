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
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	_ "github.com/go-sql-driver/mysql"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("MySQL TLS/SSL", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !RunTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !RunTestEnterprise(framework.Enterprise) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Enterprise))
		}
		if !framework.SSLEnabled {
			Skip("Enable SSL to test this")
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
		Context("Exporter", func() {
			Context("Standalone", func() {
				It("Should verify Exporter", func() {
					issuerMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.InsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
					Expect(err).NotTo(HaveOccurred())

					// Create MySQL standalone with SSL secured and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						// Add Monitor
						fi.AddMySQLMonitor(in)
					}, framework.AddTLSConfig(issuer.ObjectMeta))

					Expect(err).NotTo(HaveOccurred())
					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Verify exporter")
					err = fi.VerifyMySQLExporter(my)
					Expect(err).NotTo(HaveOccurred())
					By("Done")
				})
			})

			Context("Group Replication", func() {
				It("Should verify Exporter", func() {
					issuerMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.InsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
					Expect(err).NotTo(HaveOccurred())

					// Create MySQL standalone with SSL secured and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
						clusterMode := api.MySQLClusterModeGroup
						in.Spec.Topology = &api.MySQLClusterTopology{
							Mode: &clusterMode,
							Group: &api.MySQLGroupSpec{
								Name: "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
							},
						}
						fi.AddMySQLMonitor(in)
					}, framework.AddTLSConfig(issuer.ObjectMeta))
					Expect(err).NotTo(HaveOccurred())

					dbInfo := framework.DatabaseConnectionInfo{
						DatabaseName: framework.DBMySQL,
						User:         framework.MySQLRootUser,
						Param:        fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Verify exporter")
					err = fi.VerifyMySQLExporter(my)
					Expect(err).NotTo(HaveOccurred())
					By("Done")
				})
			})
		})

		Context("General", func() {
			Context("with requireSSL true", func() {
				Context("Standalone", func() {
					It("should run successfully", func() {
						issuerMeta := metav1.ObjectMeta{
							Name:      rand.WithUniqSuffix("mysql"),
							Namespace: fi.Namespace(),
						}
						issuer, err := fi.InsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
						Expect(err).NotTo(HaveOccurred())
						// Create MySQL standalone with SSL secured and wait for running
						my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, framework.AddTLSConfig(issuer.ObjectMeta))
						Expect(err).NotTo(HaveOccurred())
						dbInfo := framework.DatabaseConnectionInfo{
							DatabaseName: framework.DBMySQL,
							User:         framework.MySQLRootUser,
							Param:        fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
						}
						fi.EventuallyDBReady(my, dbInfo)

						// Create a mysql User with required SSL
						By("Create mysql User with required SSL")
						fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, dbInfo).Should(BeTrue())
						dbInfo.User = framework.MySQLRequiredSSLUser
						fi.EventuallyCheckConnectionRequiredSSLUser(my, dbInfo)
						fi.PopulateMySQL(my.ObjectMeta, dbInfo)
					})
				})

				Context("Group Replication", func() {
					It("should run successfully", func() {
						issuerMeta := metav1.ObjectMeta{
							Name:      rand.WithUniqSuffix("mysql"),
							Namespace: fi.Namespace(),
						}
						issuer, err := fi.InsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
						Expect(err).NotTo(HaveOccurred())

						// Create MySQL standalone with SSL secured and wait for running
						my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
							in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
							clusterMode := api.MySQLClusterModeGroup
							in.Spec.Topology = &api.MySQLClusterTopology{
								Mode: &clusterMode,
								Group: &api.MySQLGroupSpec{
									Name: "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
								},
							}
						}, framework.AddTLSConfig(issuer.ObjectMeta))
						Expect(err).NotTo(HaveOccurred())
						dbInfo := framework.DatabaseConnectionInfo{
							DatabaseName: framework.DBMySQL,
							User:         framework.MySQLRootUser,
							Param:        fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
						}
						fi.EventuallyDBReady(my, dbInfo)

						// Create a mysql User with required SSL
						By("Create mysql User with required SSL")
						fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, dbInfo).Should(BeTrue())
						dbInfo.User = framework.MySQLRequiredSSLUser
						fi.EventuallyCheckConnectionRequiredSSLUser(my, dbInfo)
						fi.PopulateMySQL(my.ObjectMeta, dbInfo)
					})
				})
			})

			Context("with requireSSL false", func() {
				Context("Standalone", func() {
					It("should run successfully", func() {
						issuerMeta := metav1.ObjectMeta{
							Name:      rand.WithUniqSuffix("mysql"),
							Namespace: fi.Namespace(),
						}
						issuer, err := fi.InsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
						Expect(err).NotTo(HaveOccurred())

						// Create MySQL standalone with SSL secured and wait for running
						my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, framework.AddTLSConfig(issuer.ObjectMeta))
						Expect(err).NotTo(HaveOccurred())
						dbInfo := framework.DatabaseConnectionInfo{
							DatabaseName: framework.DBMySQL,
							User:         framework.MySQLRootUser,
							Param:        fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
						}
						fi.EventuallyDBReady(my, dbInfo)

						// Create a mysql User with required SSL
						By("Create mysql User with required SSL")
						fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, dbInfo).Should(BeTrue())
						dbInfo.User = framework.MySQLRequiredSSLUser
						fi.EventuallyCheckConnectionRequiredSSLUser(my, dbInfo)
						fi.PopulateMySQL(my.ObjectMeta, dbInfo)
					})
				})

				Context("Group Replication", func() {
					It("should run successfully", func() {
						issuerMeta := metav1.ObjectMeta{
							Name:      rand.WithUniqSuffix("mysql"),
							Namespace: fi.Namespace(),
						}
						issuer, err := fi.InsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
						Expect(err).NotTo(HaveOccurred())
						// Create MySQL standalone with SSL secured and wait for running
						my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
							in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
							clusterMode := api.MySQLClusterModeGroup
							in.Spec.Topology = &api.MySQLClusterTopology{
								Mode: &clusterMode,
								Group: &api.MySQLGroupSpec{
									Name: "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
								},
							}
						}, framework.AddTLSConfig(issuer.ObjectMeta))
						Expect(err).NotTo(HaveOccurred())

						dbInfo := framework.DatabaseConnectionInfo{
							DatabaseName: framework.DBMySQL,
							User:         framework.MySQLRootUser,
							Param:        fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
						}
						fi.EventuallyDBReady(my, dbInfo)

						// Create a mysql User with required SSL
						By("Create mysql User with required SSL")
						fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, dbInfo).Should(BeTrue())
						dbInfo.User = framework.MySQLRequiredSSLUser
						fi.EventuallyCheckConnectionRequiredSSLUser(my, dbInfo)
						fi.PopulateMySQL(my.ObjectMeta, dbInfo)
					})
				})
			})
		})
	})
})
