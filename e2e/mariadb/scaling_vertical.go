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
	"context"
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	opsapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"
	"kubedb.dev/tests/e2e/matcher"

	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("MariaDB", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMariaDB))
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

	Context("Vertical Scaling Database", func() {
		Context("Vertical scale", func() {
			It("Should vertical scale MariaDB standalone", func() {
				// Create MariaDB standalone and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())
				// Database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)
				fi.PopulateMariaDB(md, dbInfo)

				// Vertical Scaling MariaDB resources
				mdOpsReq := fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeVerticalScaling
					in.Spec.VerticalScaling = &opsapi.MariaDBVerticalScalingSpec{
						MariaDB: &core.ResourceRequirements{
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

				By("Checking MariaDB resource updated")
				md, err = fi.DBClient().KubedbV1alpha2().MariaDBs(md.Namespace).Get(context.TODO(), md.Name, metav1.GetOptions{}) // get updated MariaDB object
				Expect(err).NotTo(HaveOccurred())
				Expect(*mdOpsReq.Spec.VerticalScaling.MariaDB).Should(matcher.BeSameAs(md.Spec.PodTemplate.Spec.Resources))
				Expect(*mdOpsReq.Spec.VerticalScaling.MariaDB).Should(matcher.BeSameAs(md.Spec.PodTemplate.Spec.Resources))

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
		Context("Vertical scale", func() {
			It("Should vertical scale MariaDB Cluster", func() {
				// Create MariaDB Cluster and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Spec.Replicas = types.Int32P(3)
				})
				Expect(err).NotTo(HaveOccurred())
				// Database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)
				fi.PopulateMariaDB(md, dbInfo)

				// Vertical Scaling MariaDB resources
				mdOpsReq := fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeVerticalScaling
					in.Spec.VerticalScaling = &opsapi.MariaDBVerticalScalingSpec{
						MariaDB: &core.ResourceRequirements{
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

				By("Checking MariaDB resource updated")
				md, err = fi.DBClient().KubedbV1alpha2().MariaDBs(md.Namespace).Get(context.TODO(), md.Name, metav1.GetOptions{}) // get updated MariaDB object
				Expect(err).NotTo(HaveOccurred())
				Expect(*mdOpsReq.Spec.VerticalScaling.MariaDB).Should(matcher.BeSameAs(md.Spec.PodTemplate.Spec.Resources))

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})

		Context("Vertical scale", func() {
			It("Should vertical scale MariaDB standalone with Exporter", func() {
				// Create MariaDB standalone with Exporter and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					// Add Monitor
					fi.AddMariaDBMonitor(in)
				})
				Expect(err).NotTo(HaveOccurred())
				// Database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)
				fi.PopulateMariaDB(md, dbInfo)

				// Vertical Scaling MariaDB resources
				mdOpsReq := fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeVerticalScaling
					in.Spec.VerticalScaling = &opsapi.MariaDBVerticalScalingSpec{
						MariaDB: &core.ResourceRequirements{
							Limits: core.ResourceList{
								core.ResourceMemory: resource.MustParse("600Mi"),
								core.ResourceCPU:    resource.MustParse("100m"),
							},
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("600Mi"),
								core.ResourceCPU:    resource.MustParse("100m"),
							},
						},
						Exporter: &core.ResourceRequirements{
							Limits: core.ResourceList{
								core.ResourceMemory: resource.MustParse("600Mi"),
								core.ResourceCPU:    resource.MustParse("100m"),
							},
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("600Mi"),
								core.ResourceCPU:    resource.MustParse("100m"),
							},
						},
					}
				})

				By("Checking MariaDB resource updated")
				md, err = fi.DBClient().KubedbV1alpha2().MariaDBs(md.Namespace).Get(context.TODO(), md.Name, metav1.GetOptions{}) // get updated MariaDB object
				Expect(err).NotTo(HaveOccurred())
				Expect(*mdOpsReq.Spec.VerticalScaling.MariaDB).Should(matcher.BeSameAs(md.Spec.PodTemplate.Spec.Resources))
				Expect(*mdOpsReq.Spec.VerticalScaling.Exporter).Should(matcher.BeSameAs(md.Spec.Monitor.Prometheus.Exporter.Resources))

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
		Context("Vertical scale", func() {
			It("Should vertical scale MariaDB Cluster with Exporter", func() {
				// Create MariaDB Cluster with Exporter and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Spec.Replicas = types.Int32P(3)
					// Add Monitor
					fi.AddMariaDBMonitor(in)
				})
				Expect(err).NotTo(HaveOccurred())
				// Database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)
				fi.PopulateMariaDB(md, dbInfo)

				// Vertical Scaling MariaDB resources
				mdOpsReq := fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeVerticalScaling
					in.Spec.VerticalScaling = &opsapi.MariaDBVerticalScalingSpec{
						MariaDB: &core.ResourceRequirements{
							Limits: core.ResourceList{
								core.ResourceMemory: resource.MustParse("600Mi"),
								core.ResourceCPU:    resource.MustParse("100m"),
							},
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("600Mi"),
								core.ResourceCPU:    resource.MustParse("100m"),
							},
						},
						Exporter: &core.ResourceRequirements{
							Limits: core.ResourceList{
								core.ResourceMemory: resource.MustParse("600Mi"),
								core.ResourceCPU:    resource.MustParse("100m"),
							},
							Requests: core.ResourceList{
								core.ResourceMemory: resource.MustParse("600Mi"),
								core.ResourceCPU:    resource.MustParse("100m"),
							},
						},
					}
				})

				By("Checking MariaDB resource updated")
				md, err = fi.DBClient().KubedbV1alpha2().MariaDBs(md.Namespace).Get(context.TODO(), md.Name, metav1.GetOptions{}) // get updated MariaDB object
				Expect(err).NotTo(HaveOccurred())
				Expect(*mdOpsReq.Spec.VerticalScaling.MariaDB).Should(matcher.BeSameAs(md.Spec.PodTemplate.Spec.Resources))
				Expect(*mdOpsReq.Spec.VerticalScaling.Exporter).Should(matcher.BeSameAs(md.Spec.Monitor.Prometheus.Exporter.Resources))

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
	})
})
