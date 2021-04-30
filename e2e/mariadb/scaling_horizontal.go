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

	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	Context("Horizontal Scaling Database", func() {
		Context("Horizontal Up Scale", func() {
			It("Should horizontal up scale MariaDB Cluster", func() {
				// Create MariaDB Cluster and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Spec.Replicas = types.Int32P(3)
				})
				Expect(err).NotTo(HaveOccurred())
				// Database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)
				fi.PopulateMariaDB(md, dbInfo)

				// Horizontal Scaling MariaDB resources
				mdOpsReq := fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeHorizontalScaling
					in.Spec.HorizontalScaling = &opsapi.MariaDBHorizontalScalingSpec{
						Member: types.Int32P(5),
					}
				})

				By("Checking MariaDB replica count after upscale")
				md, err = fi.DBClient().KubedbV1alpha2().MariaDBs(md.Namespace).Get(context.TODO(), md.Name, metav1.GetOptions{}) // get updated MariaDB object
				Expect(err).NotTo(HaveOccurred())
				fi.EventuallyONLINEMembersCountMD(md.ObjectMeta, dbInfo).Should(Equal(int(*mdOpsReq.Spec.HorizontalScaling.Member)))

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})

		Context("Horizontal Down Scale", func() {
			It("Should horizontal down scale MariaDB Cluster", func() {
				// Create MariaDB Cluster and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Spec.Replicas = types.Int32P(5)
				})
				Expect(err).NotTo(HaveOccurred())
				// Database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)
				fi.PopulateMariaDB(md, dbInfo)

				// Horizontal Scaling MariaDB resources
				mdOpsReq := fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeHorizontalScaling
					in.Spec.HorizontalScaling = &opsapi.MariaDBHorizontalScalingSpec{
						Member: types.Int32P(3),
					}
				})

				By("Checking MariaDB replica count after downscale")
				md, err = fi.DBClient().KubedbV1alpha2().MariaDBs(md.Namespace).Get(context.TODO(), md.Name, metav1.GetOptions{}) // get updated MariaDB object
				Expect(err).NotTo(HaveOccurred())
				fi.EventuallyONLINEMembersCountMD(md.ObjectMeta, dbInfo).Should(Equal(int(*mdOpsReq.Spec.HorizontalScaling.Member)))

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
	})
})
