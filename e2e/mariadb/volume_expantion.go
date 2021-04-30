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
	"reflect"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	opsapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("MariaDB", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()
		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMariaDB))
		}
		if !runTestEnterprise(framework.VolumeExpansion) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.VolumeExpansion))
		}
	})

	JustAfterEach(func() {
		fi.PrintDebugInfoOnFailure()
	})

	AfterEach(func() {
		err := fi.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())

	})

	Context("Volume Expansion", func() {
		Context("MariaDB Standalone", func() {
			It("Should volume expanded", func() {
				// Create MariaDB standalone and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Spec.Storage = &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse(framework.DBPvcStorageSize),
							},
						},
						AccessModes: []core.PersistentVolumeAccessMode{
							core.ReadWriteOnce,
						},
						StorageClassName: types.StringP(fi.StorageClass),
					}
					// Set termination policy WipeOut to delete all mysql resources permanently
					in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				})
				Expect(err).NotTo(HaveOccurred())
				// Database connection information

				// Database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)

				fi.PopulateMariaDB(md, dbInfo)

				requestedVolumeSize := resource.MustParse("2Gi")
				_ = fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeVolumeExpansion
					in.Spec.VolumeExpansion = &opsapi.MariaDBVolumeExpansionSpec{
						MariaDB: &requestedVolumeSize,
					}
				})

				fi.EventuallyDBReadyMD(md, dbInfo)
				By("Checking database volume expanded")
				// in KubeDB, StatefulSet's 1st pod name will be formed as "data-<database_name>-0"
				pvcName := fmt.Sprintf("data-%s-0", md.Name)
				pvc, err := fi.GetPersistentVolumeClaim(pvcName)
				Expect(err).NotTo(HaveOccurred())
				if !reflect.DeepEqual(requestedVolumeSize, *pvc.Status.Capacity.Storage()) {
					Expect(fmt.Errorf("current and previous is not equal")).NotTo(HaveOccurred())
				}

				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})

		Context("MariaDB Cluster", func() {
			It("Should volume expanded", func() {
				// Create MariaDB standalone and wait for running
				md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
					in.Spec.Storage = &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse(framework.DBPvcStorageSize),
							},
						},
						AccessModes: []core.PersistentVolumeAccessMode{
							core.ReadWriteOnce,
						},
						StorageClassName: types.StringP(fi.StorageClass),
					}
					in.Spec.Replicas = types.Int32P(api.MariaDBDefaultClusterSize)
					// Set termination policy WipeOut to delete all mysql resources permanently
					in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				})
				Expect(err).NotTo(HaveOccurred())
				// Database connection information

				// Database connection information
				dbInfo := framework.GetMariaDBInfo(framework.DBMySQL, framework.MySQLRootUser, "")
				fi.EventuallyDBReadyMD(md, dbInfo)

				fi.PopulateMariaDB(md, dbInfo)

				requestedVolumeSize := resource.MustParse("2Gi")
				_ = fi.CreateMariaDBOpsRequestsAndWaitForSuccess(md.Name, func(in *opsapi.MariaDBOpsRequest) {
					in.Spec.Type = opsapi.OpsRequestTypeVolumeExpansion
					in.Spec.VolumeExpansion = &opsapi.MariaDBVolumeExpansionSpec{
						MariaDB: &requestedVolumeSize,
					}
				})

				fi.EventuallyDBReadyMD(md, dbInfo)
				By("Checking database volume expanded")

				for i := 0; i < api.MariaDBDefaultClusterSize; i++ {
					// in KubeDB, StatefulSet's 1st pod name will be formed as "data-<pod_name>"
					pvcName := fmt.Sprintf("data-%s-%d", md.Name, i)
					pvc, err := fi.GetPersistentVolumeClaim(pvcName)
					Expect(err).NotTo(HaveOccurred())
					if !reflect.DeepEqual(requestedVolumeSize, *pvc.Status.Capacity.Storage()) {
						Expect(fmt.Errorf("current and previous is not equal for pvc %s", pvcName)).NotTo(HaveOccurred())
					}
				}
				// Retrieve Inserted Data
				By("Checking Row Count of Table")
				fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
			})
		})
	})
})
