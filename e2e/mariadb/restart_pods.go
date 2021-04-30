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

	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MariaDB", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMariaDB))
		}
		if !runTestCommunity(framework.Initialize) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Initialize))
		}
	})

	JustAfterEach(func() {
		fi.PrintDebugInfoOnFailure()
	})

	AfterEach(func() {
		err := fi.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())

	})

	Describe("Restart Pod", func() {
		Context("MariaDB Standalone", func() {
			It("should restart the pod successfully", func() {
				// Create MariaDB standalone and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion)
				Expect(err).NotTo(HaveOccurred())
				// Database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")

				fi.EventuallyDBReadyMD(md, dbInfo)
				fi.PopulateMariaDB(md, dbInfo)

				// Restart MariaDB Pod and wait for success
				_ = fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeRestart
				})
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
	})

	Describe("Restart Pods", func() {
		Context("MariaDB Cluster", func() {
			It("should restart the pods successfully", func() {
				// Create MariaDB standalone and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Spec.Replicas = types.Int32P(3)
				})
				Expect(err).NotTo(HaveOccurred())
				// Database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")

				fi.EventuallyDBReadyMD(md, dbInfo)
				fi.PopulateMariaDB(md, dbInfo)

				// Restart MariaDB pods and wait for success
				_ = fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeRestart
				})
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
	})
})
