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

package postgres

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
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

	Describe("General Test", func() {
		Context("Initialize With Script", func() {
			It("should run successfully", func() {
				// insure initScriptConfigMap
				cm, err := fi.EnsureMyInitScriptConfigMap()
				Expect(err).NotTo(HaveOccurred())
				// Create MariaDB standalone and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
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
				// Database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")

				fi.EventuallyDBReadyMD(md, dbInfo)

				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
	})
})
