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
	"context"
	"fmt"
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	opsapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
)

var _ = Describe("MySQL", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !RunTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !RunTestEnterprise(framework.Upgrade) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Upgrade))
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

	Context("Upgrade Database Version", func() {
		Context("MySQL Standalone", func() {
			It("Should Upgrade MySQL Standalone", func() {
				// MySQL objectMeta
				issuerMeta := metav1.ObjectMeta{
					Name:      rand.WithUniqSuffix("mysql"),
					Namespace: fi.Namespace(),
				}
				issuer, err := fi.EnsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
				Expect(err).NotTo(HaveOccurred())
				// Create MySQL standalone with tls secured and wait for running
				my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, framework.AddTLSConfig(issuer.ObjectMeta))
				Expect(err).NotTo(HaveOccurred())
				// Database connection information
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

				// Upgrade MySQL Version and waiting for success
				myOR := fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeUpgrade
					in.Spec.Upgrade = &opsapi.MySQLUpgradeSpec{
						TargetVersion: framework.DBUpdatedVersion,
					}
				})

				By("Checking MySQL version upgraded")
				targetedVersion, err := fi.DBClient().CatalogV1alpha1().MySQLVersions().Get(context.TODO(), myOR.Spec.Upgrade.TargetVersion, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				fi.EventuallyDatabaseVersionUpdated(my.ObjectMeta, dbInfo, targetedVersion.Spec.Version).Should(BeTrue())

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
		Context("MySQL Group Replication", func() {
			Context("Upgrade Major/Patch Version", func() {
				It("Should Upgrade Major/Patch Version", func() {
					issuerMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.EnsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
					Expect(err).NotTo(HaveOccurred())
					// Create MySQL Group Replication with tls secured and wait for running
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
					// Database connection information
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

					// Upgrade MySQL Version and waiting for success
					myOR := fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeUpgrade
						in.Spec.Upgrade = &opsapi.MySQLUpgradeSpec{
							TargetVersion: framework.DBUpdatedVersion,
						}
					})

					By("Checking MySQL version upgraded")
					targetedVersion, err := fi.DBClient().CatalogV1alpha1().MySQLVersions().Get(context.TODO(), myOR.Spec.Upgrade.TargetVersion, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					fi.EventuallyDatabaseVersionUpdated(my.ObjectMeta, dbInfo, targetedVersion.Spec.Version).Should(BeTrue())

					// Retrieve Inserted Data
					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
				})
			})
		})
	})
})
