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

var _ = FDescribe("MariaDB", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMariaDB))
		}
		if !runTestEnterprise(framework.Upgrade) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Upgrade))
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
		Context("MariaDB Standalone", func() {
			It("Should Upgrade MariaDB Standalone", func() {

				md, err := fi.CreateMariaDBAndWaitForRunning(framework.OldDBVersion)
				Expect(err).NotTo(HaveOccurred())
				// Database connection information

				// Database connection information
				mydbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, mydbInfo)

				fi.PopulateMariaDB(md, mydbInfo)
				// Upgrade MariaDB Version and waiting for success
				mdOpsReq := fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeUpgrade
					in.Spec.Upgrade = &opsapi.MariaDBUpgradeSpec{
						TargetVersion: framework.DBVersion,
					}
				})

				By("Checking MariaDB version upgraded")
				targetedVersion, err := fi.DBClient().CatalogV1alpha1().MariaDBVersions().Get(context.TODO(), mdOpsReq.Spec.Upgrade.TargetVersion, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				fi.EventuallyDatabaseVersionUpdatedMD(md.ObjectMeta, mydbInfo, targetedVersion.Spec.Version).Should(BeTrue())

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, mydbInfo).Should(Equal(3))
			})
		})

		FContext("MariaDB Cluster", func() {
			It("Should Upgrade MariaDB Cluster", func() {

				md, err := fi.CreateMariaDBAndWaitForRunning(framework.OldDBVersion, func(in *api.MariaDB) {
					in.Spec.Replicas = types.Int32P(3)
				})
				Expect(err).NotTo(HaveOccurred())
				// Database connection information

				// Database connection information
				mydbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, mydbInfo)

				fi.PopulateMariaDB(md, mydbInfo)
				// Upgrade MariaDB Version and waiting for success
				mdOpsReq := fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeUpgrade
					in.Spec.Upgrade = &opsapi.MariaDBUpgradeSpec{
						TargetVersion: framework.DBVersion,
					}
				})

				By("Checking MariaDB version upgraded")
				targetedVersion, err := fi.DBClient().CatalogV1alpha1().MariaDBVersions().Get(context.TODO(), mdOpsReq.Spec.Upgrade.TargetVersion, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				fi.EventuallyDatabaseVersionUpdatedMD(md.ObjectMeta, mydbInfo, targetedVersion.Spec.Version).Should(BeTrue())

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, mydbInfo).Should(Equal(3))
			})
		})
	})
})
