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

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	opsapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"
	"kubedb.dev/tests/e2e/matcher"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("MySQL", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !RunTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !RunTestEnterprise(framework.Scale) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Scale))
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

	Context("Scaling Database", func() {
		Context("Horizontal scale", func() {
			It("Should horizontal scale MySQL", func() {
				// MySQL objectMeta
				issuerMeta := metav1.ObjectMeta{
					Name:      rand.WithUniqSuffix("mysql"),
					Namespace: fi.Namespace(),
				}
				issuer, err := fi.EnsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
				Expect(err).NotTo(HaveOccurred())
				// Create MySQL Group Replication with SSL secured and wait for running
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

				// Create a mysql User with required SSL
				By("Create mysql User with required SSL")
				fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, dbInfo).Should(BeTrue())
				dbInfo.User = framework.MySQLRequiredSSLUser
				fi.EventuallyCheckConnectionRequiredSSLUser(my, dbInfo)
				fi.PopulateMySQL(my.ObjectMeta, dbInfo)

				By("Configuring MySQL group member scaled up")
				myORUp := fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
					in.Name = rand.WithUniqSuffix("myops-up")
					in.Spec.Type = opsapi.OpsRequestTypeHorizontalScaling
					in.Spec.HorizontalScaling = &opsapi.MySQLHorizontalScalingSpec{
						Member: types.Int32P(5),
					}
				})

				dbInfo.User = framework.MySQLRootUser
				By("Checking MySQL horizontal scaled up")
				fi.EventuallyONLINEMembersCount(my.ObjectMeta, dbInfo).Should(Equal(int(*myORUp.Spec.HorizontalScaling.Member)))

				By("Configuring MySQL group member scaled down")
				myORDown := fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
					in.Name = rand.WithUniqSuffix("myops-down")
					in.Spec.Type = opsapi.OpsRequestTypeHorizontalScaling
					in.Spec.HorizontalScaling = &opsapi.MySQLHorizontalScalingSpec{
						Member: types.Int32P(4),
					}
				})

				By("Checking MySQL horizontal scaled down")
				fi.EventuallyONLINEMembersCount(my.ObjectMeta, dbInfo).Should(Equal(int(*myORDown.Spec.HorizontalScaling.Member)))

				// Retrieve Inserted Data
				dbInfo.User = framework.MySQLRequiredSSLUser
				By("Checking Row Count of Table")
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})

		Context("Vertical scale", func() {
			It("Should vertical scale MySQL standalone", func() {
				// MySQL objectMeta
				issuerMeta := metav1.ObjectMeta{
					Name:      rand.WithUniqSuffix("mysql"),
					Namespace: fi.Namespace(),
				}
				issuer, err := fi.EnsureIssuer(issuerMeta, api.MySQL{}.ResourceFQN())
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

				// Vertical Scaling MySQL resources
				myOR := fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeVerticalScaling
					in.Spec.VerticalScaling = &opsapi.MySQLVerticalScalingSpec{
						MySQL: &core.ResourceRequirements{
							Limits: core.ResourceList{
								core.ResourceMemory: resource.MustParse("1200Mi"),
								core.ResourceCPU:    resource.MustParse("600m"),
							},
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("1200Mi"),
								core.ResourceCPU:    resource.MustParse("600m"),
							},
						},
					}
				})

				By("Checking MySQL resource updated")
				my, err = fi.DBClient().KubedbV1alpha2().MySQLs(my.Namespace).Get(context.TODO(), my.Name, metav1.GetOptions{}) // get updated MySQL object
				Expect(err).NotTo(HaveOccurred())
				Expect(*myOR.Spec.VerticalScaling.MySQL).Should(matcher.BeSameAs(my.Spec.PodTemplate.Spec.Resources))

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
		FContext("Vertical scale", func() {
			It("Should vertical scale MySQL Group Replication", func() {
				// MySQL objectMeta
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

				// Vertical Scaling MySQL resources
				myOR := fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeVerticalScaling
					in.Spec.VerticalScaling = &opsapi.MySQLVerticalScalingSpec{
						MySQL: &core.ResourceRequirements{
							Limits: core.ResourceList{
								core.ResourceMemory: resource.MustParse("1200Mi"),
								core.ResourceCPU:    resource.MustParse("600m"),
							},
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("1200Mi"),
								core.ResourceCPU:    resource.MustParse("600m"),
							},
						},
					}
				})

				By("Checking MySQL resource updated")
				my, err = fi.DBClient().KubedbV1alpha2().MySQLs(my.Namespace).Get(context.TODO(), my.Name, metav1.GetOptions{}) // get updated MySQL object
				Expect(err).NotTo(HaveOccurred())
				Expect(*myOR.Spec.VerticalScaling.MySQL).Should(matcher.BeSameAs(my.Spec.PodTemplate.Spec.Resources))

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
	})
})
