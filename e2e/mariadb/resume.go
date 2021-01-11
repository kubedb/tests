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
	kmapi "kmodules.xyz/client-go/api/v1"
)

var _ = Describe("MySQL", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
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
					// MySQL ObjectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
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
					fi.EventuallyCreateTable(myMeta, dbInfo).Should(BeTrue())

					By("Inserting Rows")
					fi.EventuallyInsertRow(myMeta, dbInfo, 3).Should(BeTrue())

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(myMeta, dbInfo).Should(Equal(3))

					By("Delete mysql: " + myMeta.Namespace + "/" + myMeta.Name)
					err = fi.DeleteMySQL(myMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mysql to be deleted")
					fi.EventuallyMySQL(myMeta).Should(BeFalse())

					// Create MySQL object again to resume it
					_, err = fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})
					Expect(err).NotTo(HaveOccurred())
					fi.EventuallyDBReady(my, dbInfo)

					By("Delete mysql: " + myMeta.Namespace + "/" + myMeta.Name)
					err = fi.DeleteMySQL(myMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mysql to be deleted")
					fi.EventuallyMySQL(myMeta).Should(BeFalse())

					// Create MySQL object again to resume it
					_, err = fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(myMeta, dbInfo).Should(Equal(3))
				})
			})

			Context("Without Init", func() {
				It("should resume database successfully", func() {
					// MySQL ObjectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
						// Set termination policy Halt to leave the PVCs and secrets intact for reuse
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
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
					fi.EventuallyCreateTable(myMeta, dbInfo).Should(BeTrue())

					By("Inserting Rows")
					fi.EventuallyInsertRow(myMeta, dbInfo, 3).Should(BeTrue())

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(myMeta, dbInfo).Should(Equal(3))

					By("Delete mysql: " + myMeta.Namespace + "/" + myMeta.Name)
					err = fi.DeleteMySQL(myMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mysql to be deleted")
					fi.EventuallyMySQL(myMeta).Should(BeFalse())

					// Create MySQL object again to resume it
					_, err = fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(myMeta, dbInfo).Should(Equal(3))
				})
			})

			Context("with init Script", func() {
				It("should resume database successfully", func() {
					// MySQL ObjectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					// insure initScriptConfigMap
					cm, err := fi.EnsureMyInitScriptConfigMap()
					Expect(err).NotTo(HaveOccurred())
					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
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
					dbInfo := framework.DatabaseConnectionInfo{
						StatefulSetOrdinal: 0,
						ClientPodIndex:     0,
						DatabaseName:       framework.DBMySQL,
						User:               framework.MySQLRootUser,
						Param:              "",
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(myMeta, dbInfo).Should(Equal(3))

					By("Delete mysql " + myMeta.Namespace + "/" + myMeta.Name)
					err = fi.DeleteMySQL(myMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mysql to be deleted")
					fi.EventuallyMySQL(myMeta).Should(BeFalse())

					// Create MySQL object again to resume it
					_, err = fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
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
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					Expect(err).NotTo(HaveOccurred())
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(myMeta, dbInfo).Should(Equal(3))

					my, err = fi.GetMySQL(myMeta)
					Expect(err).NotTo(HaveOccurred())
					Expect(my.Spec.Init).NotTo(BeNil())

					By("Checking MySQL crd does not have status.conditions[DataRestored]")
					Expect(kmapi.HasCondition(my.Status.Conditions, api.DatabaseDataRestored)).To(BeFalse())
				})
			})

			Context("Multiple times with init", func() {
				It("should resume database successfully", func() {
					// MySQL ObjectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					// insure initScriptConfigMap
					cm, err := fi.EnsureMyInitScriptConfigMap()
					Expect(err).NotTo(HaveOccurred())
					// Create MySQL standalone and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						in.Namespace = myMeta.Namespace
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
					dbInfo := framework.DatabaseConnectionInfo{
						StatefulSetOrdinal: 0,
						ClientPodIndex:     0,
						DatabaseName:       framework.DBMySQL,
						User:               framework.MySQLRootUser,
						Param:              "",
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(myMeta, dbInfo).Should(Equal(3))

					for i := 0; i < 3; i++ {
						By(fmt.Sprintf("%v-th", i+1) + " time running.")

						By("Delete mysql " + myMeta.Namespace + "/" + myMeta.Name)
						err = fi.DeleteMySQL(myMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for mysql to be deleted")
						fi.EventuallyMySQL(myMeta).Should(BeFalse())

						// Create MySQL object again to resume it
						_, err = fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
							in.Name = myMeta.Name
							in.Namespace = myMeta.Namespace
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
							// Set termination policy WipeOut to delete all mysql resources permanently
							in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
						})
						Expect(err).NotTo(HaveOccurred())
						fi.EventuallyDBReady(my, dbInfo)

						By("Checking Row Count of Table")
						fi.EventuallyCountRow(myMeta, dbInfo).Should(Equal(3))

						my, err := fi.GetMySQL(myMeta)
						Expect(err).NotTo(HaveOccurred())
						Expect(my.Spec.Init).ShouldNot(BeNil())

						By("Checking MySQL crd does not have status.conditions[DataRestored]")
						Expect(kmapi.HasCondition(my.Status.Conditions, api.DatabaseDataRestored)).To(BeFalse())
					}
					By("Update mysql to set spec.terminationPolicy = WipeOut")
					_, err = fi.PatchMySQL(myMeta, func(in *api.MySQL) *api.MySQL {
						// Set termination policy WipeOut to delete all mysql resources permanently
						in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
						return in
					})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

	})
})
