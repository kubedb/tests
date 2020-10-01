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

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !runTestEnterprise(framework.Scale) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Scale))
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
				fi.EventuallyDBReady(my, dbInfo)

				By("Creating Table")
				fi.EventuallyCreateTable(my.ObjectMeta, dbInfo).Should(BeTrue())

				By("Inserting Rows")
				fi.EventuallyInsertRow(my.ObjectMeta, dbInfo, 3).Should(BeTrue())

				By("Checking Row Count of Table")
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))

				By("Configuring MySQL group member scaled up")
				myORUp := fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
					in.Name = rand.WithUniqSuffix("myops-up")
					in.Spec.Type = opsapi.OpsRequestTypeHorizontalScaling
					in.Spec.HorizontalScaling = &opsapi.MySQLHorizontalScalingSpec{
						Member: types.Int32P(5),
					}
				})

				By("Checking MySQL horizontal scaled up")
				for i := int32(0); i < *myORUp.Spec.HorizontalScaling.Member; i++ {
					By(fmt.Sprintf("Checking ONLINE member count from Pod '%s-%d'", my.Name, i))
					dbInfo.ClientPodIndex = int(1)
					fi.EventuallyONLINEMembersCount(my.ObjectMeta, dbInfo).Should(Equal(int(*myORUp.Spec.HorizontalScaling.Member)))
				}

				By("Configuring MySQL group member scaled down")
				myORDown := fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
					in.Name = rand.WithUniqSuffix("myops-down")
					in.Spec.Type = opsapi.OpsRequestTypeHorizontalScaling
					in.Spec.HorizontalScaling = &opsapi.MySQLHorizontalScalingSpec{
						Member: types.Int32P(4),
					}
				})

				By("Checking MySQL horizontal scaled down")
				for i := int32(0); i < *myORDown.Spec.HorizontalScaling.Member; i++ {
					By(fmt.Sprintf("Checking ONLINE member count from Pod '%s-%d'", my.Name, i))
					dbInfo.ClientPodIndex = int(1)
					fi.EventuallyONLINEMembersCount(my.ObjectMeta, dbInfo).Should(Equal(int(*myORDown.Spec.HorizontalScaling.Member)))
				}

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				dbInfo.ClientPodIndex = int(3)
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
		Context("Vertical scale", func() {
			It("Should vertical scale MySQL standalone", func() {
				// Create MySQL standalone and wait for running
				my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())
				// Database connection information
				dbInfo := framework.DatabaseConnectionInfo{
					StatefulSetOrdinal: 0,
					ClientPodIndex:     0,
					DatabaseName:       framework.DBMySQL,
					User:               framework.MySQLRootUser,
					Param:              "",
				}
				fi.EventuallyDBReady(my, dbInfo)

				By("Creating Table")
				fi.EventuallyCreateTable(my.ObjectMeta, dbInfo).Should(BeTrue())

				By("Inserting Rows")
				fi.EventuallyInsertRow(my.ObjectMeta, dbInfo, 3).Should(BeTrue())

				By("Checking Row Count of Table")
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))

				// Vertical Scaling MySQL resources
				myOR := fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeVerticalScaling
					in.Spec.VerticalScaling = &opsapi.MySQLVerticalScalingSpec{
						MySQL: &core.ResourceRequirements{
							Limits: core.ResourceList{
								core.ResourceMemory: resource.MustParse("300Mi"),
								core.ResourceCPU:    resource.MustParse("200m"),
							},
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("200Mi"),
								core.ResourceCPU:    resource.MustParse("100m"),
							},
						},
					}
				})

				By("Checking MySQL resource updated")
				my, err = fi.DBClient().KubedbV1alpha1().MySQLs(my.Namespace).Get(context.TODO(), my.Name, metav1.GetOptions{}) // get updated MySQL object
				Expect(err).NotTo(HaveOccurred())
				Expect(*myOR.Spec.VerticalScaling.MySQL).Should(matcher.BeSameAs(my.Spec.PodTemplate.Spec.Resources))

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
		Context("Vertical scale", func() {
			It("Should vertical scale MySQL Group Replication", func() {
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
				fi.EventuallyDBReady(my, dbInfo)

				By("Creating Table")
				fi.EventuallyCreateTable(my.ObjectMeta, dbInfo).Should(BeTrue())

				By("Inserting Rows")
				fi.EventuallyInsertRow(my.ObjectMeta, dbInfo, 3).Should(BeTrue())

				By("Checking Row Count of Table")
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))

				// Vertical Scaling MySQL resources
				myOR := fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeVerticalScaling
					in.Spec.VerticalScaling = &opsapi.MySQLVerticalScalingSpec{
						MySQL: &core.ResourceRequirements{
							Limits: core.ResourceList{
								core.ResourceMemory: resource.MustParse("300Mi"),
								core.ResourceCPU:    resource.MustParse("200m"),
							},
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("200Mi"),
								core.ResourceCPU:    resource.MustParse("100m"),
							},
						},
					}
				})

				By("Checking MySQL resource updated")
				my, err = fi.DBClient().KubedbV1alpha1().MySQLs(my.Namespace).Get(context.TODO(), my.Name, metav1.GetOptions{}) // get updated MySQL object
				Expect(err).NotTo(HaveOccurred())
				Expect(*myOR.Spec.VerticalScaling.MySQL).Should(matcher.BeSameAs(my.Spec.PodTemplate.Spec.Resources))

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
	})
})
