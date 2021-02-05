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

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("MariaDB", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMariaDB))
		}
		if !runTestCommunity(framework.Resume) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Resume))
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

		Context("Resume", func() {

			Context("Super Fast User - Create-Delete-Create-Delete-Create ", func() {
				It("should resume database successfully", func() {
					// MariaDB ObjectMeta
					mdMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mariadb"),
						Namespace: fi.Namespace(),
					}
					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")

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
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})
					Expect(err).NotTo(HaveOccurred())
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Delete mariadb: " + mdMeta.Namespace + "/" + mdMeta.Name)
					err = fi.DeleteMariaDB(mdMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mariadb to be deleted")
					fi.EventuallyMariaDB(mdMeta).Should(BeFalse())

					// Create MariaDB object again to resume it
					_, err = fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						// Set termination policy WipeOut to delete all mariadb resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))
				})
			})

			Context("Without Init", func() {
				It("should resume database successfully", func() {
					// MariaDB ObjectMeta
					mdMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mariadb"),
						Namespace: fi.Namespace(),
					}
					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")

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
						// Set termination policy WipeOut to delete all mariadb resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))
				})
			})

			Context("with init Script", func() {
				It("should resume database successfully", func() {
					// MariaDB ObjectMeta
					mdMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mariadb"),
						Namespace: fi.Namespace(),
					}
					// insure initScriptConfigMap
					cm, err := fi.EnsureMyInitScriptConfigMap()
					Expect(err).NotTo(HaveOccurred())
					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						in.Spec.Init = &api.InitSpec{
							Script: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: cm.Name,
										},
									},
								},
							},
						}
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")

					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))

					By("Delete mariadb " + mdMeta.Namespace + "/" + mdMeta.Name)
					err = fi.DeleteMariaDB(mdMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mariadb to be deleted")
					fi.EventuallyMariaDB(mdMeta).Should(BeFalse())

					// Create MariaDB object again to resume it
					_, err = fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						in.Spec.Init = &api.InitSpec{
							Script: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: cm.Name,
										},
									},
								},
							},
						}
						// Set termination policy WipeOut to delete all mariadb resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))

					md, err = fi.GetMariaDB(mdMeta)
					Expect(err).NotTo(HaveOccurred())
					Expect(md.Spec.Init).NotTo(BeNil())

					//By("Checking MariaDB crd does not have status.conditions[DataRestored]")
					//Expect(kmapi.HasCondition(md.Status.Conditions, api.DatabaseDataRestored)).To(BeFalse())
				})
			})

			Context("Multiple times with init", func() {
				It("should resume database successfully", func() {
					// MariaDB ObjectMeta
					mdMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mariadb"),
						Namespace: fi.Namespace(),
					}
					// insure initScriptConfigMap
					cm, err := fi.EnsureMyInitScriptConfigMap()
					Expect(err).NotTo(HaveOccurred())
					// Create MariaDB standalone and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						in.Spec.Init = &api.InitSpec{
							Script: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: cm.Name,
										},
									},
								},
							},
						}
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")

					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))

					for i := 0; i < 3; i++ {
						By(fmt.Sprintf("%v-th", i+1) + " time running.")

						By("Delete mariadb " + mdMeta.Namespace + "/" + mdMeta.Name)
						err = fi.DeleteMariaDB(mdMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for mariadb to be deleted")
						fi.EventuallyMariaDB(mdMeta).Should(BeFalse())

						// Create MariaDB object again to resume it
						_, err = fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
							in.Name = mdMeta.Name
							in.Namespace = mdMeta.Namespace
							in.Spec.Init = &api.InitSpec{
								Script: &api.ScriptSourceSpec{
									VolumeSource: core.VolumeSource{
										ConfigMap: &core.ConfigMapVolumeSource{
											LocalObjectReference: core.LocalObjectReference{
												Name: cm.Name,
											},
										},
									},
								},
							}
							// Set termination policy WipeOut to delete all mariadb resources permanently
							in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
						})
						Expect(err).NotTo(HaveOccurred())
						fi.EventuallyDBReadyMD(md, dbInfo)

						By("Checking Row Count of Table")
						fi.EventuallyCountRowMD(mdMeta, dbInfo).Should(Equal(3))

						md, err := fi.GetMariaDB(mdMeta)
						Expect(err).NotTo(HaveOccurred())
						Expect(md.Spec.Init).ShouldNot(BeNil())

						//By("Checking MariaDB crd does not have status.conditions[DataRestored]")
						//Expect(kmapi.HasCondition(md.Status.Conditions, api.DatabaseDataRestored)).To(BeFalse())
					}
					By("Update mariadb to set spec.terminationPolicy = WipeOut")
					_, err = fi.PatchMariaDB(mdMeta, func(in *api.MariaDB) *api.MariaDB {
						// Set termination policy WipeOut to delete all mariadb resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
						return in
					})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

	})
})
