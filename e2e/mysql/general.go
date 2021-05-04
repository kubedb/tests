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
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var _ = Describe("MySQL", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !RunTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !RunTestCommunity(framework.General) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.General))
		}
	})

	JustAfterEach(func() {
		fi.PrintDebugInfoOnFailure()
	})

	AfterEach(func() {
		err := fi.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())

	})

	Context("General", func() {

		Context("-", func() {
			It("should run successfully", func() {
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
					DatabaseName: framework.DBMySQL,
					User:         framework.MySQLRootUser,
					Param:        "",
				}
				fi.EventuallyDBReady(my, dbInfo)
				fi.PopulateMySQL(my.ObjectMeta, dbInfo)

				By("Delete mysql: " + myMeta.Namespace + "/" + myMeta.Name)
				err = fi.DeleteMySQL(myMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Wait for mysql to be deleted")
				fi.EventuallyMySQL(myMeta).Should(BeFalse())

				// Create MySQL object again to resume it
				my, err = fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
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

		Context("with custom SA Name", func() {
			var ensureCustomSecret = func(objMeta metav1.ObjectMeta) (*core.Secret, error) {
				By("Create Custom secret for MySQL")
				secret := fi.SecretForDatabaseAuthentication(objMeta, false)
				secret, err := fi.CreateSecret(secret)
				if err != nil {
					return nil, err
				}
				fi.AppendToCleanupList(secret)
				return secret, err
			}

			It("should start and resume successfully", func() {
				// MySQl objectMeta
				myMeta := metav1.ObjectMeta{
					Name:      rand.WithUniqSuffix("mysql"),
					Namespace: fi.Namespace(),
				}
				// Create custom Secret for MySQL
				customSecret, err := ensureCustomSecret(myMeta)
				Expect(err).NotTo(HaveOccurred())
				// Create MySQL standalone and wait for running
				_, err = fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
					in.Name = myMeta.Name
					in.Namespace = myMeta.Namespace
					in.Spec.AuthSecret = &core.LocalObjectReference{
						Name: customSecret.Name,
					}
					in.Spec.PodTemplate.Spec.ServiceAccountName = "my-custom-sa"
					// Set termination policy Halt to leave the PVCs and secrets intact for reuse
					in.Spec.TerminationPolicy = api.TerminationPolicyHalt
				})
				Expect(err).NotTo(HaveOccurred())

				By("Check if MySQL " + myMeta.Namespace + "/" + myMeta.Name + " exists.")
				_, err = fi.GetMySQL(myMeta)
				if err != nil {
					if kerr.IsNotFound(err) {
						// MySQL was not created. Hence, rest of cleanup is not necessary.
						return
					}
					Expect(err).NotTo(HaveOccurred())
				}

				By("Delete mysql: " + myMeta.Namespace + "/" + myMeta.Name)
				err = fi.DeleteMySQL(myMeta)
				if err != nil {
					if kerr.IsNotFound(err) {
						// MySQL was not created. Hence, rest of cleanup is not necessary.
						klog.Infof("Skipping rest of cleanup. Reason: MySQL %s/%s is not found.", myMeta.Namespace, myMeta.Name)
						return
					}
					Expect(err).NotTo(HaveOccurred())
				}

				By("Wait for mysql to be deleted " + myMeta.Namespace + "/" + myMeta.Name)
				fi.EventuallyMySQL(myMeta).Should(BeFalse())

				// Create MySQL object again to resume it
				_, err = fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
					in.Name = myMeta.Name
					in.Namespace = myMeta.Namespace
					in.Spec.AuthSecret = &core.LocalObjectReference{
						Name: customSecret.Name,
					}
					// Set termination policy WipeOut to delete all mysql resources permanently
					in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("PDB", func() {
			It("should run eviction successfully", func() {
				// Create MySQL standalone and wait for running
				my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
					in.Spec.Replicas = types.Int32P(3)
					// Set termination policy WipeOut to delete all mysql resources permanently
					in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				})
				Expect(err).NotTo(HaveOccurred())

				//Evict MySQL pods
				By("Try to evict pods")
				err = fi.EvictPodsFromStatefulSet(my.ObjectMeta, api.MySQL{}.ResourceFQN())
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
