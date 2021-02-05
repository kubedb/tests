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
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha2/util"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MYSQL_DATABASE      = "MYSQL_DATABASE"
	MYSQL_ROOT_PASSWORD = "MYSQL_ROOT_PASSWORD"
)

var _ = Describe("MariaDB", func() {

	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMariaDB))
		}
		if !runTestCommunity(framework.EnvironmentVariable) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.EnvironmentVariable))
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

		Context("EnvVars", func() {

			Context("Database Name as EnvVar", func() {

				It("should create DB with name provided in EvnVar", func() {
					// MariaDB database name
					dbName := fi.App()
					mdMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mariadb"),
						Namespace: fi.Namespace(),
					}
					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						in.Spec.PodTemplate.Spec.Env = []core.EnvVar{
							{
								Name:  MYSQL_DATABASE,
								Value: dbName,
							},
						}
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.MariaDBInfo{
						DatabaseName:       dbName,
						User:               framework.MySQLRootUser,
						Param:              "",
					}
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Creating Table")
					fi.EventuallyCreateTableMD(mdMeta, dbInfo).Should(BeTrue())

					By("Inserting Rows")
					fi.EventuallyInsertRowMD(mdMeta, dbInfo, 3).Should(BeTrue())

					By("Checking Row Count of Table")
					fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))

					By("Delete mariadb: " + mdMeta.Namespace + "/" + mdMeta.Name)
					err = fi.DeleteMariaDB(mdMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mariadb to be deleted")
					fi.EventuallyMariaDB(mdMeta).Should(BeFalse())

					// Create MariaDB object again to resume it
					_, err = fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))

				})
			})

			Context("Root Password as EnvVar", func() {

				It("should reject to create MariaDB CRD", func() {
					// Create MariaDB standalone and wait for running
					_, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Spec.PodTemplate.Spec.Env = []core.EnvVar{
							{
								Name:  MYSQL_ROOT_PASSWORD,
								Value: "not@secret",
							},
						}
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})
					Expect(err).To(HaveOccurred())
				})
			})

			Context("Update EnvVar", func() {

				It("should not reject to update EvnVar", func() {
					dbName := fi.App()
					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Spec.PodTemplate.Spec.Env = []core.EnvVar{
							{
								Name:  MYSQL_DATABASE,
								Value: dbName,
							},
						}
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.MariaDBInfo{
						DatabaseName:       dbName,
						User:               framework.MySQLRootUser,
						Param:              "",
					}
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Creating Table")
					fi.EventuallyCreateTableMD(md.ObjectMeta, dbInfo).Should(BeTrue())

					By("Inserting Rows")
					fi.EventuallyInsertRowMD(md.ObjectMeta, dbInfo, 3).Should(BeTrue())

					By("Checking Row Count of Table")
					fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))

					By("Patching EnvVar")
					_, _, err = util.PatchMariaDB(context.TODO(), fi.DBClient().KubedbV1alpha2(), md, func(in *api.MariaDB) *api.MariaDB {
						in.Spec.PodTemplate.Spec.Env = []core.EnvVar{
							{
								Name:  MYSQL_DATABASE,
								Value: "patched-db",
							},
						}
						return in
					}, metav1.PatchOptions{})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

	})
})
